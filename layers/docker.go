package layers

import (
	"context"
	"io"
	"os"

	"github.com/box-builder/box/builder/config"
	"github.com/box-builder/box/fetcher"
	"github.com/box-builder/box/image"
	"github.com/box-builder/box/types"
	"github.com/docker/docker/client"
)

const megaByte = 1024 * 1024

// Docker needs a documetnation
type Docker struct {
	doSkipLayers bool
	skipLayers   []string
	layers       []string
	images       []string
	client       *client.Client
	layerSet     map[string]struct{}
	globals      *types.Global
}

// NewDocker needs a documetnation
func NewDocker(globals *types.Global) (*Docker, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return &Docker{
		client:     client,
		globals:    globals,
		layerSet:   map[string]struct{}{},
		images:     []string{},
		skipLayers: []string{},
		layers:     []string{},
	}, nil
}

// AddImage adds layers to the layer list from a provided image, in order of
// appearance. Any existing layers are skipped over, removing them from the list.
func (d *Docker) AddImage(image string) error {
	d.images = append(d.images, image)

	resp, _, err := d.client.ImageInspectWithRaw(d.globals.Context, image)
	if err != nil {
		return err
	}

	for _, layer := range resp.RootFS.Layers {
		if _, ok := d.layerSet[layer]; !ok {
			if !d.doSkipLayers {
				// XXX this really worries me. Pretty sure there's a potential cache
				// fail/poison here, but I have to debug it.
				// BETA QUALITY YO
				d.layers = append(d.layers, layer)
			} else {
				// this is principally an optimization so we can determine later if we
				// need to reconstruct the image.
				d.skipLayers = append(d.skipLayers, layer)
			}

			d.layerSet[layer] = struct{}{}
		}
	}

	return nil
}

// SetSkipLayers toggles whether or not to skip layers that are being built
// next. Toggle again to re-enable layer recording. The final image will not
// contain the skipped layers.
func (d *Docker) SetSkipLayers(ok bool) {
	d.doSkipLayers = ok
}

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

// Lookup an image by name, returning the id.
func (d *Docker) Lookup(name string) (string, error) {
	img, _, err := d.client.ImageInspectWithRaw(d.globals.Context, name)
	if err != nil {
		return "", err
	}

	return img.ID, nil
}

// Fetch retrieves a docker image, overwrites the container configuration, and
// returns its id.
func (d *Docker) Fetch(config *config.Config, name string) (string, error) {
	location, layers, err := fetcher.Docker(d.globals.Context, d.globals, d.client, config, name)
	if err != nil {
		return "", err
	}

	d.SetLayers(layers)
	return location, nil
}

// SetLayers sets the layers.
func (d *Docker) SetLayers(layers []string) {
	d.layers = layers
}
