package tar

import (
	"archive/tar"
	"io/ioutil"
	"os"
	"strings"
	. "testing"

	. "gopkg.in/check.v1"
)

type tarSuite struct{}

var _ = Suite(&tarSuite{})

func TestTar(t *T) {
	TestingT(t)
}

func (ts *tarSuite) TestSumFile(c *C) {
	size := 1024
	shasum := "5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef"

	buf := make([]byte, size, size)

	f, err := ioutil.TempFile("", "box-temp")
	c.Assert(err, IsNil)
	defer os.Remove(f.Name())

	n, err := f.Write(buf)
	c.Assert(err, IsNil)
	c.Assert(n, Equals, size)

	f.Close()

	sum, err := SumFile(f.Name())
	c.Assert(err, IsNil)
	c.Assert(sum, Equals, shasum)
}

func (ts *tarSuite) TestArchive(c *C) {
	tarball, err := Archive(".", "/")
	c.Assert(err, IsNil)
	c.Assert(tarball, Not(Equals), "")
	defer os.Remove(tarball)

	f, err := os.Open(tarball)
	c.Assert(err, IsNil)
	defer f.Close()

	r := tar.NewReader(f)

	for {
		header, err := r.Next()
		if err != nil {
			break
		}

		c.Assert(strings.HasPrefix(header.Name, "/"), Equals, true)
	}
}
