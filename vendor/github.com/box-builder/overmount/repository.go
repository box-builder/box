package overmount

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

const lockFile = "repository.lock"

// NewRepository constructs a *Repository and creates the dir in which the
// repository lives. A repository is used to hold images and mounts.
func NewRepository(baseDir string, virtual bool) (*Repository, error) {
	return &Repository{
		baseDir:   baseDir,
		layers:    map[string]*Layer{},
		mounts:    []*Mount{},
		editMutex: new(sync.Mutex),
		virtual:   virtual,
	}, os.MkdirAll(baseDir, 0700)
}

// IsVirtual reports if the repository is virtual. Virtual repositories hold
// tars and cannot accept mounts.
func (r *Repository) IsVirtual() bool {
	return r.virtual
}

// TempDir returns a temporary path within the repository
func (r *Repository) TempDir() (string, error) {
	basePath := filepath.Join(r.baseDir, tmpdirBase)
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return "", err
	}
	return ioutil.TempDir(basePath, "")
}

// TempFile returns a temporary file within the repository
func (r *Repository) TempFile() (*os.File, error) {
	basePath := filepath.Join(r.baseDir, tmpdirBase)
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, err
	}
	return ioutil.TempFile(basePath, "")
}

// NewMount creates a new mount for use. Target, lower, and upper correspond to
// specific fields in the mount stanza; read
// https://www.kernel.org/doc/Documentation/filesystems/overlayfs.txt for more
// information.
func (r *Repository) NewMount(target, lower, upper string) (*Mount, error) {
	workDir, err := r.TempDir()
	if err != nil {
		return nil, errors.Wrap(ErrMountCannotProceed, err.Error())
	}

	mount := &Mount{
		target: target,
		upper:  upper,
		lower:  lower,
		work:   workDir,
	}

	if err := r.AddMount(mount); err != nil {
		return nil, err
	}

	return mount, nil
}

func (r *Repository) mkdirCheckRel(path string) error {
	rel, err := filepath.Rel(r.baseDir, path)
	if err != nil {
		return err
	}

	if strings.HasPrefix(rel, "../") {
		return errors.Wrap(ErrMountCannotProceed, "relative path falls below basedir root")
	}

	return os.MkdirAll(path, 0700)
}

func (r *Repository) edit(editFunc func() error) error {
	return edit(path.Join(r.baseDir, lockFile), r.editMutex, editFunc)
}

// AddLayer adds a layer to the repository.
func (r *Repository) AddLayer(layer *Layer, overwrite bool) error {
	return r.edit(func() error {
		if _, ok := r.layers[layer.id]; ok && !overwrite {
			return ErrLayerExists
		}
		r.layers[layer.id] = layer
		return nil
	})
}

// RemoveLayer removes a layer from the repository
func (r *Repository) RemoveLayer(layer *Layer) {
	r.edit(func() error {
		delete(r.layers, layer.id)
		return nil
	})
}

// AddMount adds a layer to the repository.
func (r *Repository) AddMount(mount *Mount) error {
	return r.edit(func() error {
		r.mounts = append(r.mounts, mount)
		return nil
	})
}

// RemoveMount removes a layer from the repository
func (r *Repository) RemoveMount(mount *Mount) {
	r.edit(func() error {
		for i, x := range r.mounts {
			if mount.Equals(x) {
				r.mounts = append(r.mounts[:i], r.mounts[i+1:]...)
			}
		}
		return nil
	})
}

// Import an image (provided over reader) to the repository.
func (r *Repository) Import(i Importer, reader io.ReadCloser) ([]*Layer, error) {
	return i.Import(r, reader)
}

// Export an image (provided via writer) from the repository.
func (r *Repository) Export(e Exporter, layer *Layer, tags []string) (io.ReadCloser, error) {
	return e.Export(r, layer, tags)
}
