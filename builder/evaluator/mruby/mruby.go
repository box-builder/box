package mruby

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/box-builder/box/builder/command"
	"github.com/box-builder/box/builder/executor"
	"github.com/box-builder/box/types"
	gm "github.com/mitchellh/go-mruby"
)

// MRuby is an Evaluator that can handle mruby interpreters.
type MRuby struct {
	mrb            *gm.Mrb
	afterFunc      *gm.MrbValue
	parser         *gm.Parser
	compileContext *gm.CompileContext
	result         types.BuildResult
	*Config
}

// Config is the parameters for the mruby engine
type Config struct {
	Filename string
	Interp   *command.Interpreter
	Exec     executor.Executor
	Globals  *types.Global
}

// NewMRuby instantiates a *MRuby.
func NewMRuby(config *Config) (*MRuby, error) {
	m := &MRuby{
		mrb:    gm.NewMrb(),
		Config: config,
	}

	m.prepare()

	return m, nil
}

func (m *MRuby) wrapVerbFunc(name string, vd *verbDefinition) gm.Func {
	return func(mrb *gm.Mrb, self *gm.MrbValue) (gm.Value, gm.Value) {
		select {
		case <-m.Globals.Context.Done():
			if m.Globals.Context.Err() != nil {
				return nil, m.createException(m.Globals.Context.Err())
			}

			return nil, nil
		default:
		}

		args := mrb.GetArgs()
		strArgs := extractStringArgs(args)
		cacheKey := strings.Join(append([]string{name}, strArgs...), ", ")
		cacheKey = base64.StdEncoding.EncodeToString([]byte(cacheKey))

		m.Globals.Logger.BuildStep(name, strings.Join(strArgs, ", "))

		if os.Getenv("BOX_DEBUG") != "" {
			content, _ := json.MarshalIndent(m.Exec.Config(), "", "  ")
			fmt.Println(string(content))
		}

		cached, err := m.Exec.Image().CheckCache(cacheKey)
		if err != nil {
			return nil, m.createException(err)
		}

		m.Interp.CacheKey = cacheKey

		// if we don't do this for debug, we will step past it on successive runs
		if !cached || name == "debug" {
			return nil, m.createException(vd.verbFunc(args, self))
		}

		return nil, nil
	}
}

func (m *MRuby) wrapFuncFunc(name string, jump *funcDefinition) func(m *gm.Mrb, self *gm.MrbValue) (gm.Value, gm.Value) {
	return func(mrb *gm.Mrb, self *gm.MrbValue) (gm.Value, gm.Value) {
		return jump.fun(mrb.GetArgs(), self)
	}
}

func (m *MRuby) prepare() {
	for name, jump := range m.verbJumpTable() {
		var found bool
		for _, omit := range m.Globals.OmitFuncs {
			if omit == name {
				found = true
			}
		}
		if !found {
			m.mrb.TopSelf().SingletonClass().DefineMethod(name, m.wrapVerbFunc(name, jump), jump.argSpec)
		}
	}
	for name, jump := range m.funcJumpTable() {
		var found bool
		for _, omit := range m.Globals.OmitFuncs {
			if omit == name {
				found = true
			}
		}
		if !found {
			m.mrb.TopSelf().SingletonClass().DefineMethod(name, m.wrapFuncFunc(name, jump), jump.argSpec)
		}
	}
}

func (m *MRuby) makeError(err error) error {
	m.result = types.BuildResult{
		Err:      err,
		FileName: m.Filename,
	}

	return err
}

func (m *MRuby) makeResult(result string) error {
	m.result = types.BuildResult{
		Value:    result,
		FileName: m.Filename,
	}

	return nil
}

// Result returns the last BuildResult for this evaluator.
func (m *MRuby) Result() types.BuildResult {
	return m.result
}

// RunCode runs the value intended to be a *gm.MrbValue in the mruby
// instance which is a proc to some code to run, and the previous stack
// reference (or 0 for none). The result is both a BuildResult and an integer
// that the refers to a stack in the mruby interpreter.
//
// This is typically used for instantiating code in a REPL; so that it can be
// appropriately evaluated and on any evaluation error, return its position so
// the evaluation can continue.
//
// Given this function is intended to run multiple times, it does not execute
// the after hooks if they are set.
func (m *MRuby) RunCode(line string, stackKeep int, make bool) (int, error) {
	if m.compileContext == nil {
		m.compileContext = gm.NewCompileContext(m.mrb)
		m.compileContext.CaptureErrors(true)
	}

	if m.parser == nil {
		m.parser = gm.NewParser(m.mrb)
	}

	if _, err := m.parser.Parse(line, m.compileContext); err != nil {
		return stackKeep, m.makeError(err)
	}

	keep, res, err := m.mrb.RunWithContext(m.parser.GenerateCode(), m.mrb.TopSelf(), stackKeep)
	if err != nil {
		return keep, m.makeError(err)
	}

	if res != nil && res.String() != "" {
		return keep, m.makeResult(res.String())
	}

	if make {
		if _, err := m.Exec.Layers().MakeImage(m.Exec.Config()); err != nil {
			return keep, m.makeError(err)
		}
	}

	return keep, m.makeResult(m.Exec.Image().ImageID())
}

// RunScript runs the string provided. Returns a BuildResult
func (m *MRuby) RunScript(script string) error {
	if _, err := m.mrb.LoadString(script); err != nil {
		return m.makeError(err)
	}

	if _, err := m.Exec.Layers().MakeImage(m.Exec.Config()); err != nil {
		return m.makeError(err)
	}

	if m.afterFunc != nil {
		_, err := m.mrb.Yield(m.afterFunc)
		if err != nil {
			return m.makeError(err)
		}
	}

	return m.makeResult(m.Exec.Image().ImageID())
}

// Close the interpreter.
func (m *MRuby) Close() error {
	m.mrb.EnableGC()
	m.mrb.FullGC()
	m.mrb.Close()
	return nil
}
