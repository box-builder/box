package global

import "github.com/box-builder/box/logger"

// Global represents global variables for the processing of an entire box run.
type Global struct {
	Cache     bool
	TTY       bool // controls terminal codes
	ShowRun   bool
	OmitFuncs []string
	Logger    *logger.Logger
}
