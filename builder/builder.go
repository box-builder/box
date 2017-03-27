package builder

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/box-builder/box/builder/command"
	"github.com/box-builder/box/builder/evaluator"
	"github.com/box-builder/box/builder/evaluator/mruby"
	"github.com/box-builder/box/builder/executor"
	"github.com/box-builder/box/builder/executor/docker"
	"github.com/box-builder/box/copy"
	"github.com/box-builder/box/logger"
	"github.com/box-builder/box/types"
	"github.com/fatih/color"
)

// BuildConfig is a struct containing the configuration for the builder.
type BuildConfig struct {
	Globals  *types.Global
	Runner   chan struct{}
	FileName string
}

// Builder implements the builder core.
type Builder struct {
	config *BuildConfig
	exec   executor.Executor
	eval   evaluator.Evaluator
}

// NewBuilder creates a new builder. Returns error on docker or mruby issues.
func NewBuilder(bc BuildConfig) (*Builder, error) {
	if bc.Globals == nil {
		bc.Globals = &types.Global{Context: context.Background()}
	}

	if !bc.Globals.TTY {
		color.NoColor = true
		copy.NoTTY = true
	}

	if bc.Globals.Logger == nil {
		bc.Globals.Logger = logger.New(bc.FileName, true)
	}

	exec, err := NewExecutor("docker", bc.Globals)
	if err != nil {
		return nil, err
	}

	eval, err := mruby.NewMRuby(&mruby.Config{
		Filename: bc.FileName,
		Globals:  bc.Globals,
		Exec:     exec,
		Interp:   command.NewInterpreter(bc.Globals, exec),
	})
	if err != nil {
		return nil, err
	}

	return &Builder{
		config: &bc,
		exec:   exec,
		eval:   eval,
	}, nil
}

// Config returns the BuildConfig associated with this builder
func (b *Builder) Config() *BuildConfig {
	return b.config
}

// Result returns the latest cached result from any run invocation. The
// behavior is undefined if called before any Run()-style invocation.
func (b *Builder) Result() types.BuildResult {
	return b.eval.Result()
}

// Run runs the script set by the BuildConfig. It closes the run channel when
// it finishes.
func (b *Builder) Run() types.BuildResult {
	defer close(b.config.Runner)

	script, err := ioutil.ReadFile(b.config.FileName)
	if err != nil {
		return types.BuildResult{
			FileName: b.config.FileName,
			Err:      err,
		}
	}

	b.eval.RunScript(string(script))
	return b.Result()
}

// Wait waits for the build to complete.
func (b *Builder) Wait() types.BuildResult {
	<-b.config.Runner
	return b.Result()
}

// Tag the image with the name
func (b *Builder) Tag(tag string) error {
	return b.exec.Image().Tag(tag)
}

// Close tears down all functions of the builder, preparing it for exit.
func (b *Builder) Close() error {
	return b.eval.Close()
}

// NewExecutor returns a valid executor for the given name, or error.
func NewExecutor(name string, globals *types.Global) (executor.Executor, error) {
	switch name {
	case "docker":
		return docker.NewDocker(globals)
	}

	return nil, fmt.Errorf("Executor %q not found", name)
}
