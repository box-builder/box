package builder

import (
	"context"
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
type verbDefinition struct {
	verbFunc verbFunc
	argSpec  mruby.ArgSpec
}

var verbJumpTable = map[string]verbDefinition{
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

// verbFunc is a builder DSL function used to interact with docker.
type verbFunc func(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value)

func flatten(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	id, err := b.createEmptyContainer()
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer b.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{Force: true})

	rc, _, err := b.client.CopyFromContainer(context.Background(), id, "/")
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

	id2, err := b.createEmptyContainer()
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer b.client.ContainerRemove(context.Background(), id2, types.ContainerRemoveOptions{})

	if err := b.client.CopyToContainer(context.Background(), id2, "/", f, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true}); err != nil {
		return nil, createException(m, err.Error())
	}

	commitResp, err := b.client.ContainerCommit(context.Background(), id2, types.ContainerCommitOptions{Config: b.config})
	if err != nil {
		return nil, createException(m, err.Error())
	}

	b.config.Image = commitResp.ID
	fmt.Printf("+++ Flattened Image: %s\n", b.config.Image)
	return nil, nil
}

func tag(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	if len(args) != 1 {
		return nil, createException(m, "tag call expects one argument!")
	}

	b.resetConfig()

	err := b.commit(cacheKey, nil)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	if err := b.client.ImageTag(context.Background(), b.config.Image, args[0].String()); err != nil {
		return nil, createException(m, err.Error())
	}

	fmt.Printf("+++ Tagged: %q\n", args[0].String())

	return nil, nil
}

func entrypoint(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	stringArgs := []string{}
	for _, arg := range m.GetArgs() {
		stringArgs = append(stringArgs, arg.String())
	}

	b.entrypoint = stringArgs
	b.config.Entrypoint = stringArgs

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func from(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.config.Image = args[0].String()
	b.config.Tty = true
	b.config.AttachStdout = true
	b.config.AttachStderr = true

	inspect, _, err := b.client.ImageInspectWithRaw(context.Background(), args[0].String())
	if err != nil {
		reader, err := b.client.ImagePull(context.Background(), args[0].String(), types.ImagePullOptions{})
		if err != nil {
			return nil, createException(m, err.Error())
		}

		if err := printPull(reader); err != nil {
			return nil, createException(m, err.Error())
		}

		// this will fallthrough to the assignment below
		inspect, _, err = b.client.ImageInspectWithRaw(context.Background(), args[0].String())
		if err != nil {
			return nil, createException(m, err.Error())
		}
	}

	b.config.Image = inspect.ID

	return mruby.String(b.config.Image), nil
}

func run(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if b.config.Image == "" {
		return nil, createException(m, "`from` must precede any `run` statements")
	}

	stringArgs := []string{}
	for _, arg := range m.GetArgs() {
		stringArgs = append(stringArgs, arg.String())
	}

	b.resetConfig()
	b.config.Entrypoint = []string{"/bin/sh", "-c"}
	b.config.Cmd = stringArgs
	b.config.WorkingDir = b.insideDir

	defer func() {
		b.resetConfig()
		b.config.Entrypoint = b.entrypoint
		b.config.Cmd = b.cmd
	}()

	if err := b.commit(cacheKey, runHook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func withUser(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.config.User = args[0].String()
	val, err := m.Yield(args[1], args[0])
	b.config.User = ""

	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	return val, nil
}

func inside(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.insideDir = args[0].String()
	val, err := m.Yield(args[1], args[0])
	b.insideDir = ""

	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	return val, nil
}

func env(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
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

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func cmd(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	stringArgs := []string{}
	for _, arg := range args {
		stringArgs = append(stringArgs, arg.String())
	}

	b.cmd = stringArgs
	b.config.Cmd = stringArgs

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func copy(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if len(args) != 2 {
		return nil, createException(m, "Did not receive the proper number of arguments in copy")
	}

	source := filepath.Clean(args[0].String())
	target := filepath.Clean(args[1].String())

	wd, err := os.Getwd()
	if err != nil {
		return nil, createException(m, err.Error())
	}

	// FIXME do not allow traversing above the wd
	rel, err := filepath.Rel(wd, filepath.Join(wd, source))
	if err != nil {
		return nil, createException(m, err.Error())
	}

	fmt.Printf("+++ Copying: %q to %q\n", rel, target)

	fn, err := tarPath(rel, target)
	defer os.Remove(fn)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	cacheKey, err = sumFile(fn)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	cached, err := b.consultCache(cacheKey)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	if cached {
		return nil, nil
	}

	f, err := os.Open(fn)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	hook := func(b *Builder, id string) (string, error) {
		defer f.Close()
		dir := b.insideDir
		if dir == "" {
			dir = "/"
		}

		return "", b.client.CopyToContainer(context.Background(), id, dir, f, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true})
	}

	if err := b.commit(cacheKey, hook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}
