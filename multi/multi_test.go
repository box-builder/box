package multi

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	. "testing"

	"github.com/box-builder/box/builder"
	"github.com/box-builder/box/logger"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"

	. "gopkg.in/check.v1"
)

type multiSuite struct{}

var _ = Suite(&multiSuite{})

func TestMulti(t *T) {
	TestingT(t)
}

var SuccessPlans = map[int]string{
	1: `
	from "debian"
	cmd "ls"
	tag "success1"
	`,
	2: `
	from "alpine"
	entrypoint "/bin/sh"
	tag "success2"
	`,
	3: `
	from "alpine"
	workdir "/tmp"
	tag "success3"
	`,
	4: `
	from "debian"
	run "rm -rf /tmp/*"
	tag "success4"
	`,
	5: `
	from "alpine"
	user "root"
	tag "success5"
	`,
}

var FailPlans = map[int]string{
	1: `
	from "debian"
	run "quux"
	tag "fail1"
	`,
	2: `
	from "alpine"
	syntax error
	tag "fail2"
	`,
	3: `
	from "alpine"
	workdir "/bin"
	user "nobody"
	run "touch permission-error"
	tag "fail3"
	`,
	4: `
	from "debian"
	run "exit 1"
	tag "fail4"
	`,
	5: `
	from "quezacoatl"
	tag "fail5"
	`,
}

var dockerClient *client.Client

func (ms *multiSuite) SetUpSuite(c *C) {
	var err error

	dockerClient, err = client.NewEnvClient()
	c.Assert(err, IsNil)
}

func (ms *multiSuite) SetUpTest(c *C) {
	os.Setenv("NO_CACHE", "1")
	builder.ResetPulls()
}

func mkPlanDir(dir string, i int) string {
	return filepath.Join(dir, fmt.Sprintf("%d.rb", i))
}

func mkPlans(plans map[int]string) string {
	dir, err := ioutil.TempDir("", "box-plans")
	if err != nil {
		panic(err)
	}

	for i, plan := range plans {
		if err := ioutil.WriteFile(mkPlanDir(dir, i), []byte(plan), 0666); err != nil {
			panic(err)
		}
	}

	return dir
}

func mkBuilders(plans map[int]string) []*builder.Builder {
	dir := mkPlans(plans)

	builders := []*builder.Builder{}

	for i := range plans {
		l := logger.New("", false)
		l.Record()

		b, err := builder.NewBuilder(builder.BuildConfig{
			Context:  context.Background(),
			Runner:   make(chan struct{}),
			Cache:    os.Getenv("NO_CACHE") == "",
			FileName: mkPlanDir(dir, i),
			Logger:   l,
		})

		if err != nil {
			panic(err)
		}

		builders = append(builders, b)
	}

	return builders
}

func (ms *multiSuite) TestBuilderBasic(c *C) {
	mb := NewBuilder(mkBuilders(SuccessPlans))
	mb.Build()
	c.Assert(mb.Wait(), IsNil)
	images, err := dockerClient.ImageList(context.Background(), types.ImageListOptions{})
	c.Assert(err, IsNil)

	filtered := []types.ImageSummary{}

	for _, img := range images {
		if len(img.RepoTags) != 0 {
			if strings.HasPrefix(img.RepoTags[0], "success") {
				filtered = append(filtered, img)
			}
		}
	}

	defer func(filtered []types.ImageSummary) {
		for _, img := range filtered {
			_, err := dockerClient.ImageRemove(context.Background(), img.ID, types.ImageRemoveOptions{Force: true})
			c.Assert(err, IsNil)
		}
	}(filtered)

	c.Assert(len(filtered), Equals, len(SuccessPlans))

	mb = NewBuilder(mkBuilders(FailPlans))
	mb.Build()
	c.Assert(mb.Wait(), NotNil)
	images, err = dockerClient.ImageList(context.Background(), types.ImageListOptions{})
	c.Assert(err, IsNil)

	filtered = []types.ImageSummary{}

	for _, img := range images {
		if len(img.RepoTags) != 0 {
			if strings.HasPrefix(img.RepoTags[0], "fail") {
				filtered = append(filtered, img)
			}
		}
	}

	defer func(filtered []types.ImageSummary) {
		for _, img := range filtered {
			_, err := dockerClient.ImageRemove(context.Background(), img.ID, types.ImageRemoveOptions{Force: true})
			c.Assert(err, IsNil)
		}
	}(filtered)

	c.Assert(len(filtered), Equals, 0)
}

func (ms *multiSuite) TestMultiFrom(c *C) {
	imageName := "alpine"

	_, err := dockerClient.ImageRemove(context.Background(), imageName, types.ImageRemoveOptions{Force: true, PruneChildren: true})
	c.Assert(err, IsNil)

	builders := map[int]string{}

	for i := 0; i < 10; i++ {
		builders[i] = fmt.Sprintf("from %q", imageName)
	}

	mb := NewBuilder(mkBuilders(builders))
	mb.Build()
	c.Assert(mb.Wait(), IsNil)

	var found bool

	for _, b := range mb.builders {
		if strings.Contains(b.Logger.Output().(*bytes.Buffer).String(), fmt.Sprintf("Pulling %q", imageName)) {
			if found {
				c.Fatal("Found two pulls")
			}
			found = true
		}
	}

	c.Assert(found, Equals, true)
}
