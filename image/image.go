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
	bt "github.com/erikh/box/tar"
)

// Layer is the metadata surrounding an image layer.
type Layer struct {
	layer         string
	filename      string
	layerFilename string
}

// LayerID returns the layer ID associated with this layer.
func (l *Layer) LayerID() string {
	return fmt.Sprintf("sha256:%s", l.layer)
}

func writeLayer(imgwriter *tar.Writer, tarFile string, tf *os.File) error {
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

	if _, err := io.Copy(imgwriter, tf); err != nil {
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

func doSum(tf *os.File, tw io.Reader) (string, error) {
	if _, err := io.Copy(tf, tw); err != nil {
		return "", err
	}

	sum, err := bt.SumFile(tf.Name())
	if err != nil {
		return "", err
	}

	if err := tf.Sync(); err != nil {
		return "", err
	}

	if _, err := tf.Seek(0, 0); err != nil {
		return "", err
	}

	return sum, nil
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
	f, err := os.Open(file)
	if err != nil {
		return nil, "", err
	}

	defer f.Close()

	tr := tar.NewReader(f)

	dir, err := ioutil.TempDir("", "box-image-tmp")
	if err != nil {
		return nil, dir, err
	}

	layerOrder := []string{}
	layerMap := map[string]*Layer{} // layer id -> Layer obj
	layers := []*Layer{}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, dir, err
		}

		if header.Name == "manifest.json" {
			manifest := []map[string]interface{}{}
			content, err := ioutil.ReadAll(tr)
			if err != nil {
				return nil, dir, err
			}

			if err := json.Unmarshal(content, &manifest); err != nil {
				return nil, dir, err
			}

			tmp, ok := manifest[0]["Layers"].([]interface{}) // FIXME how to handle multiple images?
			if !ok {
				return nil, dir, fmt.Errorf("Manifest is broken: %#v", manifest)
			}

			for _, layer := range tmp {
				layerID := strings.TrimSuffix(filepath.Base(filepath.Dir(layer.(string))), "/")
				layerOrder = append(layerOrder, layerID)
				layerMap[layerID] = &Layer{
					layerFilename: layer.(string),
				}
			}

			break
		}
	}

	f.Close()

	f, err = os.Open(f.Name())
	if err != nil {
		return nil, dir, err
	}
	defer f.Close()

	tr = tar.NewReader(f)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, dir, err
		}

		if strings.HasSuffix(header.Name, ".tar") {
			// this renames the file to be at the root with the sha as the filename itself.
			// this just makes traversal a lot easier and makes less of a mess out of the filesystem.
			layerID := filepath.Base(filepath.Dir(header.Name))
			if len(layerID) != 64 {
				return nil, dir, fmt.Errorf("invalid layerID: %v", layerID)
			}

			if strings.ContainsAny(layerID, "/.") {
				return nil, dir, fmt.Errorf("Layer ID contains invalid characters: %v", layerID)
			}

			out, err := os.Create(filepath.Join(dir, layerID))
			if err != nil {
				return nil, dir, err
			}

			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return nil, dir, err
			}

			out.Close()
			sum, err := bt.SumFile(out.Name())
			if err != nil {
				return nil, dir, err
			}

			l, ok := layerMap[layerID]
			if !ok {
				return nil, dir, errors.New("layer not found")
			}

			l.layer = sum
			l.filename = out.Name()
		}
	}

	for _, layerID := range layerOrder {
		if layer, ok := layerMap[layerID]; ok {
			layers = append(layers, layer)
		} else {
			return nil, dir, fmt.Errorf("Layer ID %v not found in mapping: %v", layerID, layerMap)
		}
	}

	return layers, dir, nil
}

// Make copies N layers into a single image, ships it back to docker.
func Make(config *config.Config, layers []*Layer) (string, error) {
	if len(layers) == 0 {
		return "", fmt.Errorf("no image layers to construct with")
	}

	out, err := tmpfile()
	if err != nil {
		return "", err
	}

	defer out.Close()

	imgwriter := tar.NewWriter(out)

	tarFiles, err := writeConfig(layers, imgwriter, config)
	if err != nil {
		return "", err
	}

	if len(tarFiles) != len(layers) {
		return "", fmt.Errorf("tarfiles returned were not equal to files: %v -- %v", tarFiles, layers)
	}

	for _, layer := range layers {
		f, err := os.Open(layer.filename)
		if err != nil {
			return "", err
		}

		if err := writeLayer(imgwriter, layer.layerFilename, f); err != nil {
			f.Close()
			return "", err
		}

		f.Close()
	}

	return out.Name(), nil
}

// Flatten copies a tarred up series of files (passed in through the io.Reader
// handle) to the image where they are untarred. Returns the filename of the
// image created.
func Flatten(config *config.Config, id string, size int64, tw io.Reader) (string, error) {
	out, err := tmpfile()
	if err != nil {
		return "", err
	}

	defer out.Close()

	tf, err := tmpfile()
	if err != nil {
		return "", err
	}

	defer tf.Close() // second close is fine here
	defer os.Remove(tf.Name())

	sum, err := doSum(tf, tw)
	if err != nil {
		return "", err
	}

	imgwriter := tar.NewWriter(out)
	defer imgwriter.Close()

	tarFiles, err := writeConfig([]*Layer{{layer: sum, layerFilename: fmt.Sprintf("%s/layer.tar", sum)}}, imgwriter, config)
	if err != nil {
		return "", err
	}

	if err := writeLayer(imgwriter, tarFiles[len(tarFiles)-1], tf); err != nil {
		return "", err
	}

	return out.Name(), nil
}
