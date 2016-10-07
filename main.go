package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	mruby "github.com/mitchellh/go-mruby"
)

// Builder implements the builder core.
type Builder struct {
	imageID string
	lastID  string
	id      string
	mrb     *mruby.Mrb
	client  *client.Client
	config  *container.Config
}

// NewBuilder creates a new builder. Returns error on docker or mruby issues.
func NewBuilder() (*Builder, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return &Builder{mrb: mruby.NewMrb(), client: client, config: &container.Config{}}, nil
}

// AddFunc adds a function to the mruby dispatch as well as adding hooks around
// the call to ensure containers are committed and intermediate layers are
// cleared.
func (b *Builder) AddFunc(name string, fn Func, args mruby.ArgSpec) {
	builderFunc := func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
		val1, val2 := fn(b, m, self)

		if b.id != "" {
			commitResp, err := b.client.ContainerCommit(context.Background(), b.id, types.ContainerCommitOptions{Config: b.config})
			if err != nil {
				return mruby.String(fmt.Sprintf("Error during commit: %v", err)), nil
			}

			err = b.client.ContainerRemove(context.Background(), b.id, types.ContainerRemoveOptions{Force: true})
			if err != nil {
				return mruby.String(fmt.Sprintf("Could not remove intermediate container %q: %v", b.id, err)), nil
			}

			// save for restore later
			wd := b.config.WorkingDir
			user := b.config.User
			cmd := b.config.Cmd
			entrypoint := b.config.Entrypoint

			b.config.WorkingDir = "/"
			b.config.User = "root"
			b.config.Cmd = nil

			defer func() {
				b.config.WorkingDir = wd
				b.config.User = user
				b.config.Cmd = cmd
				b.config.Entrypoint = entrypoint
			}()

			b.config.Image = commitResp.ID

			createResp, err := b.client.ContainerCreate(
				context.Background(),
				b.config,
				nil,
				nil,
				"",
			)
			if err != nil {
				return mruby.String(fmt.Sprintf("Error creating intermediate container: %v", err)), nil
			}

			resp, err := b.client.ContainerCommit(context.Background(), createResp.ID, types.ContainerCommitOptions{Config: b.config})
			if err != nil {
				return mruby.String(fmt.Sprintf("Error during commit: %v", err)), nil
			}

			err = b.client.ContainerRemove(context.Background(), createResp.ID, types.ContainerRemoveOptions{Force: true})
			if err != nil {
				return mruby.String(fmt.Sprintf("Could not remove intermediate container %q: %v", b.id, err)), nil
			}

			if b.imageID != "" {
				_, err := b.client.ImageRemove(context.Background(), b.imageID, types.ImageRemoveOptions{})
				if err != nil {
					return mruby.String(fmt.Sprintf("Error removing parent image: %v", err)), nil
				}
			}

			b.imageID = resp.ID
			b.id = createResp.ID
			fmt.Println("+++ Commit", b.imageID)
		}

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
	b.client.ContainerRemove(context.Background(), b.id, types.ContainerRemoveOptions{Force: true})
	return nil
}

func main() {
	builder, err := NewBuilder()
	if err != nil {
		panic(err)
	}

	defer builder.Close()

	for name, def := range jumpTable {
		builder.AddFunc(name, def.Func, def.ArgSpec)
	}

	var content []byte

	if len(os.Args) == 2 {
		content, err = ioutil.ReadFile(os.Args[1])
	} else {
		content, err = ioutil.ReadAll(os.Stdin)
	}
	if err != nil {
		panic(fmt.Sprintf("Could not read input: %v", err))
	}

	response, err := builder.Run(string(content))
	if err != nil {
		panic(fmt.Sprintf("Could not execute ruby: %v", err))
	}

	if response.String() != "" {
		fmt.Printf("+++ Eval: %v\n", response)
	}

	if builder.imageID != "" {
		id := strings.SplitN(builder.imageID, ":", 2)[1]
		fmt.Printf("+++ Finish: %v\n", id)
	}
}
