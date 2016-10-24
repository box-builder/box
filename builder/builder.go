package builder

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/erikh/box/builder/executor"
	"github.com/erikh/box/builder/executor/docker"
	mruby "github.com/mitchellh/go-mruby"
)

// Builder implements the builder core.
type Builder struct {
	useCache bool
	mrb      *mruby.Mrb
	exec     executor.Executor
}

// NewBuilder creates a new builder. Returns error on docker or mruby issues.
func NewBuilder(tty bool) (*Builder, error) {
	useCache := os.Getenv("NO_CACHE") == ""

	exec, err := NewExecutor("docker", useCache, tty)
	if err != nil {
		return nil, err
	}

	builder := &Builder{
		useCache: useCache,
		mrb:      mruby.NewMrb(),
		exec:     exec,
	}

	for name, def := range verbJumpTable {
		builder.AddVerb(name, def.verbFunc, def.argSpec)
	}

	for name, def := range funcJumpTable {
		inner := def.fun
		fn := func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
			return inner(builder, m, self)
		}

		builder.mrb.TopSelf().SingletonClass().DefineMethod(name, fn, def.argSpec)
	}

	return builder, nil
}

// SetCache sets the caching strategy for builds. Turn on to use caching, off
// to not. The default is set to whether or not the environment variable
// (NO_CACHE) is non-empty.
func (b *Builder) SetCache(useCache bool) {
	b.useCache = useCache
	b.exec.UseCache(useCache)
}

// ImageID returns the latest known Image identifier that we committed. At the
// end of the run this will be the golden docker image.
func (b *Builder) ImageID() string {
	return b.exec.ImageID()
}

// AddVerb adds a function to the mruby dispatch as well as adding hooks around
// the call to ensure containers are committed and intermediate layers are
// cleared.
func (b *Builder) AddVerb(name string, fn verbFunc, args mruby.ArgSpec) {
	builderFunc := func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
		args := m.GetArgs()
		strArgs := extractStringArgs(args)
		cacheKey := strings.Join(append([]string{name}, strArgs...), ", ")
		sum := sha512.Sum512_256([]byte(cacheKey))
		cacheKey = base64.StdEncoding.EncodeToString([]byte(sum[:]))

		fmt.Printf("+++ Execute: %s %s\n", name, strings.Join(strArgs, ", "))
		cached, err := b.exec.CheckCache(cacheKey)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		if !cached {
			return fn(b, cacheKey, args, m, self)
		}

		return nil, nil
	}

	b.mrb.TopSelf().SingletonClass().DefineMethod(name, builderFunc, args)
}

// Run the script.
func (b *Builder) Run(script string) (*mruby.MrbValue, error) {
	if _, err := b.mrb.LoadString(script); err != nil {
		return nil, err
	}

	id, err := b.exec.Create()
	if err != nil {
		return nil, err
	}

	defer b.exec.Destroy(id)

	if err := b.exec.Commit("", nil); err != nil {
		return nil, err
	}

	return mruby.String(b.exec.ImageID()).MrbValue(b.mrb), nil
}

// Close tears down all functions of the builder, preparing it for exit.
func (b *Builder) Close() error {
	b.mrb.Close()
	return nil
}

// NewExecutor returns a valid executor for the given name, or error.
func NewExecutor(name string, useCache, tty bool) (executor.Executor, error) {
	switch name {
	case "docker":
		return docker.NewDocker(useCache, tty)
	}

	return nil, fmt.Errorf("Executor %q not found", name)
}
