package docker

import (
	"context"
	"io"
	"io/ioutil"
	"math/rand"
	"os"

	. "gopkg.in/check.v1"

	"github.com/erikh/box/image"
)

func (ds *dockerSuite) TestMakeImage(c *C) {
	d, err := NewDocker(true, ds.tty)
	c.Assert(err, IsNil)

	_, err = d.Fetch("debian")
	c.Assert(err, IsNil)

	rc, err := d.client.ImageSave(context.Background(), []string{"debian"})
	c.Assert(err, IsNil)

	tf, err := ioutil.TempFile("", "box-test-debian-image")
	c.Assert(err, IsNil)

	defer tf.Close()
	defer os.Remove(tf.Name())

	_, err = io.Copy(tf, rc)
	c.Assert(err, IsNil)

	layers, dir, err := image.Unpack(tf.Name())
	defer os.RemoveAll(dir)
	c.Assert(err, IsNil)

	_, err = d.Fetch("debian")
	c.Assert(err, IsNil)

	omit := func(layers []*image.Layer) *image.Layer {
		omit1 := rand.Intn(len(layers))
		omitLayer1 := layers[omit1]

		if omit1 == 0 {
			layers = layers[1:]
		} else {
			layers = append(layers[:omit1-1], layers[omit1:]...)
		}
		return omitLayer1
	}

	omitLayer1 := omit(layers)
	omitLayer2 := omit(layers)

	d.skipLayers = []string{omitLayer1.LayerID(), omitLayer2.LayerID()}
	layerStrings := []string{}

	for _, layer := range layers {
		layerStrings = append(layerStrings, layer.LayerID())
	}

	d.layers = layerStrings

	c.Assert(d.MakeImage(), IsNil)

	rc, err = d.client.ImageSave(context.Background(), []string{"debian"})
	c.Assert(err, IsNil)

	tf2, err := ioutil.TempFile("", "box-test-debian-image")
	c.Assert(err, IsNil)

	defer tf2.Close()
	defer os.Remove(tf2.Name())

	_, err = io.Copy(tf2, rc)
	c.Assert(err, IsNil)

	layers2, dir2, err := image.Unpack(tf2.Name())
	defer os.RemoveAll(dir2)
	c.Assert(err, IsNil)

	c.Assert(len(layers), Equals, len(layers2))

	for i := 0; i < len(layers2); i++ {
		c.Assert(layers[i].LayerID(), Equals, layers2[i].LayerID())
	}

	layersOrig, _, err := image.Unpack(tf.Name())
	c.Assert(err, IsNil)

	c.Assert(len(layersOrig), Equals, len(layers))
}
