package docker

import (
	"archive/tar"
	"context"
	"io/ioutil"
	"os"
	"strings"
	. "testing"

	"github.com/docker/docker/pkg/term"
	"github.com/docker/engine-api/types"
	"github.com/erikh/box/logger"
	bt "github.com/erikh/box/tar"

	. "gopkg.in/check.v1"
)

type dockerSuite struct {
	tty bool
}

var _ = Suite(&dockerSuite{})

func TestDocker(t *T) {
	TestingT(t)
}

func (ds *dockerSuite) SetUpSuite(c *C) {
	ds.tty = term.IsTerminal(0)
}

func (ds *dockerSuite) clearDockerPrefix(c *C, prefix string) {
	d, err := NewDocker(context.Background(), logger.New(""), true, true, ds.tty)
	c.Assert(err, IsNil)
	c.Assert(d.ImageID(), Equals, "")
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
	d, err := NewDocker(context.Background(), logger.New(""), true, false, false)
	c.Assert(err, IsNil)
	c.Assert(d.tty, Equals, false)
	c.Assert(d.useCache, Equals, false)

	d, err = NewDocker(context.Background(), logger.New(""), true, true, true)
	c.Assert(err, IsNil)
	c.Assert(d.tty, Equals, true)
	c.Assert(d.useCache, Equals, true)

	d, err = NewDocker(context.Background(), logger.New(""), true, false, false)
	c.Assert(err, IsNil)
	d.SetStdin(true)
	c.Assert(d.stdin, Equals, true)
	d.UseCache(true)
	c.Assert(d.useCache, Equals, true)
	c.Assert(d.ImageID(), Equals, "")
	c.Assert(d.Config(), NotNil)
}

func (ds *dockerSuite) TestCreate(c *C) {
	d, err := NewDocker(context.Background(), logger.New(""), true, false, ds.tty)
	c.Assert(err, IsNil)

	id, err := d.Create()
	c.Assert(err, IsNil)

	defer d.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{})

	c.Assert(id, Not(Equals), "")
}

func (ds *dockerSuite) TestCommitCache(c *C) {
	ds.clearDockerPrefix(c, "asdf")

	d, err := NewDocker(context.Background(), logger.New(""), true, true, ds.tty)
	c.Assert(err, IsNil)
	c.Assert(d.ImageID(), Equals, "")
	ok, err := d.CheckCache("asdf")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, false)

	c.Assert(d.Commit("asdf", nil), IsNil)
	c.Assert(d.ImageID(), Not(Equals), "")

	ok, err = d.CheckCache("asdf")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)

	d, err = NewDocker(context.Background(), logger.New(""), true, true, ds.tty)
	c.Assert(err, IsNil)

	ok, err = d.CheckCache("asdf")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	c.Assert(d.ImageID(), Not(Equals), "")

	c.Assert(d.Commit("asdf2", nil), IsNil)
	c.Assert(d.ImageID(), Not(Equals), "")

	c.Assert(d.Commit("asdf3", nil), IsNil)
	c.Assert(d.ImageID(), Not(Equals), "")

	d, err = NewDocker(context.Background(), logger.New(""), true, true, ds.tty)
	c.Assert(err, IsNil)

	ok, err = d.CheckCache("asdf")
	c.Assert(ok, Equals, true)
	c.Assert(err, IsNil)
	c.Assert(d.ImageID(), Not(Equals), "")

	ok, err = d.CheckCache("asdf2")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
}

func (ds *dockerSuite) TestFetch(c *C) {
	d, err := NewDocker(context.Background(), logger.New(""), true, true, ds.tty)
	c.Assert(err, IsNil)

	id, err := d.Fetch("debian:latest")
	c.Assert(err, IsNil)

	_, _, err = d.client.ImageInspectWithRaw(context.Background(), id)
	c.Assert(err, IsNil)

	_, err = d.Fetch("quezacoatl")
	c.Assert(err, NotNil)
}

func (ds *dockerSuite) TestCopy(c *C) {
	d, err := NewDocker(context.Background(), logger.New(""), true, true, ds.tty)
	c.Assert(err, IsNil)

	_, err = d.Fetch("debian:latest")
	c.Assert(err, IsNil)

	file, _, err := bt.Archive(context.Background(), ".", ".")
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

	fi, err := f.Stat()
	c.Assert(err, IsNil)

	c.Assert(d.Flatten(id, fi.Size(), f), IsNil)
}

func (ds *dockerSuite) TestTag(c *C) {
	d, err := NewDocker(context.Background(), logger.New(""), true, true, ds.tty)
	c.Assert(err, IsNil)

	// clear old state
	d.client.ImageRemove(context.Background(), "test", types.ImageRemoveOptions{})

	id, err := d.Fetch("docker:latest")
	c.Assert(err, IsNil)

	c.Assert(d.Tag("test"), IsNil)

	inspect, _, err := d.client.ImageInspectWithRaw(context.Background(), "test")
	c.Assert(err, IsNil)

	c.Assert(inspect.ID, Equals, id)
}
