package main

import (
	"os"
	"strings"

	. "gopkg.in/check.v1"
)

func (s *cliSuite) TestRunNoOutput(c *C) {
	os.Setenv("NO_CACHE", "")

	cmd, err := build(`
    from "debian"
		copy ".", "."
		run "ls -l", output: false
  `, "-n")

	c.Assert(err, IsNil)
	checkSuccess(c, cmd)
	c.Assert(strings.Contains(cmd.Stdout(), "basic_test.go"), Equals, false, Commentf("%s", cmd.Stdout()))
}
