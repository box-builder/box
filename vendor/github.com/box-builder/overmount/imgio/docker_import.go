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
	layers         map[string]*om.Layer
	layerFileMap   map[string]string
	layerParentMap map[string]string
	tagMap         map[string][]string
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

	for layerID, filename := range up.layerFileMap {
		layer, ok := up.layers[layerID]
		if !ok {
			return nil, errors.Errorf("invalid layer id %v", layerID)
		}

		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}

		defer f.Close()
		layer.Parent = up.layers[up.layerParentMap[layerID]]

		var dg digest.Digest

		dg, err = layer.Unpack(f)
		if err == nil {
			if err := layer.SaveParent(); err != nil {
				return nil, err
			}
		} else if !os.IsExist(err) {
			return nil, err
		}

		digestMap[dg] = layer
	}

	for _, img := range up.images {
		topLayer := digest.Digest(img.RootFS.DiffIDs[len(img.RootFS.DiffIDs)-1])
		top, ok := digestMap[topLayer]
		if !ok {
			return nil, errors.New("top layer doesn't appear to exist")
		}

		if err := top.SaveConfig(configmap.FromDockerV1(&img.V1Image)); err != nil {
			return nil, err
		}

		tags, ok := up.tagMap[top.ID()]
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
		layerFileMap:   map[string]string{},
		layerParentMap: map[string]string{},
		layers:         map[string]*om.Layer{},
		images:         []*image.Image{},
		tagMap:         map[string][]string{},
	}

	err := filepath.Walk(tempdir, func(p string, fi os.FileInfo, err error) error {
		if path.Base(p) == "layer.tar" {
			f, err := os.Open(filepath.Join(path.Dir(p), "json"))
			if err != nil {
				return err
			}

			layerJSON := map[string]interface{}{}

			if err := json.NewDecoder(f).Decode(&layerJSON); err != nil {
				f.Close()
				return err
			}
			f.Close()

			layerID, ok := layerJSON["id"].(string)
			if !ok {
				return errors.New("invalid layer id")
			}

			up.layerFileMap[layerID] = p

			if _, ok := layerJSON["parent"]; ok {
				up.layerParentMap[layerID], ok = layerJSON["parent"].(string)
				if !ok {
					return errors.New("invalid parent ID")
				}
			}

			layer, err := r.CreateLayer(layerID, nil)
			if err != nil {
				return err
			}

			up.layers[layerID] = layer
		} else if path.Ext(p) == ".json" && path.Base(p) != "manifest.json" {
			content, err := ioutil.ReadFile(p)
			if err != nil {
				return err
			}

			img := &image.Image{}

			if err := json.Unmarshal(content, img); err != nil {
				return err
			}

			up.images = append(up.images, img)
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
				up.tagMap[lastLayer] = []string{}

				for _, tag := range tags {
					up.tagMap[lastLayer] = append(up.tagMap[lastLayer], tag.(string))
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return up, nil
}
