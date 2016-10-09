package builder

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

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
	"flatten":    {flatten, mruby.ArgsNone()},
	"tag":        {tag, mruby.ArgsReq(1)},
	"copy":       {copy, mruby.ArgsReq(2)},
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

func flatten(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	cont, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer b.client.ContainerRemove(context.Background(), cont.ID, types.ContainerRemoveOptions{})

	rc, err := b.client.ContainerExport(context.Background(), cont.ID)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	f, err := ioutil.TempFile("", "box-flatten.")
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer os.Remove(f.Name())
	if _, err := io.Copy(f, rc); err != nil && err != io.EOF {
		f.Close()
		return nil, createException(m, err.Error())
	}
	f.Close()

	f, err = os.Open(f.Name())
	if err != nil {
		return nil, createException(m, err.Error())
	}

	b.config.Image = ""

	cont2, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer b.client.ContainerRemove(context.Background(), cont2.ID, types.ContainerRemoveOptions{})

	if err := b.client.CopyToContainer(context.Background(), cont2.ID, "/", f, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true}); err != nil {
		fmt.Println("copy")
		return nil, createException(m, err.Error())
	}

	commitResp, err := b.client.ContainerCommit(context.Background(), cont2.ID, types.ContainerCommitOptions{Config: b.config})
	if err != nil {
		fmt.Println("commit")
		return nil, createException(m, err.Error())
	}

	b.imageID = commitResp.ID
	fmt.Printf("+++ Flattened Image: %s\n", b.imageID)
	return nil, nil
}

func tag(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	if len(args) != 1 {
		return nil, createException(m, "tag call expects one argument!")
	}

	if err := b.client.ImageTag(context.Background(), b.imageID, args[0].String()); err != nil {
		return nil, createException(m, err.Error())
	}

	fmt.Printf("+++ Tagged: %q\n", args[0].String())

	return nil, nil
}

func entrypoint(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	stringArgs := []string{}
	for _, arg := range m.GetArgs() {
		stringArgs = append(stringArgs, arg.String())
	}

	b.entrypoint = stringArgs
	b.config.Entrypoint = stringArgs

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

	return args[0], nil
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
	wd := b.config.WorkingDir

	b.config.Entrypoint = []string{"/bin/sh", "-c"}
	b.config.Cmd = stringArgs
	if b.insideDir != "" {
		b.config.WorkingDir = b.insideDir
	}

	defer func() {
		b.config.Entrypoint = entrypoint
		b.config.Cmd = cmd
		b.config.WorkingDir = wd
	}()

	hook := func(b *Builder, id string) error {
		cearesp, err := b.client.ContainerAttach(context.Background(), id, types.ContainerAttachOptions{Stream: true, Stdout: true, Stderr: true})
		if err != nil {
			return fmt.Errorf("Could not attach to container: %v", err)
		}

		err = b.client.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
		if err != nil {
			return fmt.Errorf("Could not start container: %v", err)
		}

		fmt.Println("------ BEGIN OUTPUT ------")

		_, err = io.Copy(os.Stdout, cearesp.Reader)
		if err != nil && err != io.EOF {
			return err
		}

		fmt.Println("------ END OUTPUT ------")

		stat, err := b.client.ContainerWait(context.Background(), id)
		if err != nil {
			return err
		}

		if stat != 0 {
			return fmt.Errorf("Command exited with status %d for container %q", stat, id)
		}

		return nil
	}

	if err := b.commit(hook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func withUser(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.config.User = args[0].String()
	val, err := m.Yield(args[1], args[0])
	b.config.User = ""

	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	return val, nil
}

func inside(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.insideDir = args[0].String()
	val, err := m.Yield(args[1], args[0])
	b.insideDir = ""

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

	if err := b.commit(nil); err != nil {
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

	return nil, nil
}

func copy(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if len(args) != 2 {
		return nil, createException(m, "Did not receive the proper number of arguments in copy")
	}

	source := args[0].String()
	target := args[1].String()

	wd, err := os.Getwd()
	if err != nil {
		return nil, createException(m, err.Error())
	}

	rel, err := filepath.Rel(wd, filepath.Join(wd, source))
	if err != nil {
		return nil, createException(m, err.Error())
	}

	fmt.Printf("+++ Copying: %q to %q\n", rel, target)

	errChan := make(chan error)

	rd, wr := io.Pipe()
	tw := tar.NewWriter(wr)

	hook := func(b *Builder, id string) error {
		dir := b.config.WorkingDir
		if b.insideDir != "" {
			dir = b.insideDir
		}

		return b.client.CopyToContainer(context.Background(), id, dir, rd, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true})
	}

	go func() {
		errChan <- b.commit(hook)
	}()

	fi, err := os.Stat(rel)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	if fi.IsDir() {
		err := filepath.Walk(rel, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if fi.IsDir() {
				return nil
			}

			fmt.Printf("--- COPY: %s\n", path)

			header, err := tar.FileInfoHeader(fi, filepath.Join(target, path))
			if err != nil {
				return err
			}

			header.Name = filepath.Join(target, path)
			header.Linkname = filepath.Join(target, path)

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, err = io.Copy(tw, f)
			if err != nil && err != io.EOF {
				f.Close()
				return err
			}

			f.Close()
			return nil
		})
		if err != nil {
			return nil, createException(m, err.Error())
		}

	} else {
		header, err := tar.FileInfoHeader(fi, target)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		header.Name = target
		header.Linkname = target

		if err := tw.WriteHeader(header); err != nil {
			return nil, createException(m, err.Error())
		}

		f, err := os.Open(rel)
		if err != nil {
			return nil, createException(m, err.Error())
		}
		_, err = io.Copy(tw, f)
		if err != nil && err != io.EOF {
			f.Close()
			return nil, createException(m, err.Error())
		}
		f.Close()
	}

	tw.Close()
	wr.Close()

	if err := <-errChan; err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}
