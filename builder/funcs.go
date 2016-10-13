package builder

/*
  funcs.go provides functions for use within box build operations that do *not*
  commit a layer or otherwise directly influence the build. They are intended to
  be used as gathering functions for predicates and templating.
*/

import (
	"fmt"
	"os"

	mruby "github.com/mitchellh/go-mruby"
)

type mrubyDefinition struct {
	mrubyFunc mruby.Func
	argSpec   mruby.ArgSpec
}

// mrubyJumpTable is the dispatch instructions sent to the mruby interpreter at builder setup.
var mrubyJumpTable = map[string]mrubyDefinition{
	"getenv": {getenv, mruby.ArgsReq(1)},
}

// getenv retrieves a value from the environment (passed in as string) and
// returns a string with the value. If no value exists, an empty string is
// returned.
func getenv(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	if len(args) != 1 {
		fmt.Printf("Invalid arg count in getenv: %d, must be 1", len(args))
		os.Exit(1)
	}

	return mruby.String(os.Getenv(args[0].String())), nil
}
