package evaluator

import (
	"github.com/box-builder/box/types"
)

// Evaluator is a generic language evaluator.
type Evaluator interface {
	Result() types.BuildResult
	RunCode(string, int, bool) (int, error)
	RunScript(string) error
	Close() error
}
