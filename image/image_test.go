package image

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	. "testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	. "gopkg.in/check.v1"
)

type imageSuite struct {
	client *client.Client
}

var _ = Suite(&imageSuite{})

func TestImage(t *T) {
	TestingT(t)
}

func (is *imageSuite) SetUpSuite(c *C) {
	var err error
	is.client, err = client.NewEnvClient()
	c.Assert(err, IsNil)
}

func (is *imageSuite) download(c *C, imageName string) string {
	o, err := is.client.ImagePull(context.Background(), imageName, types.ImagePullOptions{})
	c.Assert(err, IsNil)

	// docker pull will not finish until this fd (the progress meter) is read to the end.
	_, err = io.Copy(ioutil.Discard, o)
	c.Assert(err, IsNil)

	rc, err := is.client.ImageSave(context.Background(), []string{imageName})
	c.Assert(err, IsNil)

	fi, err := ioutil.TempFile("", "image-download-")
	c.Assert(err, IsNil)

	_, err = io.Copy(fi, rc)
	c.Assert(err, IsNil)

	rc.Close()
	fi.Close()

	return fi.Name()
}

func (is *imageSuite) TestUnpack(c *C) {
	fn := is.download(c, "debian")
	defer os.Remove(fn)
	layers, dir, err := Unpack(fn)
	defer os.RemoveAll(dir)
	c.Assert(err, IsNil)
	fi, err := os.Stat(dir)
	c.Assert(err, IsNil)
	c.Assert(fi.IsDir(), Equals, true)
	c.Assert(len(layers), Not(Equals), 0)

	for _, layer := range layers {
		_, err := os.Stat(layer.filename)
		c.Assert(err, IsNil)
	}
}
