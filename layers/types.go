package layers

import (
	"io"

	"github.com/box-builder/box/builder/config"
	"github.com/box-builder/box/types"
)

// Image needs a description
type Image interface {
	// Flatten copies a tarred up series of files (passed in through the
	// io.Reader handle) to the image where they are untarred. The first argument
	// is the parent image to use.
	Flatten(io.Reader) error

	// Tag the current layer. Takes a tag name as argument.
	Tag(string) error

	// CheckCache consults the cache to see if there are any items which fit it.
	CheckCache(string) (bool, error)

	// ImageID returns the image identifier of the most recent layer.
	ImageID() string

	// Save saves an image to the provided filename.
	Save(string, string, string) error
}

// Layers needs a description
type Layers interface {
	// Pull an image. Takes a name and returns an image ID+error.
	Fetch(*config.Config, string) (string, error)

	// SetLayers sets the layers.
	SetLayers([]string)

	// AddImage adds layers to the layer list from a provided image, in order of
	// appearance. Any existing layers are skipped over, removing them from the list.
	AddImage(string) error

	// SetSkipLayers toggles whether or not to skip layers that are being built
	// next. Toggle again to re-enable layer recording. The final image will not
	// contain the skipped layers.
	SetSkipLayers(bool)

	// MakeImage makes the final image, skipping any layers as necessary. The
	// layers must be pre-recorded within the executor.
	// It returns an error condition, if any.
	MakeImage(config *config.Config) (string, error)

	// Look up an image identifier.
	Lookup(*config.Config, string) (string, error)
}

// ImageConfig sets the properties used to construct an image
type ImageConfig struct {
	Layers  Layers
	Config  *config.Config
	Globals *types.Global
}
