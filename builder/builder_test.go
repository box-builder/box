package builder

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	. "testing"

	. "gopkg.in/check.v1"
)

type builderSuite struct{}

const basePath = "testdata"
const dockerfilePath = "testdata/dockerfiles"
const copyPath = "testdata/copy"

var _ = Suite(&builderSuite{})

func TestBuilder(t *T) {
	os.Setenv("NO_CACHE", "1")
	TestingT(t)
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
