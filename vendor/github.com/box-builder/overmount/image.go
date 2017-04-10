package overmount

import (
	"os"

	"github.com/pkg/errors"
)

// NewImage preps a set of layers to be a part of an image. There must be at least two layers
func (r *Repository) NewImage(topLayer *Layer) *Image {
	return &Image{repository: r, layer: topLayer}
}

// Mount mounts an image with the specified layer as its highest element.
// Images must have at least two layers to be mounted. If you need to work with
// the first layer, operate on the layer directly with the Asset interface.
//
// Call unmount to undo this operation.
func (i *Image) Mount() error {
	if i.repository.IsVirtual() {
		return errors.Wrap(ErrMountCannotProceed, "cannot mount in virtual repository")
	}

	upper := i.layer.Path()
	target := i.layer.MountPath()

	if fi, err := os.Stat(target); err == nil && fi.IsDir() {
		return errors.Wrap(ErrMountCannotProceed, "mount exists")
	}

	layer := i.layer.Parent
	if layer == nil {
		return errors.Wrap(ErrMountCannotProceed, "must have at least two layers")
	}

	lower := ""

	for layer != nil {
		if err := i.repository.mkdirCheckRel(layer.Path()); err != nil {
			return err
		}
		if lower != "" {
			lower = layer.Path() + ":" + lower
		} else {
			lower = layer.Path()
		}
		layer = layer.Parent
	}

	for _, path := range []string{target, upper} {
		if err := i.repository.mkdirCheckRel(path); err != nil {
			return errors.Wrap(ErrMountCannotProceed, err.Error())
		}
	}

	mount, err := i.repository.NewMount(target, lower, upper)
	if err != nil {
		return err
	}

	i.mount = mount

	return mount.Open()
}

// Unmount unmounts the image. This does not affect layer storage.
func (i *Image) Unmount() error {
	if i.mount == nil {
		if err := forceUnmount(i.layer.MountPath()); err != nil {
			return errors.Wrap(ErrMountCannotProceed, err.Error())
		}
		return nil
	}

	return i.mount.Close()
}

// Commit saves all the parents
func (i *Image) Commit() error {
	for iter := i.layer; iter != nil; iter = iter.Parent {
		if err := iter.SaveParent(); err != nil {
			return err
		}
	}

	return nil
}
