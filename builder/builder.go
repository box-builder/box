package builder

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/erikh/box/builder/executor"
	"github.com/erikh/box/builder/executor/docker"
	"github.com/erikh/box/builder/signal"
	"github.com/erikh/box/copy"
	"github.com/erikh/box/log"
	"github.com/fatih/color"
	mruby "github.com/mitchellh/go-mruby"
)

// Builder implements the builder core.
type Builder struct {
	mrb       *mruby.Mrb
	useCache  bool
	exec      executor.Executor
	fromImage string
}

func keep(omitFuncs []string, name string) bool {
	for _, fun := range omitFuncs {
		if name == fun {
			return false
		}
	}
	return true
}

// NewBuilder creates a new builder. Returns error on docker or mruby issues.
func NewBuilder(tty bool, omitFuncs []string) (*Builder, error) {
	useCache := os.Getenv("NO_CACHE") == ""

	if !tty {
		color.NoColor = true
		copy.NoTTY = true
	}

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
		if keep(omitFuncs, name) {
			builder.AddVerb(name, def)
		}
	}

	for name, def := range funcJumpTable {
		if keep(omitFuncs, name) {
			inner := def.fun
			fn := func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
				return inner(builder, m, self)
			}

			builder.mrb.TopSelf().SingletonClass().DefineMethod(name, fn, def.argSpec)
		}
	}

	signal.SetSignal(nil)

	return builder, nil
}

// Tag tags the last image yielded by the builder with the provided name.
func (b *Builder) Tag(name string) error {
	return b.exec.Tag(name)
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

func (b *Builder) wrapVerbFunc(name string, vd *verbDefinition) mruby.Func {
	return func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
		strArgs := extractStringArgs(m.GetArgs())
		cacheKey := strings.Join(append([]string{name}, strArgs...), ", ")
		cacheKey = base64.StdEncoding.EncodeToString([]byte(cacheKey))

		log.BuildStep(name, strings.Join(strArgs, ", "))

		if os.Getenv("BOX_DEBUG") != "" {
			content, _ := json.MarshalIndent(b.exec.Config(), "", "  ")
			fmt.Println(string(content))
		}

		cached, err := b.exec.CheckCache(cacheKey)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		// if we don't do this for debug, we will step past it on successive runs
		if !cached || name == "debug" {
			return vd.verbFunc(b, cacheKey, m.GetArgs(), m, self)
		}

		return nil, nil
	}
}

// AddVerb adds a function to the mruby dispatch as well as adding hooks around
// the call to ensure containers are committed and intermediate layers are
// cleared.
func (b *Builder) AddVerb(name string, vd *verbDefinition) {
	b.mrb.TopSelf().SingletonClass().DefineMethod(name, b.wrapVerbFunc(name, vd), vd.argSpec)
}

// RunCode runs the ruby value (a proc) and returns the result.
func (b *Builder) RunCode(val *mruby.MrbValue, stackKeep int) (*mruby.MrbValue, int, error) {
	keep, res, err := b.mrb.RunWithContext(val, b.mrb.TopSelf(), stackKeep)
	if err != nil {
		return nil, keep, err
	}

	if res != nil {
		return res, keep, err
	}

	if err := b.exec.MakeImage(); err != nil {
		return nil, keep, err
	}

	return mruby.String(b.exec.ImageID()).MrbValue(b.mrb), keep, nil
}

// Run the script.
func (b *Builder) Run(script string) (*mruby.MrbValue, error) {
	if _, err := b.mrb.LoadString(script); err != nil {
		return nil, err
	}

	if err := b.exec.MakeImage(); err != nil {
		return nil, err
	}

	return mruby.String(b.exec.ImageID()).MrbValue(b.mrb), nil
}

// Mrb returns the mrb (mruby) instance the builder is using.
func (b *Builder) Mrb() *mruby.Mrb {
	return b.mrb
}

// Close tears down all functions of the builder, preparing it for exit.
func (b *Builder) Close() error {
	b.mrb.EnableGC()
	b.mrb.FullGC()
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
