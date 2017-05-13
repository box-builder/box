// Package command - command for the box builder
//
// Documentation is deliberately simple here, it maps directly to the verbs and
// functions in the box builder documentation:
// https://box-builder.github.io/box. Duplicating it here seems pointless.
//
package command

import (
	"github.com/box-builder/box/builder/executor"
	"github.com/box-builder/box/types"
)

// Interpreter is a set of statements combined with an executor used to compose
// images. It is driven by an evaluator.
type Interpreter struct {
	CacheKey string // if set to "", does not consider cache next step
	globals  *types.Global
	exec     executor.Executor
	vars     map[string]string
}

// NewInterpreter contypes a new *Interpreter.
func NewInterpreter(globals *types.Global, exec executor.Executor, vars map[string]string) *Interpreter {
	return &Interpreter{
		globals: globals,
		exec:    exec,
		vars:    vars,
	}
}

func (i *Interpreter) makeLayer(useHook bool) error {
	hook := i.exec.RunHook
	if !useHook {
		hook = nil
	}

	return i.exec.Commit(i.CacheKey, hook)
}
