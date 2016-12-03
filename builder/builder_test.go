package builder

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	. "testing"

	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"

	. "gopkg.in/check.v1"
)

type builderSuite struct{}

const basePath = "testdata"
const dockerfilePath = "testdata/dockerfiles"
const copyPath = "testdata/copy"

var dockerClient *client.Client

var _ = Suite(&builderSuite{})

func TestBuilder(t *T) {
	TestingT(t)
}

func (bs *builderSuite) SetUpSuite(c *C) {
	var err error

	dockerClient, err = client.NewEnvClient()
	c.Assert(err, IsNil)

	b, err := runBuilder(`from "debian"`)
	c.Assert(err, IsNil)
	b.Close()
}

func (bs *builderSuite) SetUpTest(c *C) {
	os.Setenv("NO_CACHE", "1")
}

func (bs *builderSuite) TestImport(c *C) {
	f, err := ioutil.TempFile("", "import-tmp")
	c.Assert(err, IsNil)

	defer f.Close()

	_, err = f.Write([]byte(`
    from "debian"
  `))

	c.Assert(err, IsNil)

	b, err := runBuilder(fmt.Sprintf(`
    import "%s"
  `, f.Name()))
	c.Assert(err, IsNil)
	c.Assert(b.ImageID(), Not(Equals), "")
	b.Close()

	b, err = runBuilder(`
    import "/nonexistent"
  `)
	c.Assert(err, NotNil)
	c.Assert(b.ImageID(), Equals, "")
	b.Close()
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
	b.Close()

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
	b.Close()

	b, err = runBuilder(`
    from "debian"
    copy ".", "test"
  `)

	c.Assert(err, IsNil)

	result = readContainerFile(c, b, "/test/builder.go")
	c.Assert(content, DeepEquals, result)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    workdir "/test"
    copy ".", "test/"
  `)

	c.Assert(err, IsNil)

	result = readContainerFile(c, b, "/test/test/builder.go")
	c.Assert(content, DeepEquals, result)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    inside "/test" do
      copy ".", "test/"
    end
  `)

	c.Assert(err, IsNil)

	result = readContainerFile(c, b, "/test/test/builder.go")
	c.Assert(content, DeepEquals, result)

	b.Close()

	b, err = runBuilder(`
    from "debian"
    inside "/test" do
      copy "..", "test/"
    end
  `)

	c.Assert(err, NotNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    inside "/test" do
      copy "testdata/..", "test/"
    end
  `)

	c.Assert(err, IsNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    inside "/test" do
      copy "testdata/../..", "test/"
    end
  `)

	c.Assert(err, NotNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    inside "/test" do
      copy "testdata/../../builder/..", "test/"
    end
  `)

	c.Assert(err, NotNil)
	b.Close()
}

func (bs *builderSuite) TestTag(c *C) {
	b, err := runBuilder(`
    from "debian"
    tag "test"
  `)

	c.Assert(err, IsNil)
	c.Assert(b.exec.Config().Image, Not(Equals), "test")

	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), "test")
	c.Assert(err, IsNil)

	c.Assert(inspect.RepoTags, DeepEquals, []string{"test:latest"})
	b.Close()
}

func (bs *builderSuite) TestFlatten(c *C) {
	b, err := runBuilder(`
    from "debian"
    run "echo foo >bar"
    run "echo here is another layer >a_file"
    tag "notflattened"
    run "chown -R nobody:nogroup a_file"
    flatten
    tag "flattened"
  `)

	c.Assert(err, IsNil)
	c.Assert(b.exec.Config().Image, Not(Equals), "flattened")

	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)

	c.Assert(len(inspect.RootFS.Layers), Equals, 1)

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), "notflattened")
	c.Assert(err, IsNil)
	c.Assert(len(inspect.RootFS.Layers), Not(Equals), 1)
	b.Close()

	result := runContainerCommand(c, b, []string{"/bin/sh", "-c", "/usr/bin/stat -c %U a_file"})
	c.Assert(string(result), Equals, "nobody\n")
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
	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, DeepEquals, strslice.StrSlice{"/bin/cat"})
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"/bin/bash"})
	b.Close()

	// if cmd is set earlier than entrypoint, it should not change
	b, err = runBuilder(`
    from "debian"
    cmd "hi"
    entrypoint "/bin/echo"
  `)

	c.Assert(err, IsNil)
	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, DeepEquals, strslice.StrSlice{"/bin/echo"})
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"hi"})
	b.Close()

	// likewise for entrypoint.
	b, err = runBuilder(`
    from "debian"
    entrypoint "/bin/echo"
    cmd "hi"
  `)

	c.Assert(err, IsNil)
	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, DeepEquals, strslice.StrSlice{"/bin/echo"})
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"hi"})
	b.Close()

	// normal cmd usage.
	b, err = runBuilder(`
    from "debian"
    cmd "hi"
  `)

	c.Assert(err, IsNil)
	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, IsNil)
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"hi"})
	b.Close()
}

func (bs *builderSuite) TestRun(c *C) {
	b, err := runBuilder(`
    from "debian"
    run "echo -n foo >/bar"
  `)

	c.Assert(err, IsNil)
	result := readContainerFile(c, b, "/bar")
	c.Assert(string(result), Equals, "foo")
	b.Close()

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test && chown nobody:nogroup /test"
    with_user "nobody" do
      run "echo -n foo >/test/bar"
    end
  `)

	result = runContainerCommand(c, b, []string{"/bin/sh", "-c", "/usr/bin/stat -c %U /test/bar"})
	c.Assert(string(result), Equals, "nobody\n")
	b.Close()

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test && chown nobody:nogroup /test"
    user "nobody"
    run "echo -n foo >/test/bar"
  `)

	result = runContainerCommand(c, b, []string{"/bin/sh", "-c", "/usr/bin/stat -c %U /test/bar"})
	c.Assert(string(result), Equals, "nobody\n")
	b.Close()

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
	b.Close()

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test"
    workdir "/test"
    run "echo -n foo >bar"
  `)

	c.Assert(err, IsNil)
	result = readContainerFile(c, b, "/test/bar")
	c.Assert(string(result), Equals, "foo")
	b.Close()
}

