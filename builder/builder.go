package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	mruby "github.com/mitchellh/go-mruby"
)

// Builder implements the builder core.
type Builder struct {
	imageID    string
	lastID     string
	id         string
	mrb        *mruby.Mrb
	client     *client.Client
	config     *container.Config
	cmd        []string
	entrypoint []string
}

// NewBuilder creates a new builder. Returns error on docker or mruby issues.
func NewBuilder() (*Builder, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	builder := &Builder{mrb: mruby.NewMrb(), client: client, config: &container.Config{}}
	for name, def := range jumpTable {
		builder.AddFunc(name, def.Func, def.ArgSpec)
	}

	return builder, nil
}

// AddFunc adds a function to the mruby dispatch as well as adding hooks around
// the call to ensure containers are committed and intermediate layers are
// cleared.
func (b *Builder) AddFunc(name string, fn Func, args mruby.ArgSpec) {
	builderFunc := func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
		args := m.GetArgs()
		strArgs := []string{}
		for _, arg := range args {
			strArgs = append(strArgs, arg.String())
		}

		fmt.Printf("+++ Execute: %s %s\n", name, strings.Join(strArgs, ", "))
		val1, val2 := fn(b, m, self)

		// save for restore later
		wd := b.config.WorkingDir
		user := b.config.User

		b.config.WorkingDir = wd
		b.config.User = user
		b.config.Cmd = b.cmd
		b.config.Entrypoint = b.entrypoint

		defer func() {
			b.config.WorkingDir = "/"
			b.config.User = "root"
			b.config.Cmd = nil
			b.config.Entrypoint = nil
		}()

		if err := b.commit(); err != nil {
			return mruby.String(fmt.Sprintf("Error creating intermediate container: %v", err)), nil
		}

		fmt.Println("+++ Commit:", b.imageID)

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
	b.client.ContainerRemove(context.Background(), b.id, types.ContainerRemoveOptions{})
	return nil
}
