package builder

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	. "testing"
	"time"

	"github.com/box-builder/box/builder/command"
	btypes "github.com/box-builder/box/types"
	"github.com/docker/docker/api/types"
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
	command.ResetPulls()
}

func (bs *builderSuite) TearDownTest(c *C) {
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

func (bs *builderSuite) TestFrom(c *C) {
	b, err := runBuilder(`
		from "alpine"
	`)

	c.Assert(err, IsNil)
	b.Close()

	b, err = runBuilder(`
		from "quezacoatl"
	`)

	c.Assert(err, NotNil)
	b.Close()
}

func (bs *builderSuite) TestAfter(c *C) {
	b, err := runBuilder(`
		from "alpine"
		after { tag "test" }
	`)
	c.Assert(err, IsNil)
	b.Close()

	_, _, err = dockerClient.ImageInspectWithRaw(context.Background(), "test")
	c.Assert(err, IsNil)
}

func (bs *builderSuite) TestContext(c *C) {
	toCtx, cancel := context.WithTimeout(context.Background(), time.Second)

	b, err := NewBuilder(BuildConfig{Globals: &btypes.Global{Context: toCtx}, Runner: make(chan struct{})})
	c.Assert(err, IsNil)

	errChan := make(chan error)

	go func() {
		errChan <- b.eval.RunScript(`
			from "debian"
			run "sleep 2"
			run "ls"
		`)
	}()

	c.Assert(<-errChan, NotNil)
	b.Close()

	cancelCtx, cancel := context.WithCancel(context.Background())
	b, err = NewBuilder(BuildConfig{Globals: &btypes.Global{Context: cancelCtx}, Runner: make(chan struct{})})
	c.Assert(err, IsNil)

	command.ResetPulls() // manually reset so the download starts again

	go func() {
		errChan <- b.eval.RunScript(`
			from "debian"
			run "sleep 2"
			run "ls"
		`)
	}()

	go func() {
		time.Sleep(time.Second)
		cancel()
	}()

	c.Assert(<-errChan, NotNil)
	b.Close()
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
	c.Assert(b.exec.Image().ImageID(), Not(Equals), "")
	b.Close()

	b, err = runBuilder(`
    import "/nonexistent"
  `)
	c.Assert(err, NotNil)
	c.Assert(b.exec.Image().ImageID(), Equals, "")
	b.Close()
}

func (bs *builderSuite) TestCopyToRelativePathWithWorkdir(c *C) {
	b, err := runBuilder(`
    from "debian"
    run "mkdir /test"
    workdir "/test"
    copy ".", "builder"
    run "test -f /test/builder/builder.go"
  `)
	c.Assert(err, IsNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test"
    workdir "/test"
    copy "config", "."
    run "test -f /test/config/config.go"
  `)
	c.Assert(err, IsNil)
	b.Close()
}

func (bs *builderSuite) TestCopyWithGlob(c *C) {
	b, err := runBuilder(`
    from "debian"
    run "mkdir /test"
    workdir "/test"
    copy "*", "."
    run "test -f /test/config/config.go"
		run "test -f /test/builder.go"
  `)
	c.Assert(err, IsNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test"
    workdir "/test"
    copy "*", "."
		run "test -f /test/\\*"
	`)
	c.Assert(err, NotNil)
	b.Close()
}

func (bs *builderSuite) TestCopyWithIgnore(c *C) {
	b, err := runBuilder(`
		from "debian"
		copy ".", "builder", ignore_list: ["builder.go"]
		run "ls /builder"
		run "test -f /builder/builder.go"
	`)
	c.Assert(err, NotNil)
	b.Close()

	f, err := os.Create("filelist")
	c.Assert(err, IsNil)

	_, err = f.Write([]byte("builder.go\nutil.go\n"))
	c.Assert(err, IsNil)
	f.Close()

	b, err = runBuilder(`
		from "debian"
		copy ".", "builder", ignore_file: "filelist"
		run "test -f /builder/builder.go || test -f /builder/util.go"
	`)
	c.Assert(err, NotNil)
	b.Close()

	os.Remove(f.Name())

	f, err = os.Create(".dockerignore")
	c.Assert(err, IsNil)

	_, err = f.Write([]byte("builder.go\nutil.go\n"))
	c.Assert(err, IsNil)
	f.Close()

	b, err = runBuilder(`
		from "debian"
		copy ".", "builder"
		run "test -f /builder/builder.go || test -f /builder/util.go"
	`)
	c.Assert(err, NotNil)
	b.Close()

	os.Remove(f.Name())
}

func (bs *builderSuite) TestCopyOverDir(c *C) {
	testpath := filepath.Join(dockerfilePath, "test1.rb")

	_, err := runBuilder(fmt.Sprintf(`
    from "debian"
    copy "%s", "/tmp"
  `, testpath))
	c.Assert(err, NotNil)

	_, err = runBuilder(fmt.Sprintf(`
    from "debian"
    copy "%s", "/tmp/"
    run "test -f /tmp/test1.rb"
  `, testpath))
	c.Assert(err, IsNil)
}

func (bs *builderSuite) TestCopyOverVolume(c *C) {
	// box deliberately does not support image volumes, so we must build from docker first.
	cmd := exec.Command("docker", "build", "-t", "volumes", "-f", "testdata/dockerfiles/Dockerfile.volumes", ".")
	out, err := cmd.CombinedOutput()
	c.Assert(err, IsNil, Commentf("%v", string(out)))

	_, err = runBuilder(`
  from "volumes"
  copy ".", "/tmp/"
  `)
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
		run "mkdir /test"
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

func (bs *builderSuite) TestSave(c *C) {
	b, err := runBuilder(`
    from "debian"
		save tag: "test"
  `)

	c.Assert(err, IsNil)
	c.Assert(b.exec.Config().Image, Not(Equals), "test")

	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), "test")
	c.Assert(err, IsNil)

	var found bool

	for _, tag := range inspect.RepoTags {
		if tag == "test:latest" {
			found = true
		}
	}

	c.Assert(found, Equals, true)
	b.Close()

	b, err = runBuilder(`
    from "debian"
		run "apt-get update -qq"
		save file: "test.tar"
  `)
	c.Assert(err, IsNil)
	b.Close()

	defer os.Remove("test.tar")
	f, err := os.Open("test.tar")
	c.Assert(err, IsNil)

	r, err := dockerClient.ImageLoad(context.Background(), f, true)
	c.Assert(err, IsNil)
	io.Copy(ioutil.Discard, r.Body)

	b, err = runBuilder(`
    from "debian"
		save file: "../test.tar"
  `)
	c.Assert(err, NotNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
		save file: "/test.tar"
  `)
	c.Assert(err, NotNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
		run "apt-get update -qq"
		save file: "oci.tar", kind: "oci"
  `)
	c.Assert(err, IsNil)
	b.Close()

	defer os.Remove("oci.tar")
	f, err = os.Open("oci.tar")
	c.Assert(err, IsNil)

	tr := tar.NewReader(f)
	found = false
	for {
		header, err := tr.Next()
		c.Assert(err, IsNil)

		if path.Base(header.Name) == "oci" {
			found = true
			break
		}
	}

	c.Assert(found, Equals, true)
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

	b, err = runBuilder(`
    from "debian"
		entrypoint []
		cmd []
  `)
	c.Assert(err, IsNil)

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)

	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"/bin/sh"})
	c.Assert(inspect.Config.Entrypoint, IsNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
		entrypoint []
		cmd ["/bin/bash"]
  `)
	c.Assert(err, IsNil)

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"/bin/bash"})
	c.Assert(inspect.Config.Entrypoint, IsNil)
	b.Close()

	b, err = runBuilder(`
    from "debian"
		entrypoint %w[/bin/echo -e]
		cmd %w[foo bar quux baz]
  `)
	c.Assert(err, IsNil)
	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), b.exec.Config().Image)
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Entrypoint, DeepEquals, strslice.StrSlice{"/bin/echo", "-e"})
	c.Assert(inspect.Config.Cmd, DeepEquals, strslice.StrSlice{"foo", "bar", "quux", "baz"})
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
	c.Assert(err, IsNil)

	result = runContainerCommand(c, b, []string{"/bin/sh", "-c", "/usr/bin/stat -c %U /test/bar"})
	c.Assert(string(result), Equals, "nobody\n")
	b.Close()

	b, err = runBuilder(`
    from "debian"
    run "mkdir /test && chown nobody:nogroup /test"
    user "nobody"
    run "echo -n foo >/test/bar"
  `)
	c.Assert(err, IsNil)

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

	c.Assert(err, IsNil)
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
    env "TERM" => "myterm", "PATH" => "/test"
    tag "builder-env-base"
  `)
	c.Assert(err, IsNil)
	b.Close()

	b, err = runBuilder(`
    from "builder-env-base"
    tag "builder-env"
  `)
	c.Assert(err, IsNil)

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

	count = 0

	for _, item := range inspect.Config.Env {
		if strings.HasPrefix(item, "PATH=") {
			count++
		}
	}

	c.Assert(count, Equals, 1)
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
	c.Assert(err, IsNil)
	c.Assert(strslice.StrSlice(b.exec.Config().Cmd.Image), DeepEquals, inspect.Config.Cmd)

	// Docker rewrites a nil as the array below.
	c.Assert(strslice.StrSlice{"/bin/sh", "-c"}, DeepEquals, inspect.Config.Entrypoint)

	b.Close()
}

