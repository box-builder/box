package overmount

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"

	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

const (
	rootFSPath       = "rootfs"
	parentPath       = "parent"
	configPath       = "config.json"
	lockFilePath     = "lockfile"
	virtualLayerPath = "layer.tar"
)

// CreateLayer prepares a new layer for work and creates it in the repository.
func (r *Repository) CreateLayer(id string, parent *Layer) (*Layer, error) {
	return r.newLayer(id, parent, true)
}

// NewLayer prepares a new layer for work but DOES NOT add it to the
// repository. The ID is the directory that will be created in the repository;
// see NewRepository for more info. If the layer is already in the repository
// and known, it will be returned and no file operations or checks will be
// performed. The layer may not actually exist at this point.
func (r *Repository) NewLayer(id string, parent *Layer) (*Layer, error) {
	if layer, ok := r.layers[id]; ok {
		return layer, nil
	}

	return r.newLayer(id, parent, false)
}

func (r *Repository) newLayer(id string, parent *Layer, create bool) (*Layer, error) {
	var err error

	layer := &Layer{
		Parent: parent,

		id:         id,
		repository: r,
		editMutex:  new(sync.Mutex),
	}

	if create && !layer.Exists() {
		if err := layer.Create(); err != nil {
			return layer, err // return the layer here (document later) in case they need to clean it up.
		}
	}

	if err := r.AddLayer(layer); err != nil {
		return nil, err
	}

	layer.asset, err = NewAsset(layer.Path(), digest.SHA256.Digester(), r.IsVirtual())
	if err != nil {
		return nil, err
	}

	return layer, nil
}

func (l *Layer) edit(editFunc func() error) (retErr error) {
	return edit(path.Join(l.layerBase(), lockFilePath), l.editMutex, editFunc)
}

// ID returns the ID of the layer.
func (l *Layer) ID() string {
	return l.id
}

// MountPath gets the mount path for a given subpath.
func (l *Layer) MountPath() string {
	return filepath.Join(l.repository.baseDir, mountBase, l.id)
}

// Exists indicates whether or not a layer already exists.
func (l *Layer) Exists() bool {
	fi, err := os.Stat(l.layerBase())
	if err != nil {
		return false
	}

	return fi.IsDir()
}

// Digest returns the digest.Digest for the layer and any error.
func (l *Layer) Digest() (digest.Digest, error) {
	return l.asset.LoadDigest()
}

// Create creates the layer and makes it available for use, if possible.
// Otherwise, it returns an error.
func (l *Layer) Create() error {
	return checkDir(l.layerBase(), ErrInvalidLayer)
}

func (l *Layer) layerBase() string {
	return filepath.Join(l.repository.baseDir, layerBase, l.id)
}

// Path gets the layer store path for a given subpath.
func (l *Layer) Path() string {
	if l.repository.IsVirtual() {
		return filepath.Join(l.layerBase(), virtualLayerPath)
	}

	return filepath.Join(l.layerBase(), rootFSPath)
}

func (l *Layer) parentPath() string {
	return filepath.Join(l.layerBase(), parentPath)
}

func (l *Layer) configPath() string {
	return filepath.Join(l.layerBase(), configPath)
}

// Config returns a reference to the image configuration for this layer.
func (l *Layer) Config() (*ImageConfig, error) {
	f, err := os.Open(l.configPath())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var i ImageConfig
	return &i, json.NewDecoder(f).Decode(&i)
}

// SaveConfig writes a *v1.Image configuration to the repository for the layer.
func (l *Layer) SaveConfig(config *ImageConfig) error {
	return l.edit(func() error {
		f, err := os.Create(l.configPath())
		if err != nil {
			return err
		}
		defer f.Close()

		return json.NewEncoder(f).Encode(config)
	})
}

// SaveParent will silently only save the
func (l *Layer) SaveParent() error {
	return l.edit(func() error {
		if l.Parent == nil {
			return nil
		}

		fi, err := os.Stat(l.parentPath())
		if err != nil {
			if os.IsNotExist(err) {
				return l.overwriteParent()
			}
			return err
		} else if !fi.Mode().IsRegular() {
			return errors.Wrap(ErrInvalidLayer, "parent configuration is invalid")
		}

		return nil
	})
}

// OverwriteParent overwrites the parent setting for this layer.
func (l *Layer) overwriteParent() error {
	if l.Parent == nil {
		return nil
	}

	return ioutil.WriteFile(l.parentPath(), []byte(l.Parent.ID()), 0600)
}

// LoadParent loads only the parent for this specific instance. See
// RestoreParent for restoring the whole chain.
func (l *Layer) LoadParent() error {
	id, err := ioutil.ReadFile(l.parentPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if len(id) == 0 {
		return nil
	}

	parent, err := l.repository.NewLayer(string(id), nil)
	if err != nil {
		return err
	}

	fi, err := os.Stat(parent.layerBase())
	if err != nil || !fi.IsDir() {
		return errors.Wrap(ErrInvalidLayer, parent.layerBase())
	}

	l.Parent = parent

	return nil
}

// RestoreParent reads any parent file and sets the layer accordingly. It does this recursively.
func (l *Layer) RestoreParent() error {
	if err := l.LoadParent(); err != nil {
		return err
	}

	if l.Parent != nil {
		return l.Parent.RestoreParent()
	}

	return nil
}

// Unpack unpacks the asset into the layer Path(). It returns the computed digest.
func (l *Layer) Unpack(reader io.Reader) (digest.Digest, error) {
	err := l.edit(func() error { return l.asset.Unpack(reader) })
	return l.asset.Digest(), err
}

// Pack archives the layer to the writer as a tar file.
func (l *Layer) Pack(writer io.Writer) (digest.Digest, error) {
	err := l.edit(func() error { return l.asset.Pack(writer) })
	return l.asset.Digest(), err
}

// Remove a layer from the filesystem and the repository.
func (l *Layer) Remove() error {
	return l.edit(func() error {
		l.repository.RemoveLayer(l)
		return os.RemoveAll(l.layerBase())
	})
}
