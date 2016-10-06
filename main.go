package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	mruby "github.com/mitchellh/go-mruby"
)

// Definition is a jump table definition used for programming the DSL into the
// mruby interpreter.
type Definition struct {
	Func    Func
	ArgSpec mruby.ArgSpec
}

func from(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	resp, err := b.client.ContainerCreate(
		context.Background(),
		&container.Config{Image: args[0].String(), Tty: true},
		nil,
		nil,
		"",
	)

	if err != nil {
		return mruby.String(err.Error()), nil
	}

	b.id = resp.ID

	if err := b.client.ContainerStart(context.Background(), b.id, types.ContainerStartOptions{}); err != nil {
		return mruby.String(err.Error()), nil
	}

	return mruby.String(fmt.Sprintf("Response: %v", resp.ID)), nil
}

func run(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if b.id == "" {
		return mruby.String("`from` must be the first docker command`"), nil
	}

	stringArgs := []string{"/bin/sh", "-c"}
	for _, arg := range m.GetArgs() {
		stringArgs = append(stringArgs, arg.String())
	}

	cecresp, err := b.client.ContainerExecCreate(context.Background(), b.id, types.ExecConfig{
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          stringArgs,
	})
	if err != nil {
		return mruby.String(fmt.Sprintf("Error executing: %v", err)), nil
	}

	cearesp, err := b.client.ContainerExecAttach(context.Background(), cecresp.ID, types.ExecConfig{Tty: true, AttachStdout: true, AttachStderr: true})
	if err != nil {
		return mruby.String(fmt.Sprintf("Error attaching to execution context %q: %v", cecresp.ID, err)), nil
	}

	err = b.client.ContainerExecStart(context.Background(), cecresp.ID, types.ExecStartCheck{})
	if err != nil {
		return mruby.String(fmt.Sprintf("Error attaching to execution context %q: %v", cecresp.ID, err)), nil
	}

	_, err = io.Copy(os.Stdout, cearesp.Reader)
	if err != nil && err != io.EOF {
		return mruby.String(err.Error()), nil
	}

	return nil, nil
}

var jumpTable = map[string]Definition{
	"from": {from, mruby.ArgsReq(1)},
	"run":  {run, mruby.ArgsAny()},
}

// Func is a builder DSL function used to interact with docker.
type Func func(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value)

// Builder implements the builder core.
type Builder struct {
	imageID string
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

	return &Builder{mrb: mruby.NewMrb(), client: client}, nil
}

// AddFunc adds a function to the mruby dispatch as well as adding hooks around
// the call to ensure containers are committed and intermediate layers are
// cleared.
func (b *Builder) AddFunc(name string, fn Func, args mruby.ArgSpec) {
	builderFunc := func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
		val1, val2 := fn(b, m, self)

		if b.id != "" {
			resp, err := b.client.ContainerCommit(context.Background(), b.id, types.ContainerCommitOptions{Config: b.config})
			if err != nil {
				return mruby.String(fmt.Sprintf("Error during commit: %v", err)), nil
			}

			if b.imageID != "" {
				_, err := b.client.ImageRemove(context.Background(), b.imageID, types.ImageRemoveOptions{})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error removing parent image: %v", err)
				}
			}

			b.imageID = resp.ID
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

	_, err = builder.Run(string(content))
	if err != nil {
		panic(fmt.Sprintf("Could not execute ruby: %v", err))
	}

	fmt.Printf("success: %v\n", strings.SplitN(builder.imageID, ":", 2)[1])
}
