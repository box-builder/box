package builder

/*
  verbs.go is a collection of the verbs used to manipulate docker images and tags.
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
}

// verbFunc is a builder DSL function used to interact with docker.
type verbFunc func(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value)

// workdir sets the WorkingDir in the docker environment. It sets this
// throughout the image creation; all run/copy statements will respect this
// value. If you wish to break out or work within it further, look at the
// `inside` call.
func workdir(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	if len(args) != 1 {
		return nil, createException(m, fmt.Sprintf("This call only accepts one argument; you provided %d.", len(args)))
	}

	// FIXME must be absolute path, fix & test this.

	b.workdir = args[0].String()
	b.config.WorkingDir = args[0].String()

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

// user sets the username this container will use by default. It also affects
// following run statements (but not copy, which always copies as root
// currently). If you wish to switch to a user temporarily, consider using
// `with_user`.
func user(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	if len(args) != 1 {
		return nil, createException(m, fmt.Sprintf("This call only accepts one argument; you provided %d.", len(args)))
	}

	b.user = args[0].String()
	b.config.User = args[0].String()

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

// flatten requires no argumemnts and flattens all layers and commits a new
// layer. This is useful for reducing the size of images or making them easier
// to distribute.
//
// NOTE: flattening will always bust the build cache.
//
// NOTE: flattening requires downloading the image and re-uploading it. This
// can take a lot of time over remote connections and is not advised.
//
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

// tag tags an image within the docker daemon, named after the string provided.
// It must be a valid tag name.
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

// entrypoint sets the entrypoint for the image at runtime. It will not be
// used for run invocations. Note that setting this clears any previously set cmd.
func entrypoint(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	stringArgs := []string{}
	for _, arg := range m.GetArgs() {
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

// from sets the initial image and if necessary, pulls it from the registry. It
// also sets the initial layer and must be called before several operations.
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

// run runs a command and saves the layer.
//
// It respects user and workdir, but not entrypoint and command. It does this
// so it can respect the values provided in the script instead of what was
// intended for the final image.
//
// Cache keys are generated based on the command name, so to be certain your
// command is run in the event of it hitting cache, run box with NO_CACHE=1.
//
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

	defer b.resetConfig()

	if err := b.commit(cacheKey, runHook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

// with_user, when provided with a string username and block invokes commands
// within the user's login context. Unfortunately, copy does not respect this
// yet. It does not affect the final image.
//
// Example:
//
//    with_user "erikh" do
//      run "vim +PluginInstall +qall"
//    end
//
func withUser(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

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

// inside, when provided with a directory name string and block, invokes
// commands within the context of the working directory being set to the
// string. It does not affect the final image.
//
// Example:
//
//    inside "/dev" do
//      run "mknod webscale c 1 3"
//    end
//
func inside(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

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

// env, when provided with a hash of string => string key/value combinations,
// will set the environment in the image and future run invocations.
//
// Example:
//
//    env "GOPATH" => "/go"
//
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

// cmd, when provided with a string will set the docker image's Cmd property,
// which are the arguments that follow the entrypoint (and are overridden when
// you provide a command to `docker run`). It does not affect run invocations.
//
// Note that if you set this before entrypoint, it will be cleared.
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

// Copy copies files from the host to the container. It only works relative to
// the current directory. The build cache is calculated by summing the tar
// result of edited files. Since mtime is also considered, changes to that will
// also bust the cache.
//
// NOTE: copy does not respect inside or workdir right now, this is a bug.
//
// NOTE: copy does not respect user permissions when the `user` or `with_user`
// modifiers are applied. This is also a bug, but a much harder to fix one.
//
// Example:
//
//    copy ".", "test"
//
func copy(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
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

	// FIXME do not allow traversing above the wd
	rel, err := filepath.Rel(wd, filepath.Join(wd, source))
	if err != nil {
		return nil, createException(m, err.Error())
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
