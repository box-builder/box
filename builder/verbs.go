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
	"path"
	"path/filepath"
	"strings"

	"github.com/erikh/box/copy"
	"github.com/erikh/box/tar"
	mruby "github.com/mitchellh/go-mruby"
)

// Definition is a jump table definition used for programming the DSL into the
// mruby interpreter.
type verbDefinition struct {
	verbFunc verbFunc
	argSpec  mruby.ArgSpec
}

// verbJumpTable is the dispatch instructions sent to the builder at preparation time.
var verbJumpTable = map[string]*verbDefinition{
	"debug":      {debug, mruby.ArgsNone()},
	"flatten":    {flatten, mruby.ArgsNone()},
	"tag":        {tag, mruby.ArgsReq(1)},
	"copy":       {doCopy, mruby.ArgsReq(2)},
	"from":       {from, mruby.ArgsReq(1)},
	"run":        {run, mruby.ArgsAny()},
	"user":       {user, mruby.ArgsReq(1)},
	"with_user":  {withUser, mruby.ArgsBlock() | mruby.ArgsReq(2)},
	"workdir":    {workdir, mruby.ArgsReq(1)},
	"inside":     {inside, mruby.ArgsBlock() | mruby.ArgsReq(2)},
	"env":        {env, mruby.ArgsAny()},
	"cmd":        {cmd, mruby.ArgsAny()},
	"entrypoint": {entrypoint, mruby.ArgsAny()},
	"set_exec":   {setExec, mruby.ArgsReq(1)},
}

// verbFunc is a builder DSL function used to interact with docker.
type verbFunc func(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value)

func debug(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	var shell string

	if len(args) > 0 {
		shell = args[0].String()
	} else {
		shell = "/bin/bash"
	}

	b.exec.SetStdin(true)
	defer b.exec.SetStdin(false)

	b.exec.Config().TemporaryCommand([]string{}, []string{shell})

	if err := b.exec.Commit(cacheKey, b.exec.RunHook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func setExec(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
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
			b.exec.Config().Entrypoint.Image = strArgs
		case "cmd":
			b.exec.Config().Cmd.Image = strArgs
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

func workdir(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	if !path.IsAbs(args[0].String()) {
		return nil, createException(m, fmt.Sprintf("path %q is not absolute in workdir", args[0].String()))
	}

	b.exec.Config().WorkDir.Image = args[0].String()

	if err := b.exec.Commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func user(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	b.exec.Config().User.Image = args[0].String()

	if err := b.exec.Commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func flatten(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	id, err := b.exec.Create()
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer b.exec.Destroy(id)

	rc, size, err := b.exec.CopyFromContainer(id, "/")
	if err != nil {
		return nil, createException(m, err.Error())
	}

	f, err := ioutil.TempFile("", "box-flatten.")
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer os.Remove(f.Name())
	if err := copy.WithProgress(f, rc, "Downloading image contents to host"); err != nil && err != io.EOF {
		f.Close()
		return nil, createException(m, err.Error())
	}
	f.Close()

	f, err = os.Open(f.Name())
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer f.Close()

	if err := b.exec.Flatten(id, size, f); err != nil {
		return nil, createException(m, err.Error())
	}

	fmt.Printf("+++ Flattened Image: %s\n", b.exec.Config().Image)
	return nil, nil
}

func tag(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
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

	b.logger.Tag(name)

	return nil, nil
}

func entrypoint(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if err := checkImage(b); err != nil {
		return nil, createException(m, err.Error())
	}

	stringArgs := extractStringArgs(args)

	b.exec.Config().Entrypoint.Image = stringArgs

	if err := b.exec.Commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func from(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
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

func run(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	stringArgs := extractStringArgs(args)

	b.exec.Config().TemporaryCommand([]string{"/bin/sh", "-c"}, stringArgs)

	if err := b.exec.Commit(cacheKey, b.exec.RunHook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func withUser(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if err := standardCheck(b, args, 2); err != nil {
		return nil, createException(m, err.Error())
	}

	if args[1].Type() != mruby.TypeProc {
		return nil, createException(m, fmt.Sprintf("Arg %q was not block!", args[1].String()))
	}

	b.exec.Config().User.Temporary = args[0].String()

	val, err := m.Yield(args[1], args[0])
	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	b.exec.Config().User.Temporary = ""

	return val, nil
}

func inside(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if err := standardCheck(b, args, 2); err != nil {
		return nil, createException(m, err.Error())
	}

	if args[1].Type() != mruby.TypeProc {
		return nil, createException(m, fmt.Sprintf("Arg %q was not block!", args[1].String()))
	}

	currentDir := args[0].String()

	if !path.IsAbs(currentDir) {
		currentDir = b.exec.Config().WorkDir.Temporary
		if currentDir == "" {
			currentDir = b.exec.Config().WorkDir.Image
		}

		if currentDir != "" {
			currentDir = path.Join(currentDir, args[0].String())
		} else {
			currentDir = args[0].String()
		}
	}

	if !path.IsAbs(filepath.Clean(currentDir)) {
		return nil, createException(m, fmt.Sprintf("path %q is not absolute in workdir", args[0].String()))
	}

	b.exec.Config().WorkDir.Temporary = currentDir

	val, err := m.Yield(args[1], args[0])
	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	b.exec.Config().WorkDir.Temporary = ""

	return val, nil
}

func env(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
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

func cmd(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if err := checkImage(b); err != nil {
		return nil, createException(m, err.Error())
	}

	stringArgs := extractStringArgs(args)

	b.exec.Config().Cmd.Image = stringArgs

	if err := b.exec.Commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func checkCopyArgs(b *Builder, args []*mruby.MrbValue) (string, string, error) {
	if err := standardCheck(b, args, 2); err != nil {
		return "", "", err
	}

	source, err := filepath.Abs(args[0].String())
	if err != nil {
		return "", "", err
	}

	target := args[1].String()

	wd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	rel, err := filepath.Rel(wd, source)
	if err != nil {
		return "", "", err
	}

	if strings.HasPrefix(rel, "..") {
		return "", "", fmt.Errorf("cannot use relative path %s because it may fall below the root build directory", source)
	}

	workdir := b.exec.Config().WorkDir
	var targetWd string

	if workdir.Temporary == "" {
		targetWd = workdir.Image
	} else {
		targetWd = workdir.Temporary
	}

	// special case `.`
	if target == "." {
		target = filepath.Join(targetWd, rel)
	} else {
		if !strings.HasPrefix(target, "/") {
			target = filepath.Join(targetWd, target)
		}
	}

	return filepath.Clean(rel), target, nil
}

func doCopy(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	rel, target, err := checkCopyArgs(b, args)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	for _, volume := range b.exec.Config().Volumes {
		if strings.HasPrefix(target, volume) {
			return nil, createException(m, fmt.Sprintf("Volume %q cannot be copied into (you tried %q). This is caused by a bug in docker. We are working with docker on a fix.", volume, target))
		}
	}

	fn, cacheKey, err := tar.Archive(b.config.Context, rel, target)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer os.Remove(fn)

	cacheKey = fmt.Sprintf("box:copy %s", cacheKey)

	if b.exec.GetCache() {
		cached, err := b.exec.CheckCache(cacheKey)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		if cached {
			return nil, nil
		}
	}

	f, err := os.Open(fn)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer f.Close()

	hook := func(ctx context.Context, id string) (string, error) {
		return "", b.exec.CopyToContainer(id, f)
	}

	if err := b.exec.Commit(cacheKey, hook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}