func (bs *builderSuite) TestWorkDirInside(c *C) {
	b, err := runBuilder(`
    from "debian"
    workdir "."
  `)

	c.Assert(err, NotNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    inside "." do
      run "true"
    end
  `)

	c.Assert(err, NotNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test"
    workdir "/test"
    run "echo -n foo >bar"
  `)

	c.Assert(err, IsNil)
	result := readContainerFile(c, b, "/test/bar")
	c.Assert(string(result), Equals, "foo")

	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.WorkingDir, Equals, "/test")
	b.Close()

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

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.WorkingDir, Equals, "/")

	// this file is used in the copy comparisons
	content, err := ioutil.ReadFile("builder.go")
	c.Assert(err, IsNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test"
    workdir "/test"
    copy ".", "."
  `)

	c.Assert(err, IsNil)
	result = readContainerFile(c, b, "/test/builder.go")
	c.Assert(result, DeepEquals, content)

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.WorkingDir, Equals, "/test")
	b.Close()

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

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.WorkingDir, Equals, "/")
	b.Close()
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

	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.User, Equals, "nobody")
	b.Close()

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

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.User, Equals, "root")
	b.Close()
}

func (bs *builderSuite) TestBuildCache(c *C) {
	// enable cache; will reset on next test run
	os.Setenv("NO_CACHE", "")

	b, err := runBuilder(`
    from "debian"
  `)

	c.Assert(err, IsNil)

	imageID := b.exec.Config().Image
	b.Close()

	b, err = runBuilder(fmt.Sprintf(`
    from "%s"
    run "true"
  `, imageID))

	c.Assert(err, IsNil)

	cached := b.exec.Config().Image
	b.Close()

	b, err = runBuilder(fmt.Sprintf(`
    from "%s"
    run "true"
  `, imageID))

	c.Assert(err, IsNil)
	c.Assert(cached, Equals, b.exec.Config().Image)
	b.Close()

	b, err = runBuilder(fmt.Sprintf(`
    from "%s"
    run "exit 0"
  `, imageID))

	c.Assert(err, IsNil)
	c.Assert(cached, Not(Equals), b.exec.Config().Image)
	b.Close()

	b, err = runBuilder(fmt.Sprintf(`
    from "%s"
    copy ".", "."
  `, imageID))

	c.Assert(err, IsNil)

	cached = b.exec.Config().Image
	b.Close()

	b, err = runBuilder(fmt.Sprintf(`
    from "%s"
    copy ".", "."
  `, imageID))
	c.Assert(err, IsNil)

	c.Assert(cached, Equals, b.exec.Config().Image)

	f, err := os.Create("test")
	c.Assert(err, IsNil)
	defer os.Remove("test")
	f.Close()
	b.Close()

	b, err = runBuilder(fmt.Sprintf(`
    from "%s"
    copy ".", "."
  `, imageID))

	c.Assert(err, IsNil)
	c.Assert(cached, Not(Equals), b.exec.Config().Image)
	b.Close()
}

func (bs *builderSuite) TestSetExec(c *C) {
	b, err := runBuilder(`
    from "debian"
    set_exec cmd: "quux"
  `)
	c.Assert(err, NotNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    set_exec entrypoint: "quux"
  `)
	c.Assert(err, NotNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    set_exec test: ["quux"]
  `)
	c.Assert(err, NotNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    set_exec entrypoint: ["/bin/bash"]
  `)
	c.Assert(err, IsNil)

	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, DeepEquals, strslice.StrSlice{"/bin/bash"})
	b.Close()

	b, err = runBuilder(`
    from "debian"
    set_exec cmd: ["/bin/bash"]
  `)
	c.Assert(err, IsNil)

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"/bin/bash"})
	b.Close()

	b, err = runBuilder(`
    from "debian"
    cmd "exit 0"
    set_exec entrypoint: ["/bin/bash", "-c"]
  `)
	c.Assert(err, IsNil)

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, DeepEquals, strslice.StrSlice{"/bin/bash", "-c"})
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"exit 0"})
	b.Close()

	b, err = runBuilder(`
    from "debian"
    entrypoint "/bin/bash", "-c"
    set_exec cmd: ["exit 0"]
  `)
	c.Assert(err, IsNil)

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, DeepEquals, strslice.StrSlice{"/bin/bash", "-c"})
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"exit 0"})
	b.Close()
}

