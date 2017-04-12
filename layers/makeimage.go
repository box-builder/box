package layers

import (
	"bufio"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"

	"github.com/box-builder/box/builder/config"
	"github.com/box-builder/box/copy"
	om "github.com/box-builder/overmount"
	"github.com/box-builder/overmount/imgio"
)

const imgIDText = "Loaded image ID: "

func (d *Docker) editLayers(layer *om.Layer) ([]*om.Layer, error) {
	editedLayers := []*om.Layer{}

	for iter := layer; iter != nil; iter = iter.Parent {
		for _, lid := range d.layers {
			if digest.Digest(lid) == iter.Digest() {
				editedLayers = append(editedLayers, iter)
			}
		}
	}

	if len(editedLayers) == 0 {
		return nil, errors.New("layer count would be 0 after edits")
	}

	for i := 0; i < len(editedLayers); i++ {
		if i == len(editedLayers)-1 {
			editedLayers[i].Parent = nil
		} else {
			editedLayers[i].Parent = editedLayers[i+1]
		}
	}

	return editedLayers, nil
}

func (d *Docker) getImage(repo *om.Repository, from string) ([]*om.Layer, error) {
	r, err := d.client.ImageSave(d.globals.Context, []string{from})
	if err != nil {
		return nil, err
	}

	img, err := imgio.NewDocker(d.client)
	if err != nil {
		return nil, err
	}

	return repo.Import(img, r)
}

func (d *Docker) makeImage(from string) (string, error) {
	repo, err := om.NewRepository(path.Join(os.Getenv("HOME"), ".overmount"), true)
	if err != nil {
		return "", err
	}

	toplayers, err := d.getImage(repo, from)
	if err != nil {
		return "", err
	}

	if len(toplayers) > 1 {
		d.globals.Logger.Notice("More than one image detected during save; using first image. Use a more specific tag.")
	}

	if len(toplayers) == 0 {
		return "", errors.New("No images detected during save")
	}

	layer := toplayers[0]
	if err := layer.RestoreParent(); err != nil {
		return "", err
	}

	editedLayers, err := d.editLayers(layer)
	if err != nil {
		return "", err
	}

	img, err := imgio.NewDocker(d.client)
	if err != nil {
		return "", err
	}

	reader, err := repo.Export(img, editedLayers[0], []string{})
	if err != nil {
		return "", err
	}

	return d.loadReader(reader)
}

func (d *Docker) loadReader(reader io.Reader) (string, error) {
	r, w := io.Pipe()
	tee := io.TeeReader(reader, w)
	endChan := make(chan struct{})

	defer func() {
		w.Close()
		<-endChan
	}()

	go func() {
		copy.WithProgress(ioutil.Discard, r, d.globals.Logger, "Writing New Image")
		close(endChan)
	}()

	resp, err := d.client.ImageLoad(d.globals.Context, tee, true)
	if err != nil {
		return "", err
	}

	buf := bufio.NewReader(resp.Body)
	for {
		content, err := buf.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		tmp := map[string]string{}

		if err := json.Unmarshal(content, &tmp); err != nil {
			return "", err
		}

		if str, ok := tmp["stream"]; ok && strings.Contains(str, imgIDText) {
			return strings.TrimLeft(strings.TrimSpace(str), imgIDText), nil
		}
	}

	return "", errors.New("cannot locate image id")
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
