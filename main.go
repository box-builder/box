package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	mruby "github.com/mitchellh/go-mruby"
)

// Definition is a jump table definition used for programming the DSL into the
// mruby interpreter.
type Definition struct {
	Func    Func
	ArgSpec mruby.ArgSpec
}

func from(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.config.Image = args[0].String()
	b.config.Tty = true
	b.config.AttachStdout = true
	b.config.AttachStderr = true

	resp, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)

	if err != nil {
		return mruby.String(err.Error()), nil
	}

	b.id = resp.ID

	return mruby.String(fmt.Sprintf("Response: %v", resp.ID)), nil
}

func run(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if b.imageID == "" {
		return mruby.String("`from` must be the first docker command`"), nil
	}

	stringArgs := []string{}
	for _, arg := range m.GetArgs() {
		stringArgs = append(stringArgs, arg.String())
	}

	cmd := b.config.Cmd

	b.config.Cmd = append([]string{"/bin/sh", "-c"}, stringArgs...)
	defer func() { b.config.Cmd = cmd }()

	resp, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return mruby.String(fmt.Sprintf("Error creating intermediate container: %v", err)), nil
	}

	cearesp, err := b.client.ContainerAttach(context.Background(), resp.ID, types.ContainerAttachOptions{Stream: true, Stdout: true, Stderr: true})
	if err != nil {
		return mruby.String(fmt.Sprintf("Error attaching to execution context %q: %v", resp.ID, err)), nil
	}

	err = b.client.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return mruby.String(fmt.Sprintf("Error attaching to execution context %q: %v", resp.ID, err)), nil
	}

	b.id = resp.ID

	_, err = io.Copy(os.Stdout, cearesp.Reader)
	if err != nil && err != io.EOF {
		return mruby.String(err.Error()), nil
	}

	return nil, nil
}

func user(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.config.User = args[0].String()
	val, err := m.Yield(args[1], args[0])
	b.config.User = ""
	b.id = ""

	if err != nil {
		return mruby.String(fmt.Sprintf("Could not yield: %v", err)), nil
	}

	return val, nil
}

func workdir(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.config.WorkingDir = args[0].String()
	val, err := m.Yield(args[1], args[0])
	b.config.WorkingDir = ""
	b.id = ""

	if err != nil {
		return mruby.String(fmt.Sprintf("Could not yield: %v", err)), nil
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
		return mruby.String(err.Error()), nil
	}

	for i := 0; i < keys.Array().Len(); i++ {
		key, err := keys.Array().Get(i)
		if err != nil {
			return mruby.String(err.Error()), nil
		}

		value, err := hash.Get(key)
		if err != nil {
			return mruby.String(err.Error()), nil
		}

		b.config.Env = append(b.config.Env, fmt.Sprintf("%s=%s", key.String(), value.String()))
	}

	createResp, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return mruby.String(fmt.Sprintf("Error creating intermediate container: %v", err)), nil
	}

	b.id = createResp.ID

	return nil, nil
}

func cmd(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	stringArgs := []string{}
	for _, arg := range args {
		stringArgs = append(stringArgs, arg.String())
	}

	b.config.Cmd = stringArgs

	createResp, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return mruby.String(fmt.Sprintf("Error creating intermediate container: %v", err)), nil
	}

	b.id = createResp.ID

	return nil, nil
}

var jumpTable = map[string]Definition{
	"from":    {from, mruby.ArgsReq(1)},
	"run":     {run, mruby.ArgsAny()},
	"user":    {user, mruby.ArgsBlock() | mruby.ArgsReq(1)},
	"workdir": {workdir, mruby.ArgsBlock() | mruby.ArgsReq(1)},
	"env":     {env, mruby.ArgsAny()},
	"cmd":     {cmd, mruby.ArgsAny()},
}

// Func is a builder DSL function used to interact with docker.
type Func func(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value)

// Builder implements the builder core.
type Builder struct {
	imageID string
	lastID  string
	id      string
	mrb     *mruby.Mrb
	client  *client.Client
	config  *container.Config
}

