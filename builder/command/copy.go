package command

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/box-builder/box/tar"
	"github.com/box-builder/box/util"
	"github.com/pkg/errors"
)

// Copy implements `copy`
func (i *Interpreter) Copy(source, target string, ignoreList []string) error {
	if err := i.hasImage(); err != nil {
		return err
	}

	list, err := util.ReadLines(".dockerignore")
	if os.IsNotExist(err) {
		list = []string{}
	} else if err != nil {
		return err
	}

	ignoreList = append(ignoreList, list...)

	// XXX for if we ever add volume support back
	for _, volume := range i.exec.Config().Volumes {
		if strings.HasPrefix(target, volume) {
			return errors.Errorf("Volume %q cannot be copied into (you tried %q). This is caused by a bug in docker. We are working with docker on a fix.", volume, target)
		}
	}

	fn, cacheKey, err := tar.Archive(i.globals.Context, source, target, ignoreList, i.globals.Logger)
	if err != nil {
		return err
	}
	defer os.Remove(fn)

	cacheKey = fmt.Sprintf("box:copy %s", cacheKey)

	cached, err := i.exec.Image().CheckCache(cacheKey)
	if err != nil {
		return err
	}

	if cached {
		return nil
	}

	f, err := os.Open(fn)
	if err != nil {
		return err
	}

	defer f.Close()

	hook := func(ctx context.Context, id string) (string, error) {
		return "", i.exec.CopyToContainer(id, f)
	}

	return i.exec.Commit(cacheKey, hook)
}
