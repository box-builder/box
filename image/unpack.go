package image

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/box-builder/box/logger"
	bt "github.com/box-builder/box/tar"
	"github.com/box-builder/box/types"
)

/*
  FIXME This code really needs to be replaced!
*/

type imageInfo struct {
	globals    *types.Global
	layerOrder []string
	layerMap   map[string]*Layer
}

// Unpack unpacks an image into the temporary filesystem. Returns a list of
// paths for each layer. Information about the image itself is not written to
// disk; the tarballs are just dumped.
//
// First return value is the order of the layers. Then, the directory of the
// files kept so it can be removed later. The dir will always be returned if
// possible; even when a later operation returns an error.
func Unpack(globals *types.Global, file string) ([]*Layer, string, error) {
	var err error
	file, err = filepath.EvalSymlinks(file)
	if err != nil {
		return nil, "", err
	}

	dir, err := ioutil.TempDir("", "box-image-tmp")
	if err != nil {
		return nil, dir, err
	}

	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, dir, err
	}

	img, err := extractManifest(globals, file)
	if err != nil {
		return nil, dir, err
	}

	if err := extractLayers(img, dir, file); err != nil {
		return nil, dir, err
	}

	layers := []*Layer{}

	for _, chainID := range img.layerOrder {
		layers = append(layers, img.layerMap[chainID])
	}

	return layers, dir, nil
}

func extractLayers(img *imageInfo, dir, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	tr := tar.NewReader(f)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if strings.HasSuffix(header.Name, ".tar") {
			// this renames the file to be at the root with the sha as the filename itself.
			// this just makes traversal a lot easier and makes less of a mess out of the filesystem.
			chainID := filepath.Base(filepath.Dir(header.Name))
			if len(chainID) != 64 {
				return fmt.Errorf("invalid chainID: %v", chainID)
			}

			if strings.ContainsAny(chainID, "/.") {
				return fmt.Errorf("Chain ID contains invalid characters: %v", chainID)
			}

			chainID = "sha256:" + chainID
			l, ok := img.layerMap[chainID]
			if !ok {
				return errors.New("layer not found")
			}

			sum, err := bt.SumWithCopy(ioutil.Discard, tr, logger.New(chainID[:12], false), fmt.Sprintf("Unpacking Layer ID %s", chainID[:12]))
			if err != nil {
				return err
			}

			l.id = "sha256:" + sum
		}
	}

	return nil
}

func extractManifest(globals *types.Global, file string) (*imageInfo, error) {
	img := &imageInfo{
		layerOrder: []string{},
		layerMap:   map[string]*Layer{},
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	tr := tar.NewReader(f)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		if header.Name == "manifest.json" {
			manifest := []map[string]interface{}{}
			content, err := ioutil.ReadAll(tr)
			if err != nil {
				return nil, err
			}

			if err := json.Unmarshal(content, &manifest); err != nil {
				return nil, err
			}

			layers := []interface{}{}

			for _, mf := range manifest {
				tmp, ok := mf["Layers"].([]interface{})
				if !ok {
					return nil, fmt.Errorf("Manifest is broken: %#v", manifest)
				}

				layers = append(layers, tmp...)
			}

			for _, layer := range layers {
				chainID := "sha256:" + strings.TrimSuffix(filepath.Base(filepath.Dir(layer.(string))), "/")
				img.layerOrder = append(img.layerOrder, chainID)
				img.layerMap[chainID] = &Layer{
					chainID: chainID,
					globals: globals,
				}
			}

			break
		}
	}

	return img, nil
}
