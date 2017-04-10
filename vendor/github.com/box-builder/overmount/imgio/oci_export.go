package imgio

import (
	"archive/tar"
	"encoding/json"
	"io"
	"os"
	"path"

	om "github.com/box-builder/overmount"
	"github.com/box-builder/overmount/configmap"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

const (
	ociSchemaVersion  = 2
	refsDir           = "refs"
	blobsDir          = "blobs/sha256"
	tempfilePrefix    = "overmount-pack-"
	layerMediaType    = "application/vnd.oci.image.layer.v1.tar"
	configMediaType   = "application/vnd.oci.image.config.v1+json"
	manifestMediaType = "application/vnd.oci.image.manifest.v1+json"
)

// Export exports an OCI image to the reader as a tar file.
func (o *OCI) Export(repo *om.Repository, layer *om.Layer, tags []string) (io.ReadCloser, error) {
	if !layer.Exists() {
		return nil, errors.Wrap(om.ErrInvalidLayer, "layer does not exist")
	}

	r, w := io.Pipe()
	go o.write(repo, w, layer, tags)

	return r, nil
}

func (o *OCI) writeImageConfig(layer *om.Layer, tw *tar.Writer, diffIDs []digest.Digest) (digest.Digest, int64, error) {
	config, err := layer.Config()
	if err != nil {
		return "", 0, err
	}

	oci := configmap.ToOCIV1(config)

	oci.RootFS = v1.RootFS{
		Type:    "layers",
		DiffIDs: diffIDs,
	}

	return o.writeJSONBlob(oci, tw)
}

func (o *OCI) writePrefix(tw *tar.Writer) error {
	if err := o.writeDirs(tw); err != nil {
		return err
	}

	return o.writeImageLayout(tw)
}

func (o *OCI) writeLayers(repo *om.Repository, layer *om.Layer, tw *tar.Writer) ([]digest.Digest, []digest.Digest, []int64, []*om.Layer, error) {
	return runChain(layer, tw, func(parent digest.Digest, iter *om.Layer, tw *tar.Writer) (digest.Digest, digest.Digest, int64, error) {
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

		if _, err := tf.Seek(0, 0); err != nil {
			return "", "", 0, err
		}

		fi, err := tf.Stat()
		if err != nil {
			return "", "", 0, err
		}

		err = tw.WriteHeader(&tar.Header{
			Name:     path.Join(blobsDir, diffID.Hex()),
			Mode:     0600,
			Typeflag: tar.TypeReg,
			Size:     fi.Size(),
		})

		if err != nil {
			return "", "", 0, errors.Wrap(om.ErrImageCannotBeComposed, "cannot add file to tar writer")
		}

		if _, err := io.Copy(tw, tf); err != nil {
			return "", "", 0, err
		}

		return chainID, diffID, fi.Size(), nil
	})
}

func (o *OCI) writeJSONBlob(obj interface{}, tw *tar.Writer) (digest.Digest, int64, error) {
	content, err := json.Marshal(obj)
	if err != nil {
		return "", 0, err
	}

	dg := digest.FromBytes(content)

	err = tw.WriteHeader(&tar.Header{
		Name:     path.Join(blobsDir, dg.Hex()),
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     int64(len(content)),
	})
	if err != nil {
		return "", 0, err
	}

	if _, err := tw.Write(content); err != nil {
		return "", 0, err
	}

	return dg, int64(len(content)), nil
}

func (o *OCI) writeJSONTar(filename string, obj interface{}, tw *tar.Writer) (digest.Digest, error) {
	content, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	err = tw.WriteHeader(&tar.Header{
		Name:     filename,
		Mode:     0600,
		Typeflag: tar.TypeReg,
		Size:     int64(len(content)),
	})
	if err != nil {
		return "", err
	}

	dg := digest.FromBytes(content)

	if _, err := tw.Write(content); err != nil {
		return "", err
	}

	return dg, nil
}

func (o *OCI) writeDirs(tw *tar.Writer) error {
	err := tw.WriteHeader(&tar.Header{
		Name:     "blobs",
		Mode:     0700,
		Typeflag: tar.TypeDir,
	})
	if err != nil {
		return err
	}
	err = tw.WriteHeader(&tar.Header{
		Name:     blobsDir,
		Mode:     0700,
		Typeflag: tar.TypeDir,
	})
	if err != nil {
		return err
	}

	return nil
}

func (o *OCI) writeImageLayout(tw *tar.Writer) error {
	_, err := o.writeJSONTar("oci-layout", v1.ImageLayout{Version: "1.0.0"}, tw)
	if err != nil {
		return err
	}

	return nil
}

func (o *OCI) writeManifest(manifest v1.Manifest, tw *tar.Writer) (digest.Digest, int64, error) {
	manifest.SchemaVersion = 2
	return o.writeJSONBlob(manifest, tw)
}

func (o *OCI) write(repo *om.Repository, w *io.PipeWriter, layer *om.Layer, tags []string) (retErr error) {
	defer func() {
		if retErr != nil {
			w.CloseWithError(retErr)
		}
	}()

	tw := tar.NewWriter(w)
	defer w.Close()
	defer tw.Close()

	if err := o.writePrefix(tw); err != nil {
		return err
	}

	_, diffIDs, sizes, _, err := o.writeLayers(repo, layer, tw)
	if err != nil {
		return err
	}

	layerDescriptors := []v1.Descriptor{}
	for i, diff := range diffIDs {
		layerDescriptors = append(layerDescriptors, v1.Descriptor{
			MediaType: layerMediaType,
			Digest:    diff,
			Size:      sizes[i],
		})
	}

	configHash, configSize, err := o.writeImageConfig(layer, tw, diffIDs)
	if err != nil {
		return err
	}

	manifest := v1.Manifest{
		Config: v1.Descriptor{
			MediaType: configMediaType,
			Digest:    configHash,
			Size:      configSize,
		},
		Layers: layerDescriptors,
	}

	manifestHash, manifestSize, err := o.writeManifest(manifest, tw)
	if err != nil {
		return err
	}

	content, err := json.Marshal(v1.Descriptor{
		MediaType: manifestMediaType,
		Digest:    manifestHash,
		Size:      manifestSize,
	})
	if err != nil {
		return err
	}

	for _, ref := range append([]string{layer.ID()}, tags...) {
		err = tw.WriteHeader(&tar.Header{
			Name:     path.Join(refsDir, ref),
			Mode:     0600,
			Typeflag: tar.TypeReg,
			Size:     int64(len(content)),
		})

		if _, err := tw.Write(content); err != nil {
			return err
		}
	}

	return err
}
