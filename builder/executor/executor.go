package executor

import (
	"io"

	"github.com/erikh/box/builder/config"
)

// Hook is a hook used in commit calls
type Hook func(id string) (string, error)

// Executor is an engine for talking to different layering/execution context
// subsystems. It is the meat-and-potatoes of image building.
type Executor interface {
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

	// CopyToContainer copies a tarred up series of files (passed in through the
	// io.Reader handle) to the container where they are untarred.
	CopyToContainer(string, string, io.Reader) error

	// CopyFromContainer copies a series of files in a similar fashion to
	// CopyToContainer, just in reverse.
	CopyFromContainer(string, string) (io.Reader, error)

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
	RunHook(string) (string, error)

	// SetStdin turns on the stdin features during run invocations. It is used to
	// facilitate debugging.
	SetStdin(bool)

	// UseCache determines if the cache should be considered or not.
	UseCache(bool)

	// UseTTY determines whether or not to allow docker to use a TTY for both run and pull operations.
	UseTTY(bool)
}
