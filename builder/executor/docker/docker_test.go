package docker

import (
	"archive/tar"
	"context"
	"io/ioutil"
	"os"
	"strings"
	. "testing"

	"github.com/box-builder/box/logger"
	bt "github.com/box-builder/box/tar"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/term"

	. "gopkg.in/check.v1"
)

var dockerClient *client.Client

type dockerSuite struct {
	tty bool
}

var _ = Suite(&dockerSuite{})

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

func (ds *dockerSuite) TestCreate(c *C) {
	d, err := NewDocker(context.Background(), logger.New("", false), true, false, ds.tty)
	c.Assert(err, IsNil)

	id, err := d.Create()
	c.Assert(err, IsNil)

	defer d.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{})

	c.Assert(id, Not(Equals), "")
}

func (ds *dockerSuite) TestCopy(c *C) {
	d, err := NewDocker(context.Background(), logger.New("", false), true, true, ds.tty)
	c.Assert(err, IsNil)

	_, err = d.Layers().Fetch(d.config, "debian:latest")
	c.Assert(err, IsNil)

	file, _, err := bt.Archive(context.Background(), ".", ".", []string{}, d.logger)
	c.Assert(err, IsNil)

	f, err := os.Open(file)
	c.Assert(err, IsNil)

	defer f.Close()
	defer os.Remove(file)

	id, err := d.Create()
	c.Assert(err, IsNil)
	defer d.Destroy(id)

	c.Assert(d.CopyToContainer(id, f), IsNil)
	r, _, err := d.CopyFromContainer(id, "/etc/passwd")
	c.Assert(err, IsNil)

	tr := tar.NewReader(r)
	tr.Next()
	passwd, err := ioutil.ReadAll(tr)
	c.Assert(err, IsNil)

	c.Assert(strings.Contains(string(passwd), "root"), Equals, true, Commentf("%v", string(passwd)))

	passwd, err = d.CopyOneFileFromContainer("/etc/passwd")
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(passwd), "root"), Equals, true, Commentf("%v", string(passwd)))

	f.Close()

	f, err = os.Open(f.Name())
	c.Assert(err, IsNil)
	_, err = f.Seek(0, 0)
	c.Assert(err, IsNil)

	_, err = f.Stat()
	c.Assert(err, IsNil)
}

func (ds *dockerSuite) TestCommitCache(c *C) {
	ds.clearDockerPrefix(c, "asdf")

	d, err := NewDocker(context.Background(), logger.New("", false), true, true, ds.tty)
	c.Assert(err, IsNil)
	c.Assert(d.Image().ImageID(), Equals, "")
	ok, err := d.Image().CheckCache("asdf")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, false)

	c.Assert(d.Commit("asdf", nil), IsNil)
	c.Assert(d.Image().ImageID(), Not(Equals), "")

	ok, err = d.Image().CheckCache("asdf")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)

	d, err = NewDocker(context.Background(), logger.New("", false), true, true, ds.tty)
	c.Assert(err, IsNil)

	ok, err = d.Image().CheckCache("asdf")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	c.Assert(d.Image().ImageID(), Not(Equals), "")

	c.Assert(d.Commit("asdf2", nil), IsNil)
	c.Assert(d.Image().ImageID(), Not(Equals), "")

	c.Assert(d.Commit("asdf3", nil), IsNil)
	c.Assert(d.Image().ImageID(), Not(Equals), "")

	d, err = NewDocker(context.Background(), logger.New("", false), true, true, ds.tty)
	c.Assert(err, IsNil)

	ok, err = d.Image().CheckCache("asdf")
	c.Assert(ok, Equals, true)
	c.Assert(err, IsNil)
	c.Assert(d.Image().ImageID(), Not(Equals), "")

	ok, err = d.Image().CheckCache("asdf2")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
}

func (ds *dockerSuite) clearDockerPrefix(c *C, prefix string) {
	d, err := NewDocker(context.Background(), logger.New("", false), true, true, ds.tty)
	c.Assert(err, IsNil)
	c.Assert(d.Image().ImageID(), Equals, "")
	// clear out any stale images

	images, err := d.client.ImageList(context.Background(), types.ImageListOptions{All: true})
	c.Assert(err, IsNil)

	for i := 0; i < 2; i++ {
		for _, img := range images {
			inspect, _, err := d.client.ImageInspectWithRaw(context.Background(), img.ID)
			if err != nil {
				continue
			}

			if strings.HasPrefix(inspect.Comment, prefix) {
				_, err := d.client.ImageRemove(context.Background(), img.ID, types.ImageRemoveOptions{})
				if err != nil {
					continue
				}

				for err == nil {
					_, _, err = d.client.ImageInspectWithRaw(context.Background(), img.ID)
				}
			}
		}
	}
}

func (ds *dockerSuite) TestParameters(c *C) {
	d, err := NewDocker(context.Background(), logger.New("", false), true, false, false)
	c.Assert(err, IsNil)
	c.Assert(d.tty, Equals, false)
	c.Assert(d.Image().GetCache(), Equals, false)

	d, err = NewDocker(context.Background(), logger.New("", false), true, true, true)
	c.Assert(err, IsNil)
	c.Assert(d.tty, Equals, true)
	c.Assert(d.Image().GetCache(), Equals, true)

	d, err = NewDocker(context.Background(), logger.New("", false), true, false, false)
	c.Assert(err, IsNil)
	d.SetStdin(true)
	c.Assert(d.stdin, Equals, true)
	d.Image().UseCache(true)
	c.Assert(d.Image().GetCache(), Equals, true)
	c.Assert(d.Image().ImageID(), Equals, "")
	c.Assert(d.Config(), NotNil)
}
