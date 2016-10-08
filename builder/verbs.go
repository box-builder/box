package builder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/docker/engine-api/types"
	mruby "github.com/mitchellh/go-mruby"
)

// Definition is a jump table definition used for programming the DSL into the
// mruby interpreter.
type Definition struct {
	Func    Func
	ArgSpec mruby.ArgSpec
}

var jumpTable = map[string]Definition{
	"from":       {from, mruby.ArgsReq(1)},
	"run":        {run, mruby.ArgsAny()},
	"with_user":  {withUser, mruby.ArgsBlock() | mruby.ArgsReq(1)},
	"inside":     {inside, mruby.ArgsBlock() | mruby.ArgsReq(1)},
	"env":        {env, mruby.ArgsAny()},
	"cmd":        {cmd, mruby.ArgsAny()},
	"entrypoint": {entrypoint, mruby.ArgsAny()},
}

// Func is a builder DSL function used to interact with docker.
type Func func(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value)

func entrypoint(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	stringArgs := []string{}
	for _, arg := range m.GetArgs() {
		stringArgs = append(stringArgs, arg.String())
	}

	b.entrypoint = stringArgs
	b.config.Entrypoint = stringArgs

	if err := b.commit(); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func from(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.imageID = args[0].String()
	b.config.Tty = true
	b.config.AttachStdout = true
	b.config.AttachStderr = true

	reader, err := b.client.ImagePull(context.Background(), b.imageID, types.ImagePullOptions{})
	if err != nil {
		return nil, createException(m, err.Error())
	}

	buf := bufio.NewReader(reader)
	for {
		line, err := buf.ReadBytes('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, createException(m, err.Error())
		}

		var unpacked map[string]string
		json.Unmarshal(line, &unpacked)
		fmt.Printf("%s %s %s\r", unpacked["id"], unpacked["status"], unpacked["progress"])
	}

	return mruby.String(fmt.Sprintf("Response: %v", b.id)), nil
}

func run(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if b.imageID == "" {
		return mruby.String("`from` must precede any `run` statements"), nil
	}

	stringArgs := []string{}
	for _, arg := range m.GetArgs() {
		stringArgs = append(stringArgs, arg.String())
	}

	entrypoint := b.config.Entrypoint
	cmd := b.config.Cmd
	b.config.Entrypoint = []string{"/bin/sh", "-c"}
	b.config.Cmd = stringArgs
	defer func() {
		b.config.Entrypoint = entrypoint
		b.config.Cmd = cmd
	}()

	resp, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, createException(m, fmt.Sprintf("Error creating container: %v", err))
	}

	cearesp, err := b.client.ContainerAttach(context.Background(), resp.ID, types.ContainerAttachOptions{Stream: true, Stdout: true, Stderr: true})
	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not attach to container: %v", err))
	}

	err = b.client.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not start container: %v", err))
	}

	fmt.Println("------ BEGIN OUTPUT ------")

	_, err = io.Copy(os.Stdout, cearesp.Reader)
	if err != nil && err != io.EOF {
		return nil, createException(m, err.Error())
	}

	fmt.Println("------ END OUTPUT ------")

	stat, err := b.client.ContainerWait(context.Background(), resp.ID)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	if stat != 0 {
		return nil, createException(m, fmt.Sprintf("Command exited with status %d for container %q", stat, b.id))
	}

	commitResp, err := b.client.ContainerCommit(context.Background(), resp.ID, types.ContainerCommitOptions{Config: b.config})
	if err != nil {
		return nil, createException(m, fmt.Sprintf("Error during commit: %v", err))
	}

	b.imageID = commitResp.ID
	b.id = ""

	err = b.client.ContainerRemove(context.Background(), resp.ID, types.ContainerRemoveOptions{Force: true})
	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not remove intermediate container %q: %v", b.id, err))
	}

	return nil, nil
}

func withUser(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.config.User = args[0].String()
	val, err := m.Yield(args[1], args[0])
	b.config.User = ""
	b.id = ""

	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	return val, nil
}

func inside(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.config.WorkingDir = args[0].String()
	val, err := m.Yield(args[1], args[0])
	b.config.WorkingDir = ""
	b.id = ""

	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	return val, nil
}

func env(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	hash := args[0].Hash()

	// mruby does not expose native maps, just ruby primitives, so we have to
	// iterate through it with indexing functions instead of typical idioms.
	keys, err := hash.Keys()
	if err != nil {
		return nil, createException(m, err.Error())
	}

	for i := 0; i < keys.Array().Len(); i++ {
		key, err := keys.Array().Get(i)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		value, err := hash.Get(key)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		b.config.Env = append(b.config.Env, fmt.Sprintf("%s=%s", key.String(), value.String()))
	}

	if err := b.commit(); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func cmd(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	stringArgs := []string{}
	for _, arg := range args {
		stringArgs = append(stringArgs, arg.String())
	}

	b.cmd = stringArgs
	b.config.Cmd = stringArgs

	if err := b.commit(); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}
