package imgio

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"

	om "github.com/box-builder/overmount"
	"github.com/box-builder/overmount/configmap"
	"github.com/docker/docker/image"
	dl "github.com/docker/docker/layer"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// Export produces a tar represented as an io.ReadCloser from the Layer provided.
func (d *Docker) Export(repo *om.Repository, layer *om.Layer, tags []string) (io.ReadCloser, error) {
	if !layer.Exists() {
		return nil, errors.Wrap(om.ErrInvalidLayer, "layer does not exist")
	}

	r, w := io.Pipe()
	go d.writeTar(repo, layer, w, tags)

	return r, nil
}

func (d *Docker) writeTar(repo *om.Repository, layer *om.Layer, w *io.PipeWriter, tags []string) (retErr error) {
	defer func() {
		if retErr == nil {
			w.Close()
		} else {
			w.CloseWithError(retErr)
		}
	}()

	tw := tar.NewWriter(w)

	chainIDs, diffIDs, _, _, err := runChain(layer, tw, func(parent digest.Digest, iter *om.Layer, tw *tar.Writer) (digest.Digest, digest.Digest, int64, error) {
		tf, err := repo.TempFile()
		if err != nil {
			return "", "", 0, err
		}

		defer func() {
			tf.Close()
			os.Remove(tf.Name())
		}()

		chainID, diffID, err := calcLayer(parent, iter, tf)
		if err != nil {
			return "", "", 0, err
		}

		if err := d.packLayer(chainID, tf, tw); err != nil {
			return "", "", 0, err
		}

		if err := d.writeLayerConfig(chainID, parent, iter, tw); err != nil {
			return "", "", 0, err
		}

		return chainID, diffID, 0, nil
	})
	if err != nil {
		return err
	}

	if err := d.writeRepositories(tw); err != nil {
		return err
	}

	if err := d.writeManifest(layer, chainIDs, tw, tags); err != nil {
		return err
	}

	if err := d.writeImageConfig(chainIDs[len(chainIDs)-1], diffIDs, layer, tw); err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}

	w.Close()

	return nil
}

func (d *Docker) packLayer(chainID digest.Digest, tf *os.File, tw *tar.Writer) error {
	err := tw.WriteHeader(&tar.Header{
		Name:     chainID.Hex(),
		Mode:     0700,
		Typeflag: tar.TypeDir,
	})
	if err != nil {
		return errors.Wrap(om.ErrImageCannotBeComposed, "cannot add directory to tar writer")
	}

	if _, err := tf.Seek(0, 0); err != nil {
		return err
	}

	fi, err := tf.Stat()
	if err != nil {
		return err
	}

	err = tw.WriteHeader(&tar.Header{
		Name:     path.Join(chainID.Hex(), "layer.tar"),
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     fi.Size(),
	})

	if err != nil {
		return errors.Wrap(om.ErrImageCannotBeComposed, "cannot add file to tar writer")
	}

	if _, err := io.Copy(tw, tf); err != nil {
		return err
	}

	return nil
}

func (d *Docker) writeLayerConfig(chainID digest.Digest, parentID digest.Digest, iter *om.Layer, tw *tar.Writer) error {
	var parent string
	if parentID != "" {
		parent = parentID.Hex()
	}

	content, err := json.Marshal(map[string]interface{}{
		"id":     chainID.Hex(),
		"parent": parent,
		"config": v1.ImageConfig{},
	})
	if err != nil {
		return err
	}

	err = tw.WriteHeader(&tar.Header{
		Name:     path.Join(chainID.Hex(), "json"),
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     int64(len(content)),
	})
	if err != nil {
		return err
	}

	if _, err := tw.Write(content); err != nil {
		return err
	}

	return nil
}

func (d *Docker) writeRepositories(tw *tar.Writer) error {
	content, err := json.Marshal(map[string]interface{}{})
	if err != nil {
		return err
	}

	err = tw.WriteHeader(&tar.Header{
		Name:     "repositories",
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     int64(len(content)),
	})
	if err != nil {
		return err
	}

	if _, err := tw.Write(content); err != nil {
		return err
	}

	return nil
}

func (d *Docker) writeManifest(layer *om.Layer, chainIDs []digest.Digest, tw *tar.Writer, tags []string) error {
	chainIDHexs := []string{}
	for _, chainID := range chainIDs {
		chainIDHexs = append(chainIDHexs, path.Join(chainID.Hex(), "layer.tar"))
	}

	content, err := json.Marshal([]map[string]interface{}{
		{
			"Config":   fmt.Sprintf("%s.json", chainIDs[len(chainIDs)-1].Hex()),
			"RepoTags": tags,
			"Layers":   chainIDHexs,
		},
	})
	if err != nil {
		return err
	}

	err = tw.WriteHeader(&tar.Header{
		Name:     "manifest.json",
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     int64(len(content)),
	})
	if err != nil {
		return err
	}

	if _, err := tw.Write(content); err != nil {
		return err
	}

	return nil
}

func (d *Docker) writeImageConfig(chainID digest.Digest, diffIDs []digest.Digest, layer *om.Layer, tw *tar.Writer) error {
	config, err := layer.Config()
	if err != nil {
		return errors.Wrap(om.ErrInvalidLayer, err.Error())
	}

	if config == nil {
		return errors.Wrap(om.ErrImageCannotBeComposed, "missing image configuration")
	}

	img, err := configmap.ToDockerV1(config)
	if err != nil {
		return errors.Wrap(om.ErrInvalidLayer, err.Error())
	}

	dids := []dl.DiffID{}
	for _, diff := range diffIDs {
		dids = append(dids, dl.DiffID(diff))
	}

	outerConfig := image.Image{
		V1Image: *img,
		RootFS: &image.RootFS{
			Type:    "layers",
			DiffIDs: dids,
		},
		Parent: image.ID(config.Parent),
	}

	content, err := json.Marshal(outerConfig)
	if err != nil {
		return err
	}

	err = tw.WriteHeader(&tar.Header{
		Name:     fmt.Sprintf("%s.json", chainID.Hex()),
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     int64(len(content)),
	})
	if err != nil {
		return err
	}

	if _, err := tw.Write(content); err != nil {
		return err
	}

	return nil
}
