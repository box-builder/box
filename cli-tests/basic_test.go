package main

import (
	. "gopkg.in/check.v1"
)

func (s *cliSuite) TestBasic(c *C) {
	checkSuccess(c, build(`from "debian"`))
	checkSuccess(c, build("", "test.rb"))
}
