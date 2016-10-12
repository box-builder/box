package builder

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	. "testing"

	"github.com/docker/engine-api/types"

	. "gopkg.in/check.v1"
)

type builderSuite struct{}

const basePath = "testdata"
const dockerfilePath = "testdata/dockerfiles"
const copyPath = "testdata/copy"

var _ = Suite(&builderSuite{})

func TestBuilder(t *T) {
	TestingT(t)
}

func (bs *builderSuite) TestSamePull(c *C) {
	b, err := NewBuilder()
	c.Assert(err, IsNil)

	_, err = b.Run(`from "debian"`)
	c.Assert(err, IsNil)
}

func (bs *builderSuite) TestCopy(c *C) {
	b, err := NewBuilder()
	c.Assert(err, IsNil)
	testpath := filepath.Join(dockerfilePath, "test1.rb")

	_, err = b.Run(fmt.Sprintf(`
from "debian"
copy "%s", "/test1.rb"
  `, testpath))

	b.config.Cmd = []string{"cat /test1.rb"}
	id, err := b.createEmptyContainer()
	c.Assert(err, IsNil)
	resp, err := b.client.ContainerAttach(context.Background(), id, types.ContainerAttachOptions{Stream: true, Stdout: true, Stdin: true})
	c.Assert(err, IsNil)

	err = b.client.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
	c.Assert(err, IsNil)

	buf := new(bytes.Buffer)

	n, err := io.Copy(buf, resp.Reader)
	c.Assert(err, IsNil)
	c.Assert(n, Not(Equals), 0)

	nr := bufio.NewReader(buf)
	result := []byte{}

	for err == nil {
		var inner []byte
		inner, err = nr.ReadBytes('\n')
		if len(inner) >= 2 && inner[len(inner)-2] == '\r' {
			inner = append(inner[:len(inner)-2], '\n')
		}

		result = append(result, inner...)
	}

	status, err := b.client.ContainerWait(context.Background(), id)
	c.Assert(err, IsNil)
	c.Assert(status, Equals, 0)

	content, err := ioutil.ReadFile(testpath)
	c.Assert(err, IsNil)
	fmt.Println(string(content))

	c.Assert(bytes.Equal(result, content), Equals, true)
}

func (bs *builderSuite) TestTag(c *C) {
	b, err := NewBuilder()
	c.Assert(err, IsNil)
	_, err = b.Run(`
from "debian"
tag "test"
`)

	c.Assert(err, IsNil)
	c.Assert(b.ImageID(), Not(Equals), "test")

	inspect, _, err := b.client.ImageInspectWithRaw(context.Background(), "test")
	c.Assert(err, IsNil)

	c.Assert(inspect.RepoTags, DeepEquals, []string{"test:latest"})
}

func (bs *builderSuite) TestFlatten(c *C) {
	b, err := NewBuilder()
	c.Assert(err, IsNil)
	_, err = b.Run(`
from "debian"
run "echo foo >bar"
run "echo here is another layer >a_file"
flatten
tag "flattened"
`)

	c.Assert(err, IsNil)
	c.Assert(b.ImageID(), Not(Equals), "flattened")

	inspect, _, err := b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)

	c.Assert(len(inspect.RootFS.Layers), Equals, 1)
}