func (bs *builderSuite) TestEnv(c *C) {
	b, err := runBuilder(`
    from "debian"
    env GOPATH: "/go"
  `)
	c.Assert(err, IsNil)

	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)

	found := false

	for _, str := range inspect.Config.Env {
		if str == "GOPATH=/go" {
			found = true
		}
	}

	c.Assert(found, Equals, true)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    env "GOPATH" => "/go", "PATH" => "/usr/local"
  `)
	c.Assert(err, IsNil)

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)

	count := 0

	for _, str := range inspect.Config.Env {
		switch str {
		case "GOPATH=/go":
			count++
		case "PATH=/usr/local":
			count++
		default:
		}
	}

	c.Assert(count, Equals, 2)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    env "TERM" => "myterm"
    tag "builder-env-base"
  `)
	c.Assert(err, IsNil)
	b.Close()

	b, err = runBuilder(`
    from "builder-env-base"
    tag "builder-env"
  `)

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), "builder-env")
	c.Assert(err, IsNil)
	found = false

	for _, item := range inspect.Config.Env {
		if item == "TERM=myterm" {
			found = true
			break
		}
	}

	c.Assert(found, Equals, true)
	b.Close()
}

func (bs *builderSuite) TestReaderFuncs(c *C) {
	b, err := runBuilder(`
    from "debian"
    run "echo -n #{getuid("root")} > /uid"
    run "echo -n #{getgid("nogroup")} > /gid"
    run "echo -n '#{read("/etc/passwd")}' > /passwd"
  `)
	c.Assert(err, IsNil)
	b.Close()

	content, err := b.exec.CopyOneFileFromContainer("/uid")
	c.Assert(err, IsNil)
	c.Assert(string(content), Equals, "0")

	content, err = b.exec.CopyOneFileFromContainer("/gid")
	c.Assert(err, IsNil)
	c.Assert(string(content), Equals, "65534")

	content, err = b.exec.CopyOneFileFromContainer("/passwd")
	c.Assert(err, IsNil)

	origContent, err := b.exec.CopyOneFileFromContainer("/etc/passwd")
	c.Assert(err, IsNil)

	c.Assert(content, DeepEquals, origContent)

	b, err = runBuilder(`
    from "debian"
    puts read("/nonexistent")
  `)
	c.Assert(err, NotNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    puts getuid("quux")
  `)
	c.Assert(err, NotNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    puts getgid("quux")
  `)
	c.Assert(err, NotNil)
	b.Close()
}

func (bs *builderSuite) TestExecPropagation(c *C) {
	b, err := runBuilder(`
    from "debian"
    run "useradd -s /bin/bash -m -d /home/test test"
    env something: "here"
    run "apt-get update"
    user "test"
    tag "test"
  `)
	c.Assert(err, IsNil)

	c.Assert(b.exec.Config().Entrypoint.Image, IsNil)
	c.Assert(b.exec.Config().Cmd.Image, DeepEquals, []string{"/bin/bash"})

	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), "test")
	c.Assert(strslice.StrSlice(b.exec.Config().Cmd.Image), DeepEquals, inspect.Config.Cmd)

	// Docker rewrites a nil as the array below.
	c.Assert(strslice.StrSlice{"/bin/sh", "-c"}, DeepEquals, inspect.Config.Entrypoint)

	b.Close()
}
