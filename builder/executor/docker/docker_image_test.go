package docker

import (
	"context"
	"io"
	"io/ioutil"
	"math/rand"
	"os"

	"github.com/erikh/box/image"
	"github.com/erikh/box/logger"

	. "gopkg.in/check.v1"
)

func (ds *dockerSuite) TestMakeImage(c *C) {
	imageName := "ubuntu"

	d, err := NewDocker(context.Background(), logger.New(""), true, true, ds.tty)
	c.Assert(err, IsNil)

	_, err = d.Fetch(imageName)
	c.Assert(err, IsNil)

	rc, err := d.client.ImageSave(context.Background(), []string{imageName})
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

	_, err = d.Fetch(imageName)
	c.Assert(err, IsNil)

	omit := func(layers []*image.Layer) (*image.Layer, []*image.Layer) {
		c.Assert(len(layers), Not(Equals), 0)
		omit1 := rand.Intn(len(layers))
		omitLayer1 := layers[omit1]

		var layerCopy []*image.Layer

		if omit1 == 0 {
			layerCopy = layers[1:]
		} else {
			layerCopy = append(layers[:omit1-1], layers[omit1:]...)
		}

		return omitLayer1, layerCopy
	}

	omitLayer1, layerCopy := omit(layers)
	omitLayer2, layerCopy := omit(layerCopy)

	layers = layerCopy

	d.skipLayers = []string{omitLayer1.LayerID(), omitLayer2.LayerID()}
	layerStrings := []string{}

	for _, layer := range layers {
		layerStrings = append(layerStrings, layer.LayerID())
	}

	d.layers = layerStrings

	c.Assert(d.MakeImage(), IsNil)

	rc, err = d.client.ImageSave(context.Background(), []string{d.Config().Image})
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

	c.Assert(len(layersOrig)-2, Equals, len(layers))
}
