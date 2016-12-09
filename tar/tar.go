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

func archiveSingle(rel, target string, tw *tar.Writer) error {
	fi, err := os.Lstat(rel)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(fi, target)
	if err != nil {
		return err
	}

	header.Name = target
	header.Linkname = target

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	p, err := os.Open(rel)
	if err != nil {
		return err
	}

	defer p.Close()

	return copy.WithProgress(tw, p, fmt.Sprintf("Writing %s", rel))
}

func archiveWalk(rel, target string, tw *tar.Writer) filepath.WalkFunc {
	return func(path string, fi os.FileInfo, err error) error {
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
	}
}

// Archive takes a source and target directory and returns a filename and/or
// error. The source will be archived relative to the target. The file will
// live in the user's os.TempDir().
func Archive(rel, target string) (string, string, error) {
	entries, err := filepath.Glob(rel)
	if err != nil {
		return "", "", err
	}

	f, err := ioutil.TempFile("", "box-copy.")
	if err != nil {
		return "", "", err
	}

	signal.SetSignal(func() { os.Remove(f.Name()) })
	defer signal.SetSignal(nil)

	hash := sha256.New()
	r, w := io.Pipe()
	tw := tar.NewWriter(w)

	tee := io.TeeReader(r, hash)
	go io.Copy(f, tee)

	for _, entry := range entries {
		fi, err := os.Lstat(entry)
		if err != nil {
			return "", "", err
		}
		if fi.IsDir() {
			if err := filepath.Walk(entry, archiveWalk(entry, target, tw)); err != nil {
				return "", "", err
			}
		} else {
			if err := archiveSingle(entry, target, tw); err != nil {
				return "", "", err
			}
		}
	}

	tw.Close()
	f.Close()

	return f.Name(), hex.EncodeToString(hash.Sum(nil)), nil
}

// SumWithCopy simultaneously sums and copies a stream.
func SumWithCopy(writer io.WriteCloser, reader io.Reader, fileType string) (string, error) {
	hashReader, hashWriter := io.Pipe()
	tarReader := io.TeeReader(reader, hashWriter)

	sumChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		sum, err := SumReader(hashReader)
		if err != nil {
			errChan <- err
		} else {
			sumChan <- sum
		}
	}()

	if err := copy.WithProgress(writer, tarReader, fileType); err != nil {
		writer.Close()
		return "", err
	}

	writer.Close()
	hashWriter.Close()

	var sum string

	select {
	case err := <-errChan:
		return "", err
	case sum = <-sumChan:
	}

	return sum, nil
}

// SumReader sums an io.Reader
func SumReader(reader io.Reader) (string, error) {
	hash := sha256.New()
	_, err := io.Copy(hash, reader)
	return hex.EncodeToString(hash.Sum(nil)), err
}