func (bs *builderSuite) TestLabels(c *C) {
	_, err := runBuilder(`
		from "debian"
		label
		tag "failed"
	`)
	c.Assert(err, NotNil)

	_, err = runBuilder(`
		from "debian"
		label "foo" => "bar"
		tag "labeled"
	`)
	c.Assert(err, IsNil)

	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), "labeled")
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Labels["foo"], Equals, "bar")

	_, err = runBuilder(`
		from "debian"
		label foo2: "bar"
		tag "labeled"
	`)
	c.Assert(err, IsNil)

	inspect, _, err = dockerClient.ImageInspectWithRaw(context.Background(), "labeled")
	c.Assert(err, IsNil)
	c.Assert(inspect.Config.Labels["foo2"], Equals, "bar")
}

func (bs *builderSuite) TestInsideRelativeWorkDir(c *C) {
	_, err := runBuilder(`
		from "debian"
		workdir "/etc"
		inside "apt" do
			run "ls"
		end
	`)

	c.Assert(err, IsNil)

	_, err = runBuilder(`
		from "debian"
		inside "/etc" do
			inside "apt" do
				run "ls"
			end
		end
	`)
	c.Assert(err, IsNil)

	// work dir is the default for debian here which is `/`. This should pass.
	_, err = runBuilder(`
		from "debian"
		inside "etc" do
			inside "apt" do
				run "ls"
			end
		end
	`)
	c.Assert(err, IsNil)

	_, err = runBuilder(`
		from "debian"
		workdir "/etc"
		inside "/" do
			run "cd tmp"
		end
	`)
	c.Assert(err, IsNil)

	_, err = runBuilder(`
		from "debian"
		workdir "/etc"
		inside "/" do
			run "cd tmp"
		end
	`)
	c.Assert(err, IsNil)

	_, err = runBuilder(`
		from "debian"
		workdir "/home/box-builder"
		copy ".", "box/"
	`)
	c.Assert(err, IsNil)

	_, err = runBuilder(`
		from "debian"
		workdir "/home/box-builder"
		copy ".", "box"
	`)
	c.Assert(err, IsNil)

	path, err := filepath.Abs("..")
	c.Assert(err, IsNil)

	defer os.Remove("test")
	c.Assert(os.Symlink(path, "test"), IsNil)

	_, err = runBuilder(`
		from "debian"
		workdir "/home/box-builder"
		copy ".", "box"
		run "stat /home/box-builder/box/test", output: false
	`)
	c.Assert(err, IsNil)

	os.Remove("test")

	_, err = runBuilder(`
		from "debian"
		workdir "/home/box-builder"
		copy "builder.go", "/builder.go"
	`)
	c.Assert(err, IsNil)

	_, err = runBuilder(`
		from "debian"
		copy ".", "/go/src/github.com/box-builder/box/builder/"
		run "ls /go/src/github.com/box-builder/box/builder/"
	`)
	c.Assert(err, IsNil)
}
