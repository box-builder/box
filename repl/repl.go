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

// Repl encapsulates a series of items used to create a read-evaluate-print
// loop so that end users can manually enter build instructions.
type Repl struct {
	readline *readline.Instance
	builder  *builder.Builder
}

// NewRepl constructs a new Repl.
func NewRepl() (*Repl, error) {
	rl, err := readline.New("box> ")
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
		if recover() != nil {
			// interpreter signal or other badness, just abort.
			os.Exit(0)
		}
	}()

	var line string
	for {
		tmp, err := r.readline.Readline()
		if err == io.EOF {
			return nil
		}

		if err != nil && err.Error() == "Interrupt" {
			fmt.Println("You can press ^D or type \"quit\", \"exit\" to exit the shell")
			line = ""
			continue
		}

		if err != nil {
			fmt.Printf("+++ Error %#v\n", err)
			os.Exit(1)
		}

		line += tmp

		switch strings.TrimSpace(line) {
		case "quit":
			fallthrough
		case "exit":
			os.Exit(0)
		}

		p := mruby.NewParser(mruby.NewMrb())
		if _, err := p.Parse(line, nil); err != nil {
			continue
		}

		val, err := r.builder.Run(line)
		line = ""
		if err != nil {
			fmt.Printf("+++ Error: %v\n", err)
			continue
		}

		fmt.Println(val)
	}
}
