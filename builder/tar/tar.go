package tar

import (
	"archive/tar"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/erikh/box/log"
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

	tw := tar.NewWriter(f)

	if fi.IsDir() {
		err := filepath.Walk(rel, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			log.CopyPath(path, filepath.Join(target, path))

			header, err := tar.FileInfoHeader(fi, filepath.Join(target, path))
			if err != nil {
				return err
			}

			header.Linkname = filepath.Join(target, path)
			header.Name = filepath.Join(target, path)

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			p, err := os.Open(path)
			if err != nil {
				return err
			}

			if header.Typeflag == tar.TypeReg {
				_, err = io.Copy(tw, p)
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
		_, err = io.Copy(tw, p)
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
func SumFile(fn string) (string, error) {
	f, err := os.Open(fn)
	if err != nil {
		return "", err
	}

	hash := sha512.New512_256()
	_, err = io.Copy(hash, f)
	if err != nil && err != io.EOF {
		f.Close()
		return "", err
	}
	cacheKey := fmt.Sprintf("box:copy %s", hex.EncodeToString(hash.Sum(nil)))
	f.Close()

	return cacheKey, nil
}
