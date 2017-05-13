package main

import (
	"os"

	. "gopkg.in/check.v1"
)

func (s *cliSuite) TestVars(c *C) {
	os.Setenv("NO_CACHE", "")

	cmd, err := build(`
    from "debian"
		copy ".", "."
		run "test -f #{var("testfile")}"
  `, "-n", "-v", "testfile=test.rb")

	c.Assert(err, IsNil)
	checkSuccess(c, cmd)

	// check --var as well as -v
	cmd, err = build(`
    from "debian"
		copy ".", "."
		run "test -f #{var("testfile")}"
  `, "-n", "--var", "testfile=test.rb")

	c.Assert(err, IsNil)
	checkSuccess(c, cmd)

	cmd, err = build(`
    from "debian"
		copy ".", "."
		run "test -f #{var("testfile")}"
	`, "-n")

	c.Assert(err, IsNil)
	checkFailure(c, cmd)
}
