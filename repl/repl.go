package repl

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/erikh/box/builder"
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
func NewRepl() (*Repl, error) {
	rl, err := readline.New(normalPrompt)
	if err != nil {
		return nil, err
	}

	b, err := builder.NewBuilder(true, []string{})
	if err != nil {
		rl.Close()
		return nil, err
	}

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
	var val *mruby.MrbValue

	p := mruby.NewParser(r.builder.Mrb())
	context := mruby.NewCompileContext(r.builder.Mrb())
	context.CaptureErrors(true)

	for {
		tmp, err := r.readline.Readline()
		if err == io.EOF {
			return nil
		}

		if err != nil && err.Error() == "Interrupt" {
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

		if _, err := p.Parse(line, context); err != nil {
			r.readline.SetPrompt(multilinePrompt)
			continue
		}

		val, stackKeep, err = r.builder.RunCode(p.GenerateCode(), stackKeep)
		line = ""
		r.readline.SetPrompt(normalPrompt)
		if err != nil {
			fmt.Printf("+++ Error: %v\n", err)
			continue
		}

		if val.String() != "" {
			fmt.Println(val)
		}
	}
}
