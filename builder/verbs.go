package builder

/*
  verbs.go is a collection of the verbs used to manipulate docker images and tags.

  Please refer to https://erikh.github.io/box/verbs/ for documentation on each
  of the verbs.
*/

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/erikh/box/builder/tar"
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
			b.exec.Config().Entrypoint = strArgs
		case "cmd":
			b.exec.Config().Cmd = strArgs
		default:
			return fmt.Errorf("set_exec only accepts cmd and entrypoint as keys")
		}
		return nil
	})

	if err != nil {
		return nil, createException(m, err.Error())
	}

	if err := b.exec.Commit(cacheKey, nil); err != nil {
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

	b.exec.Config().WorkDir = args[0].String()

	if err := b.exec.Commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func user(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	b.exec.Config().User = args[0].String()

	if err := b.exec.Commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func flatten(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	id, err := b.exec.Create()
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer b.exec.Destroy(id)

	rc, err := b.exec.CopyFromContainer(id, "/")
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

	defer f.Close()

	b.exec.Config().Image = ""

	hook := func(id string) (string, error) {
		if err := b.exec.CopyToContainer(id, "/", f); err != nil {
			return "", err
		}

		return "", nil
	}

	if err := b.exec.Commit("flatten", hook); err != nil {
		return nil, createException(m, err.Error())
	}

	fmt.Printf("+++ Flattened Image: %s\n", b.exec.Config().Image)
	return nil, nil
}

func tag(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	name := args[0].String()

	err := b.exec.Commit(cacheKey, nil)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	if err := b.exec.Tag(name); err != nil {
		return nil, createException(m, err.Error())
	}

	fmt.Printf("+++ Tagged: %q\n", name)

	return nil, nil
}

func entrypoint(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if err := checkImage(b); err != nil {
		return nil, createException(m, err.Error())
	}

	stringArgs := extractStringArgs(m)

	b.exec.Config().Entrypoint = stringArgs
	// override the cmd when the entrypoint is set. this is a tough problem to
	// solve in the right way. If cmd is set prior to this, we cannot be sure
	// once we set the entrypoint that it is still valid, so we erase it.
	b.exec.Config().Cmd = []string{}

	if err := b.exec.Commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func from(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := checkArgs(args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	id, err := b.exec.Fetch(args[0].String())
	if err != nil {
		return nil, createException(m, err.Error())
	}

	b.exec.Config().Image = id

	return mruby.String(id), nil
}

func run(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	stringArgs := extractStringArgs(m)

	entrypoint := b.exec.Config().Entrypoint
	cmd := b.exec.Config().Cmd

	b.exec.Config().Entrypoint = []string{"/bin/sh", "-c"}
	b.exec.Config().Cmd = stringArgs

	defer func() {
		b.exec.Config().Entrypoint = entrypoint
		b.exec.Config().Cmd = cmd
	}()

	if err := b.exec.Commit(cacheKey, b.exec.RunHook); err != nil {
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

	user := b.exec.Config().User
	b.exec.Config().User = args[0].String()

	val, err := m.Yield(args[1], args[0])
	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	b.exec.Config().User = user

	if err := b.exec.Commit(cacheKey, nil); err != nil {
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
	workdir := b.exec.Config().WorkDir
	b.exec.Config().WorkDir = args[0].String()

	val, err := m.Yield(args[1], args[0])
	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	b.exec.Config().WorkDir = workdir

	if err := b.exec.Commit(cacheKey, nil); err != nil {
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
		b.exec.Config().Env = append(b.exec.Config().Env, fmt.Sprintf("%s=%s", key.String(), value.String()))
		return nil
	})

	if err != nil {
		return nil, createException(m, err.Error())
	}

	if err := b.exec.Commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func cmd(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if err := checkImage(b); err != nil {
		return nil, createException(m, err.Error())
	}

	stringArgs := extractStringArgs(m)

	b.exec.Config().Cmd = stringArgs

	if err := b.exec.Commit(cacheKey, nil); err != nil {
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

	target = filepath.Clean(filepath.Join(b.exec.Config().WorkDir, target))

	if strings.HasSuffix(target, "/") {
		target = filepath.Join(target, rel)
	}

	fmt.Printf("+++ Copying: %q to %q\n", rel, target)

	fn, err := tar.Archive(rel, target)
	defer os.Remove(fn)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	cacheKey, err = tar.SumFile(fn)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	cached, err := b.exec.CheckCache(cacheKey)
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

	hook := func(id string) (string, error) {
		defer f.Close()
		return "", b.exec.CopyToContainer(id, "/", f)
	}

	if err := b.exec.Commit(cacheKey, hook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}
