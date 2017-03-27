package types

import (
	"context"

	"github.com/box-builder/box/logger"
)

// BuildResult is an bunch of stuff that communicates a build result.
type BuildResult struct {
	FileName string
	Value    string
	Err      error
}

// Global represents global variables for the processing of an entire box run.
type Global struct {
	Cache     bool
	TTY       bool
	ShowRun   bool
	OmitFuncs []string
	Logger    *logger.Logger
	Context   context.Context
}
