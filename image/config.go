package image

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"path"
	"time"
)

func (i *Image) writeConfig(tw *tar.Writer) error {
	if len(i.layers) < 1 {
		return fmt.Errorf("sum len (%d) is less than 1, nothing to write", len(i.layers))
	}

	lastLayer := i.layers[len(i.layers)-1]

	jsonFile := fmt.Sprintf("%s.json", lastLayer.id)
	tarFiles := []string{}
	layerIDs := []string{}
	for _, layer := range i.layers {
		layerIDs = append(layerIDs, layer.id)
		tarFiles = append(tarFiles, path.Join(layer.id, "layer.tar"))
	}

	manifest := []map[string]interface{}{{
		"Config": jsonFile,
		"Layers": tarFiles,
	}}

	content, err := json.Marshal(i.config.ToImage(layerIDs))
	if err != nil {
		return err
	}

	err = tw.WriteHeader(&tar.Header{
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
		return err
	}

	if _, err := tw.Write(content); err != nil {
		return err
	}

	content, err = json.Marshal(manifest)
	if err != nil {
		return err
	}
	err = tw.WriteHeader(&tar.Header{
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
		return err
	}

	if _, err := tw.Write(content); err != nil {
		return err
	}

	return nil
}