// NewBuilder creates a new builder. Returns error on docker or mruby issues.
func NewBuilder() (*Builder, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return &Builder{mrb: mruby.NewMrb(), client: client, config: &container.Config{}}, nil
}

// AddFunc adds a function to the mruby dispatch as well as adding hooks around
// the call to ensure containers are committed and intermediate layers are
// cleared.
func (b *Builder) AddFunc(name string, fn Func, args mruby.ArgSpec) {
	builderFunc := func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
		val1, val2 := fn(b, m, self)

		if b.id != "" {
			commitResp, err := b.client.ContainerCommit(context.Background(), b.id, types.ContainerCommitOptions{Config: b.config})
			if err != nil {
				return mruby.String(fmt.Sprintf("Error during commit: %v", err)), nil
			}

			err = b.client.ContainerRemove(context.Background(), b.id, types.ContainerRemoveOptions{Force: true})
			if err != nil {
				return mruby.String(fmt.Sprintf("Could not remove intermediate container %q: %v", b.id, err)), nil
			}

			// save for restore later
			wd := b.config.WorkingDir
			user := b.config.User
			cmd := b.config.Cmd

			b.config.WorkingDir = "/"
			b.config.User = "root"
			b.config.Cmd = nil

			defer func() {
				b.config.WorkingDir = wd
				b.config.User = user
				b.config.Cmd = cmd
			}()

			b.config.Image = commitResp.ID

			createResp, err := b.client.ContainerCreate(
				context.Background(),
				b.config,
				nil,
				nil,
				"",
			)
			if err != nil {
				return mruby.String(fmt.Sprintf("Error creating intermediate container: %v", err)), nil
			}

			resp, err := b.client.ContainerCommit(context.Background(), createResp.ID, types.ContainerCommitOptions{Config: b.config})
			if err != nil {
				return mruby.String(fmt.Sprintf("Error during commit: %v", err)), nil
			}

			err = b.client.ContainerRemove(context.Background(), createResp.ID, types.ContainerRemoveOptions{Force: true})
			if err != nil {
				return mruby.String(fmt.Sprintf("Could not remove intermediate container %q: %v", b.id, err)), nil
			}

			if b.imageID != "" {
				_, err := b.client.ImageRemove(context.Background(), b.imageID, types.ImageRemoveOptions{})
				if err != nil {
					return mruby.String(fmt.Sprintf("Error removing parent image: %v", err)), nil
				}
			}

			b.imageID = resp.ID
			b.id = createResp.ID
			fmt.Println("+++ Commit", b.imageID)
		}

		return val1, val2
	}

	b.mrb.TopSelf().SingletonClass().DefineMethod(name, builderFunc, args)
}

// Run the script.
func (b *Builder) Run(script string) (*mruby.MrbValue, error) {
	return b.mrb.LoadString(script)
}

// Close tears down all functions of the builder, preparing it for exit.
func (b *Builder) Close() error {
	b.mrb.Close()
	b.client.ContainerRemove(context.Background(), b.id, types.ContainerRemoveOptions{Force: true})
	return nil
}

func main() {
	builder, err := NewBuilder()
	if err != nil {
		panic(err)
	}

	defer builder.Close()

	for name, def := range jumpTable {
		builder.AddFunc(name, def.Func, def.ArgSpec)
	}

	var content []byte

	if len(os.Args) == 2 {
		content, err = ioutil.ReadFile(os.Args[1])
	} else {
		content, err = ioutil.ReadAll(os.Stdin)
	}
	if err != nil {
		panic(fmt.Sprintf("Could not read input: %v", err))
	}

	response, err := builder.Run(string(content))
	if err != nil {
		panic(fmt.Sprintf("Could not execute ruby: %v", err))
	}

	if response.String() != "" {
		fmt.Printf("+++ Eval: %v\n", response)
	}

	if builder.imageID != "" {
		id := strings.SplitN(builder.imageID, ":", 2)[1]
		fmt.Printf("+++ Finish: %v\n", id)
	}
}
