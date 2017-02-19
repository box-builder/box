package layers

import (
	"context"
	"io"
	"io/ioutil"
	"math/rand"
	"os"

	. "testing"

	. "gopkg.in/check.v1"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/term"
	"github.com/erikh/box/builder/config"
	"github.com/erikh/box/image"
	"github.com/erikh/box/logger"
)

var dockerClient *client.Client

type dockerSuite struct {
	tty    bool
	config *config.Config
}

var _ = Suite(&dockerSuite{config: config.NewConfig()})

func TestDocker(t *T) {
	TestingT(t)
}

func (ds *dockerSuite) SetUpSuite(c *C) {
	var err error
	ds.tty = term.IsTerminal(0)
	dockerClient, err = client.NewEnvClient()
	c.Assert(err, IsNil)
	ds.TearDownSuite(c)
}

func (ds *dockerSuite) TearDownSuite(c *C) {
	if os.Getenv("DIND") != "" {
		containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
		c.Assert(err, IsNil)

		for _, container := range containers {
			err := dockerClient.ContainerRemove(context.Background(), container.ID, types.ContainerRemoveOptions{Force: true})
			c.Assert(err, IsNil)
		}

		images, err := dockerClient.ImageList(context.Background(), types.ImageListOptions{})
		c.Assert(err, IsNil)

		for i := 0; i < 2; i++ {
			for _, image := range images {
				dockerClient.ImageRemove(context.Background(), image.ID, types.ImageRemoveOptions{Force: true})
			}
		}
	}
}

func (ds *dockerSuite) TestLookup(c *C) {
	imageName := "alpine"

	d, err := NewDocker(context.Background(), ds.tty, logger.New(""))
	c.Assert(err, IsNil)

	// XXX ok if this call fails
	d.client.ImageRemove(d.context, imageName, types.ImageRemoveOptions{PruneChildren: true, Force: true})

	id, err := d.Lookup(imageName)
	c.Assert(err, NotNil)
	c.Assert(id, Equals, "")

	origid, err := d.Fetch(ds.config, imageName)
	c.Assert(err, IsNil)
	c.Assert(origid, Not(Equals), "")

	newid, err := d.Lookup(imageName)
	c.Assert(err, IsNil)
	c.Assert(newid, Equals, origid)
}

func (ds *dockerSuite) TestMakeImage(c *C) {
	imageName := "postgres"

	d, err := NewDocker(context.Background(), ds.tty, logger.New(""))
	c.Assert(err, IsNil)

	_, err = d.Fetch(ds.config, imageName)
	c.Assert(err, IsNil)

	rc, err := d.client.ImageSave(context.Background(), []string{imageName})
	c.Assert(err, IsNil)

	tf, err := ioutil.TempFile("", "box-test-image")
	c.Assert(err, IsNil)

	defer tf.Close()
	defer os.Remove(tf.Name())

	_, err = io.Copy(tf, rc)
	c.Assert(err, IsNil)

	layers, dir, err := image.Unpack(tf.Name())
	defer os.RemoveAll(dir)
	c.Assert(err, IsNil)

	_, err = d.Fetch(ds.config, imageName)
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
	id, err := d.MakeImage(ds.config)

	c.Assert(err, IsNil)
	c.Assert(id, Not(Equals), "")

	rc, err = d.client.ImageSave(context.Background(), []string{id})
	c.Assert(err, IsNil)

	tf2, err := ioutil.TempFile("", "box-test-image")
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

func (ds *dockerSuite) TestFetch(c *C) {
	d, err := NewDocker(context.Background(), ds.tty, logger.New(""))
	c.Assert(err, IsNil)

	id, err := d.Fetch(ds.config, "debian:latest")
	c.Assert(err, IsNil)

	_, _, err = d.client.ImageInspectWithRaw(context.Background(), id)
	c.Assert(err, IsNil)

	_, err = d.Fetch(ds.config, "quezacoatl")
	c.Assert(err, NotNil)
}
