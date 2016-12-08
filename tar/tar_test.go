package tar

import (
	"archive/tar"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	. "testing"

	"golang.org/x/sys/unix"

	. "gopkg.in/check.v1"
)

type tarSuite struct{}

var _ = Suite(&tarSuite{})

func TestTar(t *T) {
	TestingT(t)
}

func (ts *tarSuite) TestArchive(c *C) {
	tarball, sum, err := Archive(".", "/")
	c.Assert(err, IsNil)
	c.Assert(sum, Not(Equals), "")
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

func (ts *tarSuite) TestArchiveSpecialFile(c *C) {
	dir, err := ioutil.TempDir("", "tar-test")
	c.Assert(err, IsNil)
	defer os.RemoveAll(dir)

	tmp, err := os.Create(filepath.Join(dir, "test"))
	c.Assert(err, IsNil)
	tmp.Close()
	c.Assert(os.Symlink(tmp.Name(), filepath.Join(dir, "testsym")), IsNil)
	c.Assert(unix.Mkfifo(filepath.Join(dir, "test.fifo"), 0666), IsNil)

	tarball, _, err := Archive(dir, "/")
	c.Assert(err, IsNil)
	c.Assert(tarball, Not(Equals), "")
	defer os.Remove(tarball)

	f, err := os.Open(tarball)
	c.Assert(err, IsNil)
	defer f.Close()

	r := tar.NewReader(f)

	count := 0
	names := []string{}

	for {
		header, err := r.Next()
		if err != nil {
			break
		}

		count++

		if strings.HasSuffix(header.Name, "testsym") {
			c.Assert(strings.HasSuffix(header.Linkname, "test"), Equals, true, Commentf("%v", header.Linkname))
		}

		names = append(names, header.Name)
	}

	c.Assert(count, Equals, 2, Commentf("%v", names))
}
