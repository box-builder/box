package tar

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/box-builder/box/copy"
	"github.com/box-builder/box/logger"
	"github.com/box-builder/box/signal"
	"github.com/docker/docker/pkg/archive"
)

// rewriteTar rewrites the tar's paths to copy the source to the target.
func rewriteTar(source, target string, logger *logger.Logger, tr *tar.Reader, tw *tar.Writer) error {
	// all this code is terrible
	fi, err := os.Stat(source)
	if err != nil {
		return err
	}

	dir := fi.IsDir()

	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		var name string
		if header.Name[0] == '/' {
			name = header.Name[1:]
		} else {
			name = header.Name
		}

		if dir {
			header.Name = filepath.Join(target, name)
		} else {
			if target[len(target)-1] == '/' {
				header.Name = filepath.Join(target, name)
			} else {
				header.Name = target
			}
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if err := copy.WithProgress(tw, tr, logger, fmt.Sprintf("%s -> %s", filepath.Join(source, name), header.Name)); err != nil {
			return err
		}
	}

	return nil
}

func expandIncludeList(source string) (string, []string, error) {
	files, err := filepath.Glob(source)
	if err != nil {
		return "", nil, err
	}

	relFiles := []string{}

	if len(files) > 1 {
		source = filepath.Dir(source)

		for _, file := range files {
			rel, err := filepath.Rel(source, file)
			if err != nil {
				return "", nil, err
			}

			if strings.HasPrefix(rel, "../") {
				return "", nil, fmt.Errorf("path for file %q falls below copy root", rel)
			}

			relFiles = append(relFiles, rel)
		}
	} else {
		return source, []string{}, nil
	}

	return source, relFiles, nil
}

// Archive archives the source into target, ignoring the list of patterns
// supplied in the string array.
func Archive(ctx context.Context, source, target string, ignoreList []string, logger *logger.Logger) (string, string, error) {
	var relFiles []string
	var err error

	source, relFiles, err = expandIncludeList(source)
	if err != nil {
		return "", "", err
	}

	reader, err := archive.TarWithOptions(source, &archive.TarOptions{IncludeFiles: relFiles, ExcludePatterns: ignoreList})
	if err != nil {
		return "", "", err
	}

	f, err := ioutil.TempFile("", "box-archive")
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	signal.Handler.AddFile(f.Name())
	defer signal.Handler.RemoveFile(f.Name())

	tr := tar.NewReader(reader)
	tw := tar.NewWriter(f)

	if err := rewriteTar(source, target, logger, tr, tw); err != nil {
		return "", "", err
	}

	reader.Close()
	tw.Close()

	if _, err := f.Seek(0, 0); err != nil {
		return "", "", err
	}

	var sum string

	if sum, err = SumReader(f); err != nil {
		return "", "", err
	}

	return f.Name(), sum, nil
}
