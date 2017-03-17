package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/pkg/term"
	"github.com/erikh/box/builder"
	"github.com/erikh/box/copy"
	"github.com/erikh/box/logger"
	"github.com/erikh/box/multi"
	"github.com/erikh/box/repl"
	"github.com/erikh/box/signal"
	"github.com/urfave/cli"
)

var (
	// Version is the version of the application
	Version = "0.4.2"
	// Name is the name of the application
	Name = "box"
	// Email is my email
	Email = "github@hollensbe.org"
	// Usage is the title of the application
	Usage = "Advanced mruby Container Image Builder"
	// Author is me
	Author = "Erik Hollensbe"

	// Copyright is the copyright, generated automatically for each year.
	Copyright = fmt.Sprintf("(C) %d %s - Licensed under MIT license", time.Now().Year(), Author)
	// UsageText is the description of how to use the program.
	UsageText = "box [options] filename"
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
		cli.StringFlag{
			Name:  "tag, t",
			Usage: "Tag the last image with this name",
		},
		cli.StringSliceFlag{
			Name:  "omit, o",
			Usage: "Omit functions/verbs. One per option, repeatable.",
		},
		cli.BoolFlag{
			Name:  "no-trim",
			Usage: "Do not trim the output to terminal width.",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:        "multi",
			Action:      runMulti,
			Description: "Run the multi build functionality; supply multiple plans to build",
			Usage:       "Run the multi build functionality; supply multiple plans to build",
			ArgsUsage:   "[filename] [filename]",
		},
		{
			Name:        "repl",
			Action:      runRepl,
			Description: "Run the read-eval-print loop to interactively work with box",
			Usage:       "Run the read-eval-print loop to interactively work with box",
			ArgsUsage:   " ",
		},
		{
			Name:        "shell",
			Action:      runRepl,
			Description: "Run the read-eval-print loop to interactively work with box",
			Usage:       "Run the read-eval-print loop to interactively work with box",
			ArgsUsage:   " ",
		},
	}

	app.Action = func(ctx *cli.Context) {
		notrim := ctx.Bool("no-trim")
		log := logger.New("main", notrim)

		if ctx.Bool("help") {
			cli.ShowAppHelp(ctx)
			os.Exit(0)
		}

		args := ctx.Args()

		if len(args) < 1 {
			cli.ShowAppHelp(ctx)
			log.Error("Please provide a filename to process!")
			os.Exit(1)
		}

		log = logger.New(args[0], notrim)

		tty := !ctx.Bool("no-tty")

		if !term.IsTerminal(0) {
			tty = ctx.Bool("force-tty")
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		runChan := make(chan struct{})
		buildConfig := builder.BuildConfig{
			ShowRun:   true,
			TTY:       tty,
			OmitFuncs: ctx.GlobalStringSlice("omit"),
			Cache:     getCache(ctx),
			Context:   cancelCtx,
			Runner:    runChan,
			FileName:  args[0],
			Logger:    logger.New(args[0], notrim),
		}

		b, err := mkBuilder(cancel, buildConfig)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}

		defer b.Close()

		result := b.Run()
		if result.Err != nil {
			log.Error(result.Err)
			os.Exit(1)
		}

		if result.Value.String() != "" {
			log.EvalResponse(result.Value.String())
		}

		tag := ctx.String("tag")

		if tag != "" {
			if err := b.Tag(tag); err != nil {
				log.Error(fmt.Sprintf("Can't tag with tag %q: %v", tag, err))
				os.Exit(1)
			}
			log.Tag(tag)
		}

		id := b.ImageID()

		if strings.Contains(id, ":") {
			id = strings.SplitN(id, ":", 2)[1]
		}

		log.Finish(id)
	}

	if err := app.Run(os.Args); err != nil {
		logger.New("main", false).Error(err)
		os.Exit(1)
	}
}

func runMulti(ctx *cli.Context) {
	copy.NoOut = true
	notrim := ctx.Bool("no-trim")
	builders := []*builder.Builder{}
	log := logger.New("main", notrim)

	args := ctx.Args()
	if len(args) < 1 {
		cli.ShowAppHelp(ctx)
		log.Error("Please provide a filename to process!")
		os.Exit(1)
	}

	for _, filename := range args {
		cancelCtx, cancel := context.WithCancel(context.Background())
		runChan := make(chan struct{})
		buildConfig := builder.BuildConfig{
			ShowRun:   false,
			TTY:       true,
			OmitFuncs: append(ctx.StringSlice("omit"), "debug"),
			Cache:     getCache(ctx),
			Context:   cancelCtx,
			Runner:    runChan,
			FileName:  filename,
			Logger:    logger.New(filename, notrim),
		}
		signal.Handler.AddFunc(cancel)
		signal.Handler.AddRunner(runChan)

		b, err := builder.NewBuilder(buildConfig)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		builders = append(builders, b)
	}

	mb := multi.NewBuilder(builders)
	mb.Build()
	if err := mb.Wait(); err != nil {
		log.Error(err)
		os.Exit(2)
	}
}

func getCache(ctx *cli.Context) bool {
	cache := os.Getenv("NO_CACHE") == ""
	if ctx.GlobalBool("no-cache") {
		cache = false
	}

	return cache
}

func runRepl(ctx *cli.Context) {
	log := logger.New("repl", ctx.Bool("no-trim"))
	r, err := repl.NewRepl(ctx.GlobalStringSlice("omit"), log)
	if err != nil {
		log.Error(fmt.Sprintf("bootstrapping repl: %v\n", err))
		os.Exit(1)
	}

	if err := r.Loop(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func mkBuilder(cancel context.CancelFunc, buildConfig builder.BuildConfig) (*builder.Builder, error) {
	b, err := builder.NewBuilder(buildConfig)
	if err != nil {
		return nil, err
	}

	signal.Handler.AddFunc(cancel)
	signal.Handler.AddRunner(buildConfig.Runner)
	return b, nil
}
