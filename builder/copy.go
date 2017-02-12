package builder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erikh/box/tar"
	mruby "github.com/mitchellh/go-mruby"
)

func parseCopyArgs(args []*mruby.MrbValue) (string, string, []string, error) {
	var source, target string
	ignoreList := []string{}

	for _, arg := range args {
		switch arg.Type() {
		case mruby.TypeString:
			if source != "" {
				if target != "" {
					return "", "", nil, errors.New("too many arguments in copy")
				}

				target = arg.String()
				continue
			}
			source = arg.String()
		case mruby.TypeHash:
			hash, err := coerceHash(arg.Hash())
			if err != nil {
				return "", "", nil, err
			}

			if _, ok := hash["ignore_list"]; ok {
				list, err := interfaceListToString(hash["ignore_list"])
				if err != nil {
					return "", "", nil, err
				}

				ignoreList = append(ignoreList, list...)
			}

			file, ok := hash["ignore_file"].(string)
			if ok {
				lines, err := readLines(file)
				if err != nil {
					return "", "", nil, err
				}

				ignoreList = append(ignoreList, lines...)
			}
		}
	}

	return source, target, ignoreList, nil
}

func checkCopyArgs(b *Builder, args []*mruby.MrbValue) (string, string, []string, error) {
	if err := checkImage(b); err != nil {
		return "", "", nil, err
	}

	source, target, ignoreList, err := parseCopyArgs(args)
	if err != nil {
		return "", "", nil, err
	}

	var rel string

	relfiles, err := filepath.Glob(source)
	if err != nil || len(relfiles) == 1 {
		source, err = filepath.Abs(source)
		if err != nil {
			return "", "", nil, err
		}

		wd, err := os.Getwd()
		if err != nil {
			return "", "", nil, err
		}

		rel, err = filepath.Rel(wd, source)
		if err != nil {
			return "", "", nil, err
		}

		if strings.HasPrefix(rel, "..") {
			return "", "", nil, fmt.Errorf("cannot use relative path %s because it may fall below the root build directory", source)
		}
	} else {
		rel = source
	}

	workdir := b.exec.Config().WorkDir
	var targetWd string

	if workdir.Temporary == "" {
		targetWd = workdir.Image
	} else {
		targetWd = workdir.Temporary
	}

	// special case `.`
	if target == "." && len(relfiles) == 1 {
		target = filepath.Join(targetWd, rel)
	} else {
		if !strings.HasPrefix(target, "/") {
			target = filepath.Join(targetWd, target)
		}
	}

	return filepath.Clean(rel), target, ignoreList, nil
}

func doCopy(b *Builder, cacheKey string, args []*mruby.MrbValue, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	rel, target, ignoreList, err := checkCopyArgs(b, args)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	list, err := readDockerIgnore()
	if err != nil {
		return nil, createException(m, err.Error())
	}

	ignoreList = append(ignoreList, list...)

	// XXX for if we ever add volume support back
	for _, volume := range b.exec.Config().Volumes {
		if strings.HasPrefix(target, volume) {
			return nil, createException(m, fmt.Sprintf("Volume %q cannot be copied into (you tried %q). This is caused by a bug in docker. We are working with docker on a fix.", volume, target))
		}
	}

	fn, cacheKey, err := tar.Archive(b.config.Context, rel, target, ignoreList)
	if err != nil {
		return nil, createException(m, err.Error())
	}
	defer os.Remove(fn)

	cacheKey = fmt.Sprintf("box:copy %s", cacheKey)

	if b.exec.Image().GetCache() {
		cached, err := b.exec.Image().CheckCache(cacheKey)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		if cached {
			return nil, nil
		}
	}

	f, err := os.Open(fn)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer f.Close()

	hook := func(ctx context.Context, id string) (string, error) {
		return "", b.exec.CopyToContainer(id, f)
	}

	if err := b.exec.Commit(cacheKey, hook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}
