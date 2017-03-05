package layers

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/containers/image/copy"
	"github.com/containers/image/docker/daemon"
	"github.com/containers/image/signature"
	ctypes "github.com/containers/image/types"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/client"
	"github.com/erikh/box/builder/config"
	"github.com/erikh/box/fetcher"
	"github.com/erikh/box/image"
	"github.com/erikh/box/logger"
)

const megaByte = 1024 * 1024

// Docker needs a documetnation
type Docker struct {
	context      context.Context
	tty          bool
	doSkipLayers bool
	skipLayers   []string
	layers       []string
	images       []string
	client       *client.Client
	layerSet     map[string]struct{}
	logger       *logger.Logger
}

// NewDocker needs a documetnation
func NewDocker(ctx context.Context, tty bool, logger *logger.Logger) (*Docker, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	return &Docker{
		client:     client,
		context:    ctx,
		tty:        tty,
		logger:     logger,
		layerSet:   map[string]struct{}{},
		images:     []string{},
		skipLayers: []string{},
		layers:     []string{},
	}, nil
}

// SetContext sets the context for subsequent calls.
func (d *Docker) SetContext(ctx context.Context) {
	d.context = ctx
}

// AddImage adds layers to the layer list from a provided image, in order of
// appearance. Any existing layers are skipped over, removing them from the list.
func (d *Docker) AddImage(image string) error {
	d.images = append(d.images, image)

	resp, _, err := d.client.ImageInspectWithRaw(d.context, image)
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

func (d *Docker) consumeEdits(progressChan chan ctypes.ProgressProperties) {
	var last string
	for prog := range progressChan {
		if d.tty {
			digest := prog.Artifact.Digest.String()

			if digest == last {
				fmt.Print("\r")
			} else if last != "" {
				fmt.Println()
			}

			d.logger.Progress(strings.SplitN(digest, ":", 2)[1][:12], float64(prog.Offset/megaByte))
			last = digest
		}
	}

	if d.tty {
		fmt.Println()
	}
}

func (d *Docker) makeImage(from string) (string, error) {
	ref, err := daemon.ParseReference(from)
	if err != nil {
		return "", err
	}

	img, err := ref.NewImage(nil)
	if err != nil {
		return "", err
	}
	defer img.Close()

	tgtRef, err := reference.ParseNamed(from)
	if err != nil {
		return "", err
	}

	tgt, err := daemon.NewReference("", tgtRef)
	if err != nil {
		return "", err
	}

	pc, err := signature.NewPolicyContext(&signature.Policy{
		Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()},
	})

	if err != nil {
		return "", err
	}

	progressChan := make(chan ctypes.ProgressProperties)

	go d.consumeEdits(progressChan)

	if d.tty {
		d.logger.Print(d.logger.Notice("Editing image\n"))
	} else {
		d.logger.Print(d.logger.Notice("Editing image..."))
	}

	img2, err := copy.Image(pc, tgt, ref, &copy.Options{
		RemoveSignatures: true,
		LayerCopyHook: func(srcLayer ctypes.BlobInfo) bool {
			var found bool
			for _, l := range d.layers {
				if srcLayer.Digest.String() == l {
					found = true
				}
			}

			return found
		},
		Progress:         progressChan,
		ProgressInterval: 100 * time.Millisecond,
	})
	close(progressChan)
	if err != nil {
		return "", err
	}

	if !d.tty {
		fmt.Println("done.")
	}

	return img2.ConfigInfo().Digest.String(), nil
}

// MakeImage makes the final image, skipping any layers as necessary. The
// layers must be pre-recorded within the executor. Note that if you have no
// layers to skip, this operation will need to do nothing, so it will do
// nothing.
//
// It returns an error condition, if any.
func (d *Docker) MakeImage(config *config.Config) (string, error) {
	// this is principally an optimization so we can determine later if we
	// need to reconstruct the image.
	if len(d.skipLayers) == 0 {
		return config.Image, nil
	}

	var err error

	config.Image, err = d.makeImage(config.Image)
	if err != nil {
		return "", err
	}

	return config.Image, nil
}

// Lookup an image by name, returning the id.
func (d *Docker) Lookup(name string) (string, error) {
	img, _, err := d.client.ImageInspectWithRaw(d.context, name)
	if err != nil {
		return "", err
	}

	return img.ID, nil
}

// Fetch retrieves a docker image, overwrites the container configuration, and
// returns its id.
func (d *Docker) Fetch(config *config.Config, name string) (string, error) {
	location, layers, err := fetcher.Docker(d.context, d.logger, d.client, d.tty, config, name)
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
