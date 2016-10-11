package builder

import (
	"fmt"
	"strings"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types/container"
	mruby "github.com/mitchellh/go-mruby"
)

// Builder implements the builder core.
type Builder struct {
	mrb        *mruby.Mrb
	client     *client.Client
	config     *container.Config
	cmd        []string
	entrypoint []string
	insideDir  string
}

// NewBuilder creates a new builder. Returns error on docker or mruby issues.
func NewBuilder() (*Builder, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	builder := &Builder{mrb: mruby.NewMrb(), client: client, config: &container.Config{}}

	for name, def := range verbJumpTable {
		builder.AddFunc(name, def.verbFunc, def.argSpec)
	}
	for name, def := range mrubyJumpTable {
		builder.mrb.TopSelf().SingletonClass().DefineMethod(name, def.mrubyFunc, def.argSpec)
	}

	builder.entrypoint = []string{"/bin/sh", "-c"}
	builder.cmd = []string{"/bin/sh"}
	builder.resetConfig()

	return builder, nil
}

// ImageID returns the latest known Image identifier that we committed. At the
// end of the run this will be the golden docker image.
func (b *Builder) ImageID() string {
	return b.config.Image
}

// AddFunc adds a function to the mruby dispatch as well as adding hooks around
// the call to ensure containers are committed and intermediate layers are
// cleared.
func (b *Builder) AddFunc(name string, fn verbFunc, args mruby.ArgSpec) {
	builderFunc := func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
		strArgs := extractStringArgs(m)
		cacheKey := strings.Join(append([]string{name}, strArgs...), ", ")
		fmt.Printf("+++ Execute: %s %s\n", name, strings.Join(strArgs, ", "))
		cached, err := b.consultCache(cacheKey)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		if !cached {
			return fn(b, cacheKey, m, self)
		}

		return nil, nil
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
	return nil
}
