package mruby

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/box-builder/box/builder/config"
	"github.com/box-builder/box/util"
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
				list, err := util.InterfaceListToString(hash["ignore_list"])
				if err != nil {
					return "", "", nil, err
				}

				ignoreList = append(ignoreList, list...)
			}

			file, ok := hash["ignore_file"].(string)
			if ok {
				lines, err := util.ReadLines(file)
				if err != nil {
					return "", "", nil, err
				}

				ignoreList = append(ignoreList, lines...)
			}
		}
	}

	return source, target, ignoreList, nil
}

func checkCopyArgs(workdir config.StringState, args []*mruby.MrbValue) (string, string, []string, error) {
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

func (m *MRuby) doCopy(args []*mruby.MrbValue, self *mruby.MrbValue) error {
	source, target, ignores, err := checkCopyArgs(m.Exec.Config().WorkDir, args)
	if err != nil {
		return err
	}
	return m.Interp.Copy(source, target, ignores)
}
