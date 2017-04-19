package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rendon/testcli"

	. "gopkg.in/check.v1"
)

func (s *cliSuite) TestBasic(c *C) {
	cmd, err := build("", "test.rb")
	c.Assert(err, IsNil)
	checkSuccess(c, cmd)
}

func (s *cliSuite) TestCache(c *C) {
	os.Setenv("NO_CACHE", "")

	cmd, err := build(`
    from "debian"
    run "ls"
    run "ls -l"
  `, "-n")

	c.Assert(err, IsNil)
	checkSuccess(c, cmd)

	c.Assert(strings.Contains(cmd.Stdout(), "Cache"), Equals, false, Commentf("%s", cmd.Stdout()))

	cmd, err = build(`
    from "debian"
    run "ls"
    run "ls -l"
  `)

	c.Assert(err, IsNil)
	cmd.SetEnv([]string{"PATH=" + os.Getenv("PATH")})
	checkSuccess(c, cmd)

	c.Assert(strings.Contains(cmd.Stdout(), "Cache"), Equals, true, Commentf("%s", cmd.Stdout()))

	os.Setenv("NO_CACHE", "1")
	cmd, err = build(`
    from "debian"
    run "ls"
    run "ls -l"
  `)

	c.Assert(err, IsNil)
	checkSuccess(c, cmd)

	c.Assert(strings.Contains(cmd.Stdout(), "Cache"), Equals, false, Commentf("%s", cmd.Stdout()))
}

func (s *cliSuite) TestOmit(c *C) {
	cmd, err := build(
		`
    from "debian"
    `, "-o", "from")

	c.Assert(err, IsNil)
	checkFailure(c, cmd)

	c.Assert(strings.Contains(cmd.Stdout(), "!!! Error: undefined method 'from' for main\n"), Equals, true)
}

func (s *cliSuite) TestTag(c *C) {
	cmd, err := build(
		`
    from "debian"
    run "ls"
    `, "-t", "tagtest")

	c.Assert(err, IsNil)
	checkSuccess(c, cmd)

	c.Assert(strings.Contains(cmd.Stdout(), `Tagged: tagtest`), Equals, true, Commentf("%s", cmd.Stdout()))
}

func (s *cliSuite) TestHelp(c *C) {
	cmd := testcli.Command("box", "--help")
	cmd.Run()
	c.Assert(strings.Contains(cmd.Stdout(), "USAGE:"), Equals, true)

	cmd = testcli.Command("box")
	cmd.Run()
	c.Assert(strings.Contains(cmd.Stdout(), "USAGE:"), Equals, true)
}

func (s *cliSuite) TestVersion(c *C) {
	cmd := testcli.Command("box", "--version")
	cmd.Run()
	c.Assert(strings.Contains(cmd.Stdout(), "box version"), Equals, true)
}

func (s *cliSuite) TestCanonicalFile(c *C) {
	// no args, no box.rb should show usage
	cmd := testcli.Command("box")
	cmd.Run()
	c.Assert(strings.Contains(cmd.Stdout(), "USAGE:"), Equals, true)

	// no args, with box.rb should build image
	tmpPath := filepath.Join(os.TempDir(), "boxtest")
	contents := []byte("from 'debian'\ntag 'boxrbtest'\n")
	err := os.Mkdir(tmpPath, 0644)
	c.Assert(err, IsNil)
	err = ioutil.WriteFile(filepath.Join(tmpPath, "/box.rb"), contents, 0644)
	c.Assert(err, IsNil)
	cwd, oserr := os.Getwd()
	c.Assert(oserr, IsNil)
	os.Chdir(tmpPath)

	cmd = testcli.Command("box")
	cmd.Run()
	c.Assert(strings.Contains(cmd.Stdout(), "Execute: tag boxrbtest"), Equals, true, Commentf("%s", cmd.Stdout()))

	// cleanup
	os.Chdir(cwd)
	os.RemoveAll(tmpPath)
}
