package executor

import (
	"context"
	"io"

	"github.com/erikh/box/builder/config"
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

	// ImageID returns the image identifier of the most recent layer.
	ImageID() string

	// Commit commits an entry to the layer list.
	Commit(string, Hook) error

	// CheckCache consults the cache to see if there are any items which fit it.
	CheckCache(string) (bool, error)

	// Flatten copies a tarred up series of files (passed in through the
	// io.Reader handle) to the image where they are untarred. The first argument
	// is the parent image to use.
	Flatten(string, int64, io.Reader) error

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

	// Tag the current layer. Takes a tag name as argument.
	Tag(string) error

	// Pull an image. Takes a name and returns an image ID+error.
	Fetch(string) (string, error)

	// RunHook is used to manage run invocations, and is processed by the run
	// statement.
	RunHook(context.Context, string) (string, error)

	// SetStdin turns on the stdin features during run invocations. It is used to
	// facilitate debugging.
	SetStdin(bool)

	// UseCache determines if the cache should be considered or not.
	UseCache(bool)

	// GetCache gets the current value of whether or not to use the cache
	GetCache() bool

	// UseTTY determines whether or not to allow docker to use a TTY for both run and pull operations.
	UseTTY(bool)

	// SetSkipLayers toggles whether or not to skip layers that are being built
	// next. Toggle again to re-enable layer recording. The final image will not
	// contain the skipped layers.
	SetSkipLayers(bool)

	// MakeImage makes the final image, skipping any layers as necessary. The
	// layers must be pre-recorded within the executor.
	// It returns an error condition, if any.
	MakeImage() error

	// CleanupImages cleans up all intermediate images.
	CleanupImages()

	// ShowRun toggles the visibility of run output.
	ShowRun(bool)
}
