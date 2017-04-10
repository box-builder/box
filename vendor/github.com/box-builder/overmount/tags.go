package overmount

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/pkg/errors"
)

const tagsDB = "tags"

var (
	// ErrTagDoesNotExist is reported when the tag retrieved or removed does not exist
	ErrTagDoesNotExist = errors.New("tag does not exist")
)

func (r *Repository) tagFileFor(name string) string {
	return path.Join(r.baseDir, tagsDB, name)
}

// AddTag tags a layer with the name
func (r *Repository) AddTag(name string, layer *Layer) error {
	return r.edit(func() error {
		f, err := r.TempFile()
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.WriteString(layer.ID()); err != nil {
			return err
		}
		f.Close()

		if err := os.MkdirAll(path.Join(r.baseDir, tagsDB), 0700); err != nil {
			return err
		}

		if err := os.Rename(f.Name(), r.tagFileFor(name)); err != nil {
			return err
		}

		return nil
	})
}

// RemoveTag removes a tag by name.
func (r *Repository) RemoveTag(name string) error {
	return r.edit(func() error {
		err := os.Remove(r.tagFileFor(name))
		if os.IsNotExist(err) {
			return errors.Wrap(ErrTagDoesNotExist, "cannot remove")
		}
		return err
	})
}

// GetTag retrieves the layer by the tag name. Returns an error if the tag or
// layer cannot be found. NOTE: the layer is *not* restored.
func (r *Repository) GetTag(name string) (*Layer, error) {
	f, err := os.Open(r.tagFileFor(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrap(ErrTagDoesNotExist, "file not found")
		}

		return nil, err
	}

	defer f.Close()

	id, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	l, err := r.NewLayer(string(id), nil)
	if err != nil {
		return nil, err
	}

	if !l.Exists() {
		return nil, errors.Wrap(ErrTagDoesNotExist, "referenced layer does not exist")
	}

	return l, nil
}
