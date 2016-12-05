package tar

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/erikh/box/builder/signal"
	"github.com/erikh/box/copy"
)

// Archive takes a source and target directory and returns a filename and/or
// error. The source will be archived relative to the target. The file will
// live in the user's os.TempDir().
func Archive(rel, target string) (string, error) {
	fi, err := os.Lstat(rel)
	if err != nil {
		return "", err
	}

	f, err := ioutil.TempFile("", "box-copy.")
	if err != nil {
		return "", err
	}

	signal.SetSignal(func() { os.Remove(f.Name()) })
	defer signal.SetSignal(nil)

	tw := tar.NewWriter(f)

	if fi.IsDir() {
		err := filepath.Walk(rel, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			header, err := tar.FileInfoHeader(fi, path)
			if err != nil {
				return err
			}

			if !(header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeSymlink) {
				return nil
			}

			realpath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}

			relpath, err := filepath.Rel(rel, path)
			if err != nil {
				return err
			}

			realpath, err = filepath.Rel(rel, realpath)
			if err != nil {
				return err
			}

			header.Linkname = filepath.Join(target, relpath)
			header.Name = filepath.Join(target, realpath)
			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			if header.Typeflag == tar.TypeReg {
				p, err := os.Open(path)
				if err != nil {
					return err
				}

				err = copy.WithProgress(tw, p, fmt.Sprintf("Writing %s", path))
				if err != nil && err != io.EOF {
					p.Close()
					return err
				}

				p.Close()
			}

			return nil
		})
		if err != nil {
			return "", err
		}
	} else if !fi.IsDir() {
		header, err := tar.FileInfoHeader(fi, target)
		if err != nil {
			return "", err
		}

		header.Name = target
		header.Linkname = target

		if err := tw.WriteHeader(header); err != nil {
			return "", err
		}

		p, err := os.Open(rel)
		if err != nil {
			return "", err
		}
		err = copy.WithProgress(tw, p, fmt.Sprintf("Writing %s", rel))
		if err != nil && err != io.EOF {
			p.Close()
			return "", err
		}
		p.Close()
	}

	tw.Close()
	f.Close()

	return f.Name(), nil
}

// SumFile reads a file an returns a hex-encoded sha512/256.
func SumFile(fn, fileType string) (string, error) {
	f, err := os.Open(fn)
	if err != nil {
		return "", err
	}

	hash := sha256.New()
	err = copy.WithProgress(hash, f, fmt.Sprintf("Summing %s", fileType))
	if err != nil && err != io.EOF {
		f.Close()
		return "", err
	}
	f.Close()

	return hex.EncodeToString(hash.Sum(nil)), nil
}
