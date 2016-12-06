package docker

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/erikh/box/copy"
	"github.com/erikh/box/image"
)

func (d *Docker) uploadImage(fn string) (io.Reader, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	imgresp, err := d.client.ImageLoad(context.Background(), f, false)
	if err != nil {
		return nil, err
	}

	return imgresp.Body, nil
}

func (d *Docker) calculateCommits(layers []*image.Layer) []*image.Layer {
	commitLayers := []*image.Layer{}

	for i := 0; i < len(layers); i++ {
		if i >= len(d.layers) {
			break
		}

		commit := true

		for layers[i].LayerID() != d.layers[i] {
			if i == 0 {
				layers = layers[1:]
			} else {
				layers = append(layers[:i-1], layers[i:]...)
			}

			if len(layers) < i || len(d.layers) < i {
				commit = false
				break
			}
		}

		if commit {
			commitLayers = append(commitLayers, layers[i])
		}
	}

	return commitLayers
}

func (d *Docker) downloadImage() (string, error) {
	rc, err := d.client.ImageSave(context.Background(), []string{d.ImageID()})
	if err != nil {
		return "", err
	}

	tf, err := ioutil.TempFile("", "box-downloaded-image")
	if err != nil {
		return "", err
	}

	if err := copy.WithProgress(tf, rc, "Downloading layers"); err != nil {
		tf.Close()
		return "", err
	}

	tf.Close()
	return tf.Name(), nil
}

// MakeImage makes the final image, skipping any layers as necessary. The
// layers must be pre-recorded within the executor. Note that if you have no
// layers to skip, this operation will need to do nothing, so it will do
// nothing.
//
// It returns an error condition, if any.
func (d *Docker) MakeImage() error {
	// this is prinicipally an optimization so we can determine later if we
	// need to reconstruct the image.
	if len(d.skipLayers) == 0 {
		return nil
	}

	tf, err := d.downloadImage()
	if err != nil {
		return err
	}

	layers, dir, err := image.Unpack(tf) // this error is used far below

	if err := os.Remove(tf); err != nil {
		return err
	}

	defer func() {
		if dir != "" {
			os.RemoveAll(dir)
		}
	}()

	if err != nil { // right here
		return err
	}

	if len(layers) < len(d.layers) {
		return fmt.Errorf("error: image mismatch; layers recorded are more than layers in image: %d - %d", len(layers), len(d.layers))
	}

	fn, err := image.Make(d.Config(), d.calculateCommits(layers))
	if err != nil {
		return err
	}

	defer os.Remove(fn)

	reader, err := d.uploadImage(fn)
	if err != nil {
		return err
	}

	d.config.Image, err = printPull(d.tty, reader)

	return err
}
