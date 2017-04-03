package image

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/box-builder/box/builder/config"
	"github.com/box-builder/box/copy"
	"github.com/box-builder/box/signal"
	bt "github.com/box-builder/box/tar"
	"github.com/box-builder/box/types"
)

// Layer is one unit of storage. FIXME complete this later
type Layer struct {
	id string

	globals *types.Global
}

// Image is many Layers
type Image struct {
	layers []*Layer
	config *config.Config
	tags   []string

	globals *types.Global
}

// NewImage creates a new *Image for use
func NewImage(globals *types.Global, layers []*Layer, config *config.Config, tags []string) *Image {
	return &Image{
		layers:  layers,
		config:  config,
		tags:    tags,
		globals: globals,
	}
}

// Flatten copies a tarred up series of files (passed in through the io.Reader
// handle) to the image where they are untarred. Returns the filename of the
// image created.
func (i *Image) Flatten(tw io.Reader) (string, error) {
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

	sum, err := bt.SumWithCopy(tf, tw, i.globals.Logger, "Processing Image for Flatten")
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

	i.layers = []*Layer{{id: sum, globals: i.globals}} // deliberately overwrite the layers in the image with the flattened one.

	if err := i.writeConfig(imgwriter); err != nil {
		return "", err
	}

	if err := i.layers[0].Copy(imgwriter, tf); err != nil {
		return "", err
	}

	return out.Name(), nil
}

// NewLayer creates a new *Layer for use
func NewLayer(globals *types.Global, id string) *Layer {
	return &Layer{globals: globals, id: id}
}

// LayerID returns the layer id.
func (l *Layer) LayerID() string {
	return l.id
}

// Copy copies a layer into the tarfile.
func (l *Layer) Copy(tw *tar.Writer, tf *os.File) error {
	fi, err := tf.Stat()
	if err != nil {
		return err
	}

	err = tw.WriteHeader(&tar.Header{
		Name:     path.Join(l.id, "layer.tar"),
		Size:     fi.Size(),
		Mode:     0666,
		Typeflag: tar.TypeReg,
	})

	if err != nil {
		return err
	}

	if err := copy.WithProgress(tw, tf, l.globals.Logger, "Writing Layer"); err != nil {
		return err
	}

	tw.Flush()

	return nil
}

func tmpfile() (*os.File, error) {
	return ioutil.TempFile("", "box-temp-image")
}
