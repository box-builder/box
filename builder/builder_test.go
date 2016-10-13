package builder

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	. "testing"

	"github.com/docker/engine-api/types/strslice"

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
	c.Assert(string(content), Not(Equals), "")

	c.Assert(bytes.Equal(result, content), Equals, true)

	b, err = runBuilder(`
    from "debian"
    copy "builder.go", "/"
  `)

	c.Assert(err, IsNil)
	result = readContainerFile(c, b, "/builder.go")
	content, err = ioutil.ReadFile("builder.go")
	c.Assert(err, IsNil)
	c.Assert(string(content), Not(Equals), "")

	c.Assert(content, DeepEquals, result)

	b, err = runBuilder(`
    from "debian"
    copy ".", "test"
  `)

	c.Assert(err, IsNil)

	result = readContainerFile(c, b, "/test/builder.go")
	c.Assert(content, DeepEquals, result)

	b, err = runBuilder(`
    from "debian"
    workdir "/test"
    copy ".", "test/"
  `)

	c.Assert(err, IsNil)

	result = readContainerFile(c, b, "/test/test/builder.go")
	c.Assert(content, DeepEquals, result)

	b, err = runBuilder(`
    from "debian"
    inside "/test" do
      copy ".", "test/"
    end
  `)

	c.Assert(err, IsNil)

	result = readContainerFile(c, b, "/test/test/builder.go")
	c.Assert(content, DeepEquals, result)
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
    tag "notflattened"
    flatten
    tag "flattened"
  `)

	c.Assert(err, IsNil)
	c.Assert(b.ImageID(), Not(Equals), "flattened")

	inspect, _, err := b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)

	c.Assert(len(inspect.RootFS.Layers), Equals, 1)

	inspect, _, err = b.client.ImageInspectWithRaw(context.Background(), "notflattened")
	c.Assert(err, IsNil)
	c.Assert(len(inspect.RootFS.Layers), Not(Equals), 1)
}

func (bs *builderSuite) TestEntrypointCmd(c *C) {
	// the echo hi is to trigger a specific interaction problem with entrypoint
	// and run where the entrypoint/cmd would not be overridden during commit
	// time for run.
	b, err := runBuilder(`
    from "debian"
    entrypoint "/bin/cat"
    run "echo hi"
  `)

	c.Assert(err, IsNil)
	inspect, _, err := b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, DeepEquals, strslice.StrSlice{"/bin/cat"})
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{})

	// if cmd is set earlier than entrypoint, it is erased.
	b, err = runBuilder(`
    from "debian"
    cmd "hi"
    entrypoint "/bin/echo"
  `)

	c.Assert(err, IsNil)
	inspect, _, err = b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, DeepEquals, strslice.StrSlice{"/bin/echo"})
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{})

	// normal cmd usage.
	b, err = runBuilder(`
    from "debian"
    entrypoint "/bin/echo"
    cmd "hi"
  `)

	c.Assert(err, IsNil)
	inspect, _, err = b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, DeepEquals, strslice.StrSlice{"/bin/echo"})
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"hi"})

	// normal cmd usage.
	b, err = runBuilder(`
    from "debian"
    cmd "hi"
  `)

	c.Assert(err, IsNil)
	inspect, _, err = b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, DeepEquals, strslice.StrSlice{"/bin/sh", "-c"})
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"hi"})
}

func (bs *builderSuite) TestRun(c *C) {
	b, err := runBuilder(`
    from "debian"
    run "echo -n foo >/bar"
  `)

	c.Assert(err, IsNil)
	result := readContainerFile(c, b, "/bar")
	c.Assert(string(result), Equals, "foo")

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test && chown nobody:nogroup /test"
    with_user "nobody" do
      run "echo -n foo >/test/bar"
    end
  `)

	result = runContainerCommand(c, b, []string{"/usr/bin/stat -c %U /test/bar"})
	c.Assert(string(result), Equals, "nobody\n")

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test && chown nobody:nogroup /test"
    user "nobody"
    run "echo -n foo >/test/bar"
  `)

	result = runContainerCommand(c, b, []string{"/usr/bin/stat -c %U /test/bar"})
	c.Assert(string(result), Equals, "nobody\n")

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test"
    inside "/test" do
      run "echo -n foo >bar"
    end
  `)

	c.Assert(err, IsNil)
	result = readContainerFile(c, b, "/test/bar")
	c.Assert(string(result), Equals, "foo")

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test"
    workdir "/test"
    run "echo -n foo >bar"
  `)

	c.Assert(err, IsNil)
	result = readContainerFile(c, b, "/test/bar")
	c.Assert(string(result), Equals, "foo")
}

func (bs *builderSuite) TestWorkDirInside(c *C) {
	b, err := runBuilder(`
    from "debian"
    run "mkdir /test"
    workdir "/test"
    run "echo -n foo >bar"
  `)

	c.Assert(err, IsNil)
	result := readContainerFile(c, b, "/test/bar")
	c.Assert(string(result), Equals, "foo")

	inspect, _, err := b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.WorkingDir, Equals, "/test")

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test"
    inside "/test" do
      run "echo -n foo >bar"
    end
  `)

	c.Assert(err, IsNil)
	result = readContainerFile(c, b, "/test/bar")
	c.Assert(string(result), Equals, "foo")

	inspect, _, err = b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.WorkingDir, Equals, "/")

	// this file is used in the copy comparisons
	content, err := ioutil.ReadFile("builder.go")
	c.Assert(err, IsNil)

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test"
    workdir "/test"
    copy ".", "."
  `)

	c.Assert(err, IsNil)
	result = readContainerFile(c, b, "/test/builder.go")
	c.Assert(result, DeepEquals, content)

	inspect, _, err = b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.WorkingDir, Equals, "/test")

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test"
    inside "/test" do
      copy ".", "."
    end
  `)

	c.Assert(err, IsNil)
	result = readContainerFile(c, b, "/test/builder.go")

	c.Assert(result, DeepEquals, content)

	inspect, _, err = b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.WorkingDir, Equals, "/")
}

func (bs *builderSuite) TestUser(c *C) {
	b, err := runBuilder(`
    from "debian"
    run "mkdir /test && chown nobody:nogroup /test"
    user "nobody"
    run "echo -n foo >/test/bar"
  `)

	c.Assert(err, IsNil)
	result := readContainerFile(c, b, "/test/bar")
	c.Assert(string(result), Equals, "foo")

	inspect, _, err := b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.User, Equals, "nobody")

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test && chown nobody:nogroup /test"
    with_user "nobody" do
      run "echo -n foo >/test/bar"
    end
  `)

	c.Assert(err, IsNil)
	result = readContainerFile(c, b, "/test/bar")
	c.Assert(string(result), Equals, "foo")

	inspect, _, err = b.client.ImageInspectWithRaw(context.Background(), b.ImageID())
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.User, Equals, "root")
}
