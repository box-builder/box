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
	"time"

	"github.com/erikh/box/builder/config"
	"github.com/erikh/box/copy"
	"github.com/erikh/box/logger"
	"github.com/erikh/box/signal"
	bt "github.com/erikh/box/tar"
)

// Layer is the metadata surrounding an image layer.
type Layer struct {
	layer         string
	filename      string
	layerFilename string
}

type imageInfo struct {
	layerOrder []string
	layerMap   map[string]*Layer
}

// LayerID returns the layer ID associated with this layer.
func (l *Layer) LayerID() string {
	return fmt.Sprintf("sha256:%s", l.layer)
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
			layerID := filepath.Base(filepath.Dir(header.Name))
			if len(layerID) != 64 {
				return fmt.Errorf("invalid layerID: %v", layerID)
			}

			if strings.ContainsAny(layerID, "/.") {
				return fmt.Errorf("Layer ID contains invalid characters: %v", layerID)
			}

			l, ok := img.layerMap[layerID]
			if !ok {
				return errors.New("layer not found")
			}

			out, err := os.Create(filepath.Join(dir, layerID))
			if err != nil {
				return err
			}

			sum, err := bt.SumWithCopy(out, tr, logger.New(layerID[:12], false), fmt.Sprintf("Unpacking Layer ID %s", layerID[:12]))
			if err != nil {
				return err
			}

			l.layer = sum
			l.filename = out.Name()
		}
	}

	return nil
}

func extractManifest(file string) (*imageInfo, error) {
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
				layerID := strings.TrimSuffix(filepath.Base(filepath.Dir(layer.(string))), "/")
				img.layerOrder = append(img.layerOrder, layerID)
				img.layerMap[layerID] = &Layer{
					layerFilename: layer.(string),
				}
			}

			break
		}
	}

	return img, nil
}

func writeLayer(imgwriter *tar.Writer, tarFile string, tf *os.File, logger *logger.Logger) error {
	fi, err := tf.Stat()
	if err != nil {
		return err
	}

	err = imgwriter.WriteHeader(&tar.Header{
		Name:     tarFile,
		Size:     fi.Size(),
		Mode:     0666,
		Typeflag: tar.TypeReg,
	})

	if err != nil {
		return err
	}

	if err := copy.WithProgress(imgwriter, tf, logger, "Writing Layer"); err != nil {
		return err
	}

	imgwriter.Flush()

	return nil
}

func writeConfig(layers []*Layer, imgwriter *tar.Writer, config *config.Config) ([]string, error) {
	if len(layers) < 1 {
		return nil, fmt.Errorf("sum len (%d) is less than 1, nothing to write", len(layers))
	}

	lastLayer := layers[len(layers)-1]

	jsonFile := fmt.Sprintf("%s.json", lastLayer.layer)
	tarFiles := []string{}
	layerIDs := []string{}
	for _, layer := range layers {
		layerIDs = append(layerIDs, layer.layer)
		tarFiles = append(tarFiles, layer.layerFilename)
	}

	manifest := []map[string]interface{}{{
		"Config": jsonFile,
		"Layers": tarFiles,
	}}

	content, err := json.Marshal(config.ToImage(layerIDs))
	if err != nil {
		return nil, err
	}

	err = imgwriter.WriteHeader(&tar.Header{
		Uname:      "root",
		Gname:      "root",
		Name:       jsonFile,
		Linkname:   jsonFile,
		Size:       int64(len(content)),
		Mode:       0666,
		Typeflag:   tar.TypeReg,
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
	})

	if err != nil {
		return nil, err
	}

	if _, err := imgwriter.Write(content); err != nil {
		return nil, err
	}

	imgwriter.Flush()

	content, err = json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	err = imgwriter.WriteHeader(&tar.Header{
		Name:       "manifest.json",
		Linkname:   "manifest.json",
		Uname:      "root",
		Gname:      "root",
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
		Size:       int64(len(content)),
		Mode:       0666,
		Typeflag:   tar.TypeReg,
	})

	if err != nil {
		return nil, err
	}

	if _, err := imgwriter.Write(content); err != nil {
		return nil, err
	}

	imgwriter.Flush()
	return tarFiles, nil
}

func tmpfile() (*os.File, error) {
	return ioutil.TempFile("", "box-temp-image")
}

// Unpack unpacks an image into the temporary filesystem. Returns a list of
// paths for each layer. Information about the image itself is not written to
// disk; the tarballs are just dumped.
//
// First return value is the order of the layers. Then, the directory of the
// files kept so it can be removed later. The dir will always be returned if
// possible; even when a later operation returns an error.
func Unpack(file string) ([]*Layer, string, error) {
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

	img, err := extractManifest(file)
	if err != nil {
		return nil, dir, err
	}

	if err := extractLayers(img, dir, file); err != nil {
		return nil, dir, err
	}

	layers := []*Layer{}

	for _, layerID := range img.layerOrder {
		if layer, ok := img.layerMap[layerID]; ok {
			layers = append(layers, layer)
		} else {
			return nil, dir, fmt.Errorf("Layer ID %v not found in mapping: %v", layerID, img.layerMap)
		}
	}

	return layers, dir, nil
}

// Flatten copies a tarred up series of files (passed in through the io.Reader
// handle) to the image where they are untarred. Returns the filename of the
// image created.
func Flatten(config *config.Config, id string, size int64, tw io.Reader, logger *logger.Logger) (string, error) {
	out, err := tmpfile()
	if err != nil {
		return "", err
	}

	signal.Handler.AddFile(out.Name())
	defer signal.Handler.RemoveFile(out.Name())

	defer out.Close()

	tf, err := tmpfile()
	if err != nil {
		return "", err
	}

	signal.Handler.AddFile(tf.Name())
	defer signal.Handler.RemoveFile(tf.Name())

	defer os.Remove(tf.Name())

	sum, err := bt.SumWithCopy(tf, tw, logger, "Processing Image for Flatten")
	if err != nil {
		return "", err
	}

	tf, err = os.Open(tf.Name())
	if err != nil {
		return "", err
	}
	defer tf.Close() // second close is fine here

	imgwriter := tar.NewWriter(out)
	defer imgwriter.Close()

	tarFiles, err := writeConfig([]*Layer{{layer: sum, layerFilename: fmt.Sprintf("%s/layer.tar", sum)}}, imgwriter, config)
	if err != nil {
		return "", err
	}

	if err := writeLayer(imgwriter, tarFiles[len(tarFiles)-1], tf, logger); err != nil {
		return "", err
	}

	return out.Name(), nil
}
