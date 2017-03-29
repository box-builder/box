package docker

import (
	"context"
	"errors"

	"github.com/box-builder/box/logger"
	"github.com/box-builder/box/types"

	. "gopkg.in/check.v1"
)

var (
	runGlobals = &types.Global{Context: context.Background(), Logger: logger.New("", false), ShowRun: true}
)

func (ds *dockerSuite) TestRunCommit(c *C) {
	commit := func(ctx context.Context, id string) (string, error) {
		return "cachekey", nil
	}

	fail := func(ctx context.Context, id string) (string, error) {
		return "", errors.New("an error")
	}

	d, err := NewDocker(runGlobals)
	c.Assert(err, IsNil)
	id, err := d.Layers().Fetch(d.config, "debian:latest")
	c.Assert(err, IsNil)
	c.Assert(d.Commit("", commit), IsNil)
	c.Assert(d.config.Image, Not(Equals), id)

	d, err = NewDocker(runGlobals)
	c.Assert(err, IsNil)
	id, err = d.Layers().Fetch(d.config, "debian:latest")
	c.Assert(err, IsNil)
	c.Assert(d.Commit("", fail), NotNil)
	c.Assert(d.config.Image, Equals, id)
}

func (ds *dockerSuite) TestRunHook(c *C) {
	d, err := NewDocker(runGlobals)
	c.Assert(err, IsNil)
	id, err := d.Layers().Fetch(d.config, "debian:latest")
	c.Assert(err, IsNil)

	d.config.Entrypoint.Temporary = []string{"/bin/sh", "-c"}
	d.config.Cmd.Temporary = []string{"exit 0"}
	c.Assert(d.Commit("test", d.RunHook), IsNil)
	c.Assert(d.config.Image, Not(Equals), id)

	d, err = NewDocker(runGlobals)
	c.Assert(err, IsNil)
	id, err = d.Layers().Fetch(d.config, "debian:latest")
	c.Assert(err, IsNil)

	createID, err := d.Create()
	c.Assert(err, IsNil)
	defer d.Destroy(createID)

	d.config.Entrypoint.Temporary = []string{"/bin/sh", "-c"}
	d.config.Cmd.Temporary = []string{"exit 1"}
	c.Assert(d.Commit("test", d.RunHook), NotNil)
	c.Assert(d.config.Image, Equals, id)
}
