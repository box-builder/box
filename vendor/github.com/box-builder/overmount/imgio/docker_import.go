package imgio

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/docker/docker/image"
	"github.com/docker/docker/pkg/archive"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"

	om "github.com/box-builder/overmount"
	"github.com/box-builder/overmount/configmap"
)

type unpackedImage struct {
	tempdir        string
	images         []*image.Image
	chainParentMap map[string]string
	layers         map[string]*om.Layer
	tagMap         map[digest.Digest][]string
}

// Import takes a tar represented as an io.Reader, and converts and unpacks
// it into the overmount repository.  Returns the top-most layer and any
// error.
func (d *Docker) Import(r *om.Repository, reader io.ReadCloser) ([]*om.Layer, error) {
	tempdir, err := r.TempDir()
	if err != nil {
		return nil, err
	}

	defer os.RemoveAll(tempdir)

	if err := archive.Untar(reader, tempdir, &archive.TarOptions{NoLchown: os.Geteuid() != 0}); err != nil {
		return nil, err
	}

	reader.Close()

	up, err := d.unpackLayers(r, tempdir)
	if err != nil {
		return nil, err
	}

	return d.constructImage(r, up)
}

func (d *Docker) constructImage(r *om.Repository, up *unpackedImage) ([]*om.Layer, error) {
	layers := []*om.Layer{}
	digestMap := map[digest.Digest]*om.Layer{}

	for layerID, layer := range up.layers {
		if parent, ok := up.layers[up.chainParentMap[layerID]]; ok {
			layer.Parent = parent

			if err := layer.SaveParent(); err != nil {
				return nil, err
			}
		}

		digestMap[layer.Digest()] = layer
	}

	for _, img := range up.images {
		topLayer := digest.Digest(img.RootFS.DiffIDs[len(img.RootFS.DiffIDs)-1])
		top, ok := digestMap[topLayer]
		if !ok {
			return nil, errors.New("top layer doesn't appear to exist")
		}

		// force a write on the top layer.
		if err := top.SaveConfig(configmap.FromDockerV1(&img.V1Image)); err != nil {
			return nil, err
		}

		// cascade the config through the image until we find another config.
		for iter := top.Parent; iter != nil; iter = iter.Parent {
			if _, err := iter.Config(); err == nil {
				break
			}

			if err := iter.SaveConfig(configmap.FromDockerV1(&img.V1Image)); err != nil {
				return nil, err
			}
		}

		tags, ok := up.tagMap[topLayer]
		if ok {
			for _, tag := range tags {
				if err := r.AddTag(tag, top); err != nil {
					return nil, err
				}
			}
		}

		layers = append(layers, top)
	}

	return layers, nil
}

func (d *Docker) unpackLayers(r *om.Repository, tempdir string) (*unpackedImage, error) {
	up := &unpackedImage{
		tempdir:        tempdir,
		chainParentMap: map[string]string{},
		layers:         map[string]*om.Layer{},
		images:         []*image.Image{},
		tagMap:         map[digest.Digest][]string{},
	}

	err := filepath.Walk(tempdir, func(p string, fi os.FileInfo, err error) error {
		if path.Base(p) == "layer.tar" {
			f, err := os.Open(filepath.Join(path.Dir(p), "json"))
			if err != nil {
				return err
			}

			layerJSON := map[string]interface{}{}

			err = json.NewDecoder(f).Decode(&layerJSON)
			f.Close()
			if err != nil {
				return err
			}

			layerID, ok := layerJSON["id"].(string)
			if !ok {
				return errors.New("invalid layer id")
			}

			if _, ok := layerJSON["parent"]; ok {
				up.chainParentMap[layerID], ok = layerJSON["parent"].(string)
				if !ok {
					return errors.New("invalid parent ID")
				}
			}

			f, err = os.Open(p)
			if err != nil {
				return err
			}

			layer, err := r.CreateLayerFromAsset(f, nil, true)
			f.Close()
			if err != nil {
				return err
			}

			up.layers[layerID] = layer
		} else if path.Base(p) == "manifest.json" {
			content, err := ioutil.ReadFile(p)
			if err != nil {
				return err
			}

			manifest := []map[string]interface{}{}
			if err := json.Unmarshal(content, &manifest); err != nil {
				return err
			}

			for _, item := range manifest {
				if item, ok := item["Layers"]; !ok || item == nil {
					continue
				}

				tags, ok := item["RepoTags"].([]interface{})
				if !ok {
					continue
				}

				layers := item["Layers"].([]interface{})

				lastLayer := layers[len(layers)-1].(string)
				lastLayer = path.Dir(lastLayer)
				dg := up.layers[lastLayer].Digest()
				up.tagMap[dg] = []string{}

				for _, tag := range tags {
					up.tagMap[dg] = append(up.tagMap[dg], tag.(string))
				}
			}
		} else if path.Ext(p) == ".json" {
			content, err := ioutil.ReadFile(p)
			if err != nil {
				return err
			}

			img := &image.Image{}

			if err := json.Unmarshal(content, img); err != nil {
				return err
			}

			up.images = append(up.images, img)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return up, nil
}
