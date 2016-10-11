package builder

import (
	"context"
	"fmt"
	"os"
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
	for name, def := range jumpTable {
		builder.AddFunc(name, def.Func, def.ArgSpec)
	}

	builder.resetConfig()

	return builder, nil
}

func (b *Builder) resetConfig() {
	// TODO make this inherit from teh base image
	b.config.WorkingDir = "/"
	b.config.User = "root"
	b.config.Cmd = nil
	b.config.Entrypoint = []string{"/bin/sh", "-c"}
}

// ImageID returns the latest known Image identifier that we committed. At the
// end of the run this will be the golden docker image.
func (b *Builder) ImageID() string {
	return b.imageID
}

// AddFunc adds a function to the mruby dispatch as well as adding hooks around
// the call to ensure containers are committed and intermediate layers are
// cleared.
func (b *Builder) AddFunc(name string, fn Func, args mruby.ArgSpec) {
	builderFunc := func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
		args := m.GetArgs()
		strArgs := []string{}
		for _, arg := range args {
			if arg.Type() != mruby.TypeProc {
				strArgs = append(strArgs, arg.String())
			}
		}

		cacheKey := strings.Join(append([]string{name}, strArgs...), ", ")
		// comment
		fmt.Printf("+++ Execute: %s %s\n", name, strings.Join(strArgs, ", "))

		if os.Getenv("NO_CACHE") == "" {
			if b.imageID != "" {
				images, err := b.client.ImageList(context.Background(), types.ImageListOptions{All: true})
				if err != nil {
					return nil, createException(m, err.Error())
				}

				for _, img := range images {
					if img.ParentID == b.imageID {
						inspect, _, err := b.client.ImageInspectWithRaw(context.Background(), img.ID)
						if err != nil {
							return nil, createException(m, err.Error())
						}

						if inspect.Comment == cacheKey {
							fmt.Printf("+++ Cache hit: using %q\n", img.ID)
							b.imageID = img.ID
							b.config = inspect.Config
							b.entrypoint = inspect.Config.Entrypoint
							b.cmd = inspect.Config.Cmd

							return nil, nil
						}
					}
				}
			}
		}

		return fn(b, cacheKey, m, self)
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
