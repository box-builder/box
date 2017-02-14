package tar

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/archive"
	"github.com/erikh/box/copy"
	"github.com/erikh/box/logger"
)

// rewriteTar rewrites the tar's paths to copy the source to the target.
func rewriteTar(source, target string, logger *logger.Logger, tr *tar.Reader, tw *tar.Writer) error {
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

		if header.Linkname != "" {
			rel, err := filepath.Rel(header.Linkname, source)
			if err != nil {
				return err
			}

			if strings.HasPrefix(rel, "../") {
				return fmt.Errorf("path for symlink %q (source: %q) falls below copy root", rel, source)
			}
		}

		if (dir || target[len(target)-1] == '/') && header.Name[0] != '/' {
			// not a single file
			header.Linkname = path.Join(target, header.Linkname)
			header.Name = path.Join(target, header.Name)
		} else {
			header.Linkname = target
			header.Name = target
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if err := copy.WithProgress(tw, tr, logger, fmt.Sprintf("%s -> %s", path.Join(source, header.Name[1:]), header.Name[1:])); err != nil {
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
