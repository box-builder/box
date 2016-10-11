package builder

import (
	"fmt"
	"os"

	mruby "github.com/mitchellh/go-mruby"
)

type mrubyDefinition struct {
	mrubyFunc mruby.Func
	argSpec   mruby.ArgSpec
}

var mrubyJumpTable = map[string]mrubyDefinition{
	"getenv": {getenv, mruby.ArgsReq(1)},
}

func getenv(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	if len(args) != 1 {
		fmt.Printf("Invalid arg count in getenv: %d, must be 1", len(args))
		os.Exit(1)
	}

	return mruby.String(os.Getenv(args[0].String())), nil
}
