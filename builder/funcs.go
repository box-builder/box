package builder

/*
  funcs.go provides functions for use within box build operations that do *not*
  commit a layer or otherwise directly influence the build. They are intended to
  be used as gathering functions for predicates and templating.
*/

import (
	"fmt"
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
	"getenv": {getenv, mruby.ArgsReq(1)},
	"getuid": {getuid, mruby.ArgsReq(1)},
	"getgid": {getgid, mruby.ArgsReq(1)},
	"read":   {read, mruby.ArgsReq(1)},
}

// getenv retrieves a value from the building environment (passed in as string)
// and returns a string with the value. If no value exists, an empty string is
// returned.
func getenv(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	if len(args) != 1 {
		fmt.Printf("Invalid arg count in getenv: %d, must be 1", len(args))
		os.Exit(1)
	}

	return mruby.String(os.Getenv(args[0].String())), nil
}

// read takes a filename as string, reads it from the latest image in the
// evaluation and returns its data. Yields an error if the file does not exist
// or from has not been called.
func read(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if len(args) != 1 {
		return nil, createException(m, fmt.Sprintf("Expected 1 arg, got %d", len(args)))
	}

	if b.ImageID() == "" {
		return nil, createException(m, "from has not been called, no image can be used to get the UID")
	}

	content, err := b.containerContent(args[0].String())
	if err != nil {
		return nil, createException(m, err.Error())
	}

	return mruby.String(string(content)), nil
}

// getuid, given a string username provides an integer response with the UID of
// the user. This works by reading the /etc/passwd file in the image.
//
// Yields an error if it cannot find the user or from has not been called.
func getuid(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if len(args) != 1 {
		return nil, createException(m, fmt.Sprintf("Expected 1 arg, got %d", len(args)))
	}

	if b.ImageID() == "" {
		return nil, createException(m, "from has not been called, no image can be used to get the UID")
	}

	content, err := b.containerContent("/etc/passwd")
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

// getgid, given a string group name provides an integer response with the GID
// of the group. This works by reading the /etc/group file in the image.
//
// Yields an error if it cannot find the group or from has not been called.
func getgid(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if len(args) != 1 {
		return nil, createException(m, fmt.Sprintf("Expected 1 arg, got %d", len(args)))
	}

	if b.ImageID() == "" {
		return nil, createException(m, "from has not been called, no image can be used to get the UID")
	}

	content, err := b.containerContent("/etc/group")
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
