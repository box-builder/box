package builder

/*
  funcs.go provides functions for use within box build operations that do *not*
  commit a layer or otherwise directly influence the build. They are intended to
  be used as gathering functions for predicates and templating.

  Please refer to https://erikh.github.io/box/functions/ for documentation on
  how each function operates.
*/

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	mruby "github.com/mitchellh/go-mruby"
)

type funcDefinition struct {
	fun     funcFunc
	argSpec mruby.ArgSpec
}

type funcFunc func(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value)

// mrubyJumpTable is the dispatch instructions sent to the mruby interpreter at builder setup.
var funcJumpTable = map[string]funcDefinition{
	"import": {importFunc, mruby.ArgsReq(1)},
	"getenv": {getenv, mruby.ArgsReq(1)},
	"getuid": {getuid, mruby.ArgsReq(1)},
	"getgid": {getgid, mruby.ArgsReq(1)},
	"read":   {read, mruby.ArgsReq(1)},
	"skip":   {skip, mruby.ArgsNone() | mruby.ArgsBlock()},
}

// importFunc implements the import function.
//
// import loads a new ruby file at the point of the function call. it is
// principally used to extend and consolidate reusable code for multiple
// styles of build.
func importFunc(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	if err := checkArgs(args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	content, err := ioutil.ReadFile(args[0].String())
	if err != nil {
		return nil, createException(m, err.Error())
	}

	result := b.RunScript(string(content))
	if result.Err != nil {
		return nil, createException(m, result.Err.Error())
	}

	return result.Value, nil
}

// getenv retrieves a value from the building environment (passed in as string)
// and returns a string with the value. If no value exists, an empty string is
// returned.
func getenv(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	return mruby.String(os.Getenv(args[0].String())), nil
}

func read(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	content, err := b.exec.CopyOneFileFromContainer(args[0].String())
	if err != nil {
		return nil, createException(m, err.Error())
	}

	return mruby.String(string(content)), nil
}

func getuid(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	content, err := b.exec.CopyOneFileFromContainer("/etc/passwd")
	if err != nil {
		return nil, createException(m, err.Error())
	}

	user := args[0].String()

	entries := strings.Split(string(content), "\n")
	for _, ent := range entries {
		parts := strings.Split(ent, ":")
		if parts[0] == user {
			return mruby.String(parts[2]), nil
		}
	}

	return nil, createException(m, fmt.Sprintf("Could not find user %q", user))
}

func getgid(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	content, err := b.exec.CopyOneFileFromContainer("/etc/group")
	if err != nil {
		return nil, createException(m, err.Error())
	}

	group := args[0].String()
	entries := strings.Split(string(content), "\n")
	for _, ent := range entries {
		parts := strings.Split(ent, ":")
		if parts[0] == group {
			return mruby.String(parts[2]), nil
		}
	}

	return nil, createException(m, fmt.Sprintf("Could not find group %q", group))
}

func skip(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	if err := standardCheck(b, args, 1); err != nil {
		return nil, createException(m, err.Error())
	}

	b.exec.SetSkipLayers(true)
	_, err := m.Yield(args[0])
	b.exec.SetSkipLayers(false)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}
