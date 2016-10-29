package main

import (
	"io/ioutil"
	"os"
	. "testing"

	"github.com/rendon/testcli"

	. "gopkg.in/check.v1"
)

type cliSuite struct{}

var _ = Suite(&cliSuite{})

func TestCLI(t *T) {
	TestingT(t)
}

func (s *cliSuite) SetUpTest(c *C) {
	os.Setenv("NO_CACHE", "1")
}

func build(content string, extraArgs ...string) (*testcli.Cmd, error) {
	if content != "" {
		f, err := ioutil.TempFile("", "box-cli-test")
		if err != nil {
			return nil, err
		}
		defer f.Close()
		defer os.Remove(f.Name())

		_, err = f.Write([]byte(content))
		if err != nil {
			return nil, err
		}

		extraArgs = append(extraArgs, f.Name())
	}

	c := testcli.Command("box", extraArgs...)
	c.Run()

	return c, nil
}

func checkSuccess(c *C, cmd *testcli.Cmd) {
	c.Assert(cmd.Success(), Equals, true, Commentf("stdout:\n%s\nstderr:\n%s\n", cmd.Stdout(), cmd.Stderr()))
}

func checkFailure(c *C, cmd *testcli.Cmd) {
	c.Assert(cmd.Failure(), Equals, true, Commentf("stdout:\n%s\nstderr:\n%s\n", cmd.Stdout(), cmd.Stderr()))
}
