// Package overmount - mount tars in an overlay filesystem
//
// overmount is intended to mount docker images, or work with similar
// functionality to achieve a series of layered filesystems which can be composed
// into an image.
//
// See the examples/ directory for examples of how to use the API.
//
// github.com/pkg/errors.Wrap is in use with many of our errors; look at the
// errors.Cause API in that package for more information on how to extract the
// static error constants.
package overmount

import (
	"io"
	"sync"

	"github.com/pkg/errors"
)

var (
	// ErrParentNotMounted is returned when the parent layer is not mounted (but exists)
	ErrParentNotMounted = errors.New("parent not mounted, cannot continue")

	// ErrMountFailed returns an underlying error when the mount has failed.
	ErrMountFailed = errors.New("mount failed")

	// ErrUnmountFailed returns an underlying error when the unmount has failed.
	ErrUnmountFailed = errors.New("unmount failed")

	// ErrMountCannotProceed returns an underlying error when the mount cannot be processed.
	ErrMountCannotProceed = errors.New("mount cannot proceed")

	// ErrImageCannotBeComposed is returned when an image (a set of layers) fails validation.
	ErrImageCannotBeComposed = errors.New("image cannot be composed")

	// ErrInvalidAsset is returned when the asset cannot be used.
	ErrInvalidAsset = errors.New("invalid asset")

	// ErrInvalidLayer is returned when the layer cannot be used.
	ErrInvalidLayer = errors.New("invalid layer")

	// ErrLayerExists is called when a layer id already exists in the repository.
	ErrLayerExists = errors.New("layer already exists")

	// ErrMountExists is called when a mount already exists in the repository.
	ErrMountExists = errors.New("mount already exists")
)

const (
	tmpdirBase = "tmp"
	mountBase  = "mount"
	layerBase  = "layers"
)

// Repository is a collection of mounts/layers. Repositories have a base path
// and a collection of layers and mounts. Overlay work directories are stored
// in `tmp`.
//
// In summary:
//
//     basedir/
//        layers/
//          layer-id/
//          top-layer/
//        tmp/
//          some-random-workdir/
//        mount/
//          another-layer-id/
//          top-layer/
//
// Repositories can hold any number of mounts and layers. They do not
// necessarily need to be related.
type Repository struct {
	baseDir string
	layers  map[string]*Layer
	mounts  []*Mount
	virtual bool

	editMutex *sync.Mutex
}

// Mount represents a single overlay mount. The lower value is computed from
// the parent layer of the layer provided to the NewMount call. The target and
// upper dirs are computed from the passed layer.
type Mount struct {
	target     string
	upper      string
	lower      string
	repository *Repository
	work       string
	mounted    bool
}

// Layer is the representation of a filesystem layer. Layers are organized in a
// reverse linked-list from topmost layer to the root layer. In an
// (*Image).Mount() scenario, the layers are mounted from the bottom up to
// culminate in a mount path that represents the top-most layer merged with all
// the lower layers.
//
// See https://www.kernel.org/doc/Documentation/filesystems/overlayfs.txt for
// more information on mount flags.
type Layer struct {
	Parent *Layer

	id         string
	asset      *Asset
	repository *Repository
	virtual    bool

	editMutex *sync.Mutex
}

// Image is the representation of a set of sequential layers to be mounted.
type Image struct {
	repository *Repository
	layer      *Layer
	mount      *Mount
}

// Importer is an interface to image importers; ways to get images into
// overmount repositories.
type Importer interface {
	// Import takes a tar represented as an io.ReadCloser, and converts and unpacks
	// it into the overmount repository.  Returns the top-most layer and any
	// error.
	Import(*Repository, io.ReadCloser) ([]*Layer, error)
}

// Exporter is an interface to image exporters; ways to get images out of
// overmount repositories.
type Exporter interface {
	// Export produces a tar represented as an io.ReadCloser from the Layer provided.
	Export(*Repository, *Layer, []string) (io.ReadCloser, error)
}
