package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/erikh/box/builder"
	"github.com/kless/term"
	"github.com/urfave/cli"
)

var (
	// Version is the version of the application
	Version = "0.1"
	// Name is the name of the application
	Name = "box"
	// Email is my email
	Email = "github@hollensbe.org"
	// Usage is the title of the application
	Usage = "Flexible Docker Builder"
	// Author is me
	Author = "Erik Hollensbe"

	// Copyright is the copyright, generated automatically for each year.
	Copyright = fmt.Sprintf("(C) %d %s - Licensed under MIT license", time.Now().Year(), Author)
	// UsageText is the description of how to use the program.
	UsageText = "box [options] filename (if omitted, pass the file contents into stdin)"
)

func main() {
	app := cli.NewApp()

	app.Name = Name
	app.Email = Email
	app.Version = Version
	app.Usage = Usage
	app.Author = Author
	app.Copyright = Copyright
	app.UsageText = UsageText
	app.HideHelp = true
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "no-cache, n",
			Usage: "Disable the build cache",
		},
		cli.BoolFlag{
			Name:  "no-tty",
			Usage: "Disable TTY features this run",
		},
		cli.BoolFlag{
			Name:  "force-tty",
			Usage: "Force TTY features this run",
		},
		cli.BoolFlag{
			Name:  "help, h",
			Usage: "Show the help",
		},
	}

	app.Action = func(ctx *cli.Context) {
		if ctx.Bool("help") {
			cli.ShowAppHelp(ctx)
			os.Exit(0)
		}

		args := ctx.Args()

		tty := !ctx.Bool("no-tty")

		if !term.IsTerminal(term.InputFD) {
			tty = ctx.Bool("force-tty")
		}

		b, err := builder.NewBuilder(tty)
		if err != nil {
			panic(err)
		}
		defer b.Close()

		var content []byte

		if len(args) == 1 {
			content, err = ioutil.ReadFile(args[0])
		} else {
			content, err = ioutil.ReadAll(os.Stdin)
		}
		if err != nil {
			fmt.Printf("!!! Error: %v\n", err.Error())
			os.Exit(2)
		}

		if ctx.Bool("no-cache") {
			b.SetCache(false)
		}

		response, err := b.Run(string(content))
		if err != nil {
			fmt.Printf("!!! Error: %v\n", err.Error())
			os.Exit(1)
		}

		if response.String() != "" {
			fmt.Printf("+++ Eval Response: %v\n", response)
		}

		id := b.ImageID()

		if strings.Contains(id, ":") {
			id = strings.SplitN(id, ":", 2)[1]
		}

		fmt.Printf("+++ Finish: %v\n", id)
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "!!! Error: %v\n", err)
		os.Exit(1)
	}
}
