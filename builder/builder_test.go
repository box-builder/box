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

func runBuilder(script string) (*Builder, error) {
	b, err := NewBuilder()
	if err != nil {
		return nil, err
	}

	_, err = b.Run(script)
	return b, err
}

func (bs *builderSuite) TestSamePull(c *C) {
	_, err := runBuilder(`from "debian"`)
	c.Assert(err, IsNil)
}

func (bs *builderSuite) TestCopy(c *C) {
	testpath := filepath.Join(dockerfilePath, "test1.rb")

	b, err := runBuilder(fmt.Sprintf(`
from "debian"
copy "%s", "/test1.rb"
  `, testpath))

	c.Assert(err, IsNil)

	result := readContainerFile(c, b, "/test1.rb")

	content, err := ioutil.ReadFile(testpath)
	c.Assert(err, IsNil)
	fmt.Println(string(content))

	c.Assert(bytes.Equal(result, content), Equals, true)
}

func (bs *builderSuite) TestTag(c *C) {
	b, err := runBuilder(`
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
	b, err := runBuilder(`
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

func readContainerFile(c *C, b *Builder, fn string) []byte {
	b.config.Cmd = []string{"cat " + fn}
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

	return result
}
