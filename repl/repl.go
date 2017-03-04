package repl

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/erikh/box/builder"
	"github.com/erikh/box/signal"
	mruby "github.com/mitchellh/go-mruby"
)

const (
	normalPrompt    = "box> "
	multilinePrompt = "box*> "
)

// Repl encapsulates a series of items used to create a read-evaluate-print
// loop so that end users can manually enter build instructions.
type Repl struct {
	readline *readline.Instance
	builder  *builder.Builder
}

// NewRepl constructs a new Repl.
func NewRepl(omit []string) (*Repl, error) {
	rl, err := readline.New(normalPrompt)
	if err != nil {
		return nil, err
	}

	signal.Handler.Exit = false
	signal.Handler.IgnoreRunners = true
	ctx, cancel := context.WithCancel(context.Background())

	b, err := builder.NewBuilder(builder.BuildConfig{
		OmitFuncs: omit,
		TTY:       true,
		Cache:     false,
		Context:   ctx,
		ShowRun:   true,
	})

	if err != nil {
		cancel()
		rl.Close()
		return nil, err
	}

	signal.Handler.AddFunc(cancel)

	return &Repl{readline: rl, builder: b}, nil
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
	var result builder.BuildResult

	p := mruby.NewParser(r.builder.Mrb())
	compileContext := mruby.NewCompileContext(r.builder.Mrb())
	compileContext.CaptureErrors(true)

	for {
		tmp, err := r.readline.Readline()
		if err == io.EOF {
			return nil
		}

		if _, ok := err.(*readline.InterruptError); ok || err != nil {
			if line != "" {
				r.readline.SetPrompt(normalPrompt)
			} else {
				fmt.Println("You can press ^D or type \"quit\", \"exit\" to exit the shell")
			}

			line = ""
			continue
		}

		if err != nil {
			fmt.Printf("+++ Error %#v\n", err)
			os.Exit(1)
		}

		line += tmp + "\n"

		switch strings.TrimSpace(line) {
		case "quit":
			fallthrough
		case "exit":
			os.Exit(0)
		}

		if _, err := p.Parse(line, compileContext); err != nil {
			r.readline.SetPrompt(multilinePrompt)
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		r.builder.SetContext(ctx)
		signal.Handler.AddFunc(cancel)

		result, stackKeep = r.builder.RunCode(p.GenerateCode(), stackKeep)
		line = ""
		r.readline.SetPrompt(normalPrompt)
		if result.Err != nil {
			fmt.Printf("+++ Error: %v\n", result.Err)
			continue
		}

		if result.Value.String() != "" {
			fmt.Println(result.Value)
		}
	}
}
