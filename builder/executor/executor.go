package executor

import (
	"context"
	"io"

	"github.com/box-builder/box/builder/config"
	"github.com/box-builder/box/layers"
)

// Hook is a hook used in commit calls
type Hook func(context.Context, string) (string, error)

// Executor is an engine for talking to different layering/execution context
// subsystems. It is the meat-and-potatoes of image building.
type Executor interface {
	// SetContext sets the context for subsequent calls.
	SetContext(context.Context)

	// LoadConfig loads the configuration into the executor.
	LoadConfig(*config.Config) error

	// Config returns the current *Config for the executor.
	Config() *config.Config

	// Commit commits an entry to the layer list.
	Commit(string, Hook) error

	// CopyFromContainer copies a series of files in a similar fashion to
	// CopyToContainer, just in reverse.
	CopyFromContainer(string, string) (io.Reader, int64, error)

	// CopyFromContainer copies a series of files in a similar fashion to
	// CopyToContainer, just in reverse.
	CopyToContainer(string, io.Reader) error

	// CopyOneFileFromContainer copies a file from the container and returns its content.
	CopyOneFileFromContainer(string) ([]byte, error)

	// Create a container. Returns the container ID.
	Create() (string, error)

	// Destroy a container by ID.
	Destroy(string) error

	// RunHook is used to manage run invocations, and is processed by the run
	// statement.
	RunHook(context.Context, string) (string, error)

	// SetStdin turns on the stdin features during run invocations. It is used to
	// facilitate debugging.
	SetStdin(bool)

	// UseTTY determines whether or not to allow docker to use a TTY for both run and pull operations.
	UseTTY(bool)

	// ShowRun toggles the visibility of run output.
	ShowRun(bool)

	// GetShowRun returns the visibility of run output.
	GetShowRun() bool

	// Layers returns the layer handler for this executor.
	Layers() layers.Layers

	// Image returns the image handler for this executor.
	Image() layers.Image
}
