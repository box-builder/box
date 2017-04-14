package repl

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	gm "github.com/mitchellh/go-mruby"

	"github.com/box-builder/box/builder/command"
	"github.com/box-builder/box/builder/evaluator"
	"github.com/box-builder/box/builder/evaluator/mruby"
	"github.com/box-builder/box/builder/executor/docker"
	"github.com/box-builder/box/logger"
	"github.com/box-builder/box/signal"
	"github.com/box-builder/box/types"
	"github.com/chzyer/readline"
	"github.com/docker/docker/pkg/term"
	"github.com/fatih/color"
)

const (
	normalPrompt    = "box> "
	multilinePrompt = "box*> "
)

// Repl encapsulates a series of items used to create a read-evaluate-print
// loop so that end users can manually enter build instructions.
type Repl struct {
	readline  *readline.Instance
	evaluator evaluator.Evaluator
	globals   *types.Global
}

// NewRepl contypes a new Repl.
func NewRepl(omit []string, log *logger.Logger) (*Repl, error) {
	rl, err := readline.New(normalPrompt)
	if err != nil {
		return nil, err
	}

	signal.Handler.Exit = false
	signal.Handler.IgnoreRunners = true
	ctx, cancel := context.WithCancel(context.Background())
	globals := &types.Global{
		OmitFuncs: omit,
		TTY:       term.IsTerminal(1),
		Color:     true,
		Cache:     false,
		ShowRun:   true,
		Logger:    log,
		Context:   ctx,
	}

	color.NoColor = false // force color on

	exec, err := docker.NewDocker(globals)
	if err != nil {
		cancel()
		rl.Close()
		return nil, err
	}

	e, err := mruby.NewMRuby(&mruby.Config{
		Filename: "repl",
		Globals:  globals,
		Interp:   command.NewInterpreter(globals, exec),
		Exec:     exec,
	})
	if err != nil {
		cancel()
		rl.Close()
		return nil, err
	}

	signal.Handler.AddFunc(cancel)

	return &Repl{readline: rl, evaluator: e, globals: globals}, nil
}

// Loop runs the loop. Returns nil on io.EOF, otherwise errors are forwarded.
func (r *Repl) Loop() error {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Aborting due to interpreter error: %v\n", err)
			os.Exit(2)
		}
	}()

	var line string
	var stackKeep int

	for {
		tmp, err := r.readline.Readline()
		if err == io.EOF {
			return nil
		}

		if _, ok := err.(*readline.InterruptError); ok || err != nil {
			if line != "" {
				r.readline.SetPrompt(normalPrompt)
			} else {
				fmt.Println("You can press ^D or type \"quit\" or \"exit\" to exit the shell")
			}

			line = ""
			continue
		}

		if err != nil {
			fmt.Printf("+++ Error %#v\n", err)
			os.Exit(1)
		}

		line += tmp + "\n"

		checkQuit(line)

		ctx, cancel := context.WithCancel(context.Background())
		r.globals.Context = ctx
		signal.Handler.AddFunc(cancel)

		newKeep, err := r.evaluator.RunCode(line, stackKeep)
		if err != nil {
			switch err.(type) {
			case gm.ParserError:
				if newKeep == stackKeep {
					r.readline.SetPrompt(multilinePrompt)
					continue
				}
			default:
				goto reset
			}
		}

		stackKeep = newKeep

	reset:
		line = ""
		r.readline.SetPrompt(normalPrompt)
		if err != nil {
			fmt.Printf("+++ Error: %v\n", err)
			continue
		}

		if r.evaluator.Result().Value != "" {
			fmt.Println(r.evaluator.Result().Value)
		}
	}
}

func checkQuit(line string) {
	switch strings.TrimSpace(line) {
	case "quit":
		fallthrough
	case "exit":
		os.Exit(0)
	}
}
