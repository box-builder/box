package builder

/*
  verbs.go is a collection of the verbs used to manipulate docker images and tags.

  Please refer to https://erikh.github.io/box/verbs/ for documentation on each
  of the verbs.
*/

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/engine-api/types"
	mruby "github.com/mitchellh/go-mruby"
)

// Definition is a jump table definition used for programming the DSL into the
// mruby interpreter.
type verbDefinition struct {
	verbFunc verbFunc
	argSpec  mruby.ArgSpec
}

// verbJumpTable is the dispatch instructions sent to the builder at preparation time.
var verbJumpTable = map[string]verbDefinition{
	"flatten":    {flatten, mruby.ArgsNone()},
	"tag":        {tag, mruby.ArgsReq(1)},
	"copy":       {copy, mruby.ArgsReq(2)},
	"from":       {from, mruby.ArgsReq(1)},
	"run":        {run, mruby.ArgsAny()},
	"user":       {user, mruby.ArgsReq(1)},
	"with_user":  {withUser, mruby.ArgsBlock() | mruby.ArgsReq(1)},
	"workdir":    {workdir, mruby.ArgsReq(1)},
	"inside":     {inside, mruby.ArgsBlock() | mruby.ArgsReq(1)},
	"env":        {env, mruby.ArgsAny()},
	"cmd":        {cmd, mruby.ArgsAny()},
	"entrypoint": {entrypoint, mruby.ArgsAny()},
	"set_exec":   {setExec, mruby.ArgsReq(1)},
}

// verbFunc is a builder DSL function used to interact with docker.
type verbFunc func(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value)

func setExec(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	err := iterateRubyHash(args[0], func(key, value *mruby.MrbValue) error {
		if value.Type() != mruby.TypeArray {
			return fmt.Errorf("Value for key %q is not array, must be array", key.String())
		}

		strArgs := []string{}
		a := value.Array()

		for i := 0; i < a.Len(); i++ {
			val, err := a.Get(i)
			if err != nil {
				return err
			}
			strArgs = append(strArgs, val.String())
		}

		switch key.String() {
		case "entrypoint":
			b.entrypoint = strArgs
		case "cmd":
			b.cmd = strArgs
		default:
			return fmt.Errorf("set_exec only accepts cmd and entrypoint as keys")
		}
		return nil
	})

	if err != nil {
		return nil, createException(m, err.Error())
	}

	b.resetConfig()

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func workdir(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	// FIXME must be absolute path, fix & test this.

	b.workdir = args[0].String()
	b.config.WorkingDir = args[0].String()

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func user(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	b.user = args[0].String()
	b.config.User = args[0].String()

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

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

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
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
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	stringArgs := []string{}
	for _, arg := range args {
		stringArgs = append(stringArgs, arg.String())
	}

	b.entrypoint = stringArgs
	b.config.Entrypoint = stringArgs
	// override the cmd when the entrypoint is set. this is a tough problem to
	// solve in the right way. If cmd is set prior to this, we cannot be sure
	// once we set the entrypoint that it is still valid, so we erase it.
	// FIXME
	// should install a new call which sets both at the same time.
	b.cmd = []string{}
	b.config.Cmd = []string{}

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func from(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := checkArgs(args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

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
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	stringArgs := []string{}
	for _, arg := range args {
		stringArgs = append(stringArgs, arg.String())
	}

	b.resetConfig()
	b.config.Entrypoint = []string{"/bin/sh", "-c"}
	b.config.Cmd = stringArgs

	defer b.resetConfig()

	if err := b.commit(cacheKey, runHook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func withUser(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 2); err != nil {
		return nil, createException(m, err.Error())
	}

	if args[1].Type() != mruby.TypeProc {
		return nil, createException(m, fmt.Sprintf("Arg %q was not block!", args[1].String()))
	}

	user := b.user
	b.user = args[0].String()
	b.resetConfig()
	val, err := m.Yield(args[1], args[0])
	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	b.user = user
	b.resetConfig()

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return val, nil
}

func inside(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 2); err != nil {
		return nil, createException(m, err.Error())
	}

	if args[1].Type() != mruby.TypeProc {
		return nil, createException(m, fmt.Sprintf("Arg %q was not block!", args[1].String()))
	}

	// FIXME must be absolute path, fix & test this.
	workdir := b.workdir
	b.workdir = args[0].String()
	b.resetConfig()

	val, err := m.Yield(args[1], args[0])
	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	b.workdir = workdir
	b.resetConfig()

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return val, nil
}

func env(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	err := iterateRubyHash(args[0], func(key, value *mruby.MrbValue) error {
		b.config.Env = append(b.config.Env, fmt.Sprintf("%s=%s", key.String(), value.String()))
		return nil
	})

	if err != nil {
		return nil, createException(m, err.Error())
	}

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func cmd(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

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

	if err := standardCheck(b, args, 2); err != nil {
		return nil, createException(m, err.Error())
	}

	source := args[0].String()
	target := args[1].String()

	wd, err := os.Getwd()
	if err != nil {
		return nil, createException(m, err.Error())
	}

	// FIXME do not allow traversing above the wd
	rel, err := filepath.Rel(wd, filepath.Join(wd, source))
	if err != nil {
		return nil, createException(m, err.Error())
	}

	paths := filepath.SplitList(rel)
	for _, path := range paths {
		if path == ".." {
			return nil, createException(m, fmt.Sprintf("Cannot use relative path %s because it may fall below the root build directory", source))
		}
	}

	target = filepath.Clean(filepath.Join(b.config.WorkingDir, target))

	if strings.HasSuffix(target, "/") {
		target = filepath.Join(target, rel)
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
		return "", b.client.CopyToContainer(context.Background(), id, "/", f, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true})
	}

	if err := b.commit(cacheKey, hook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}
