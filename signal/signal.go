package signal

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Handler is the default registered signal handler. It is created when this
// package is initialized.
var Handler = NewCancellable()

func init() {
	signals := make(chan os.Signal, 1)
	go Handler.SignalHandler(signals)

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
}

// Cancellable is a cancellable process triggered via signal. It will cascade
// through the context's cancel functions destroying each build process as a
// result.
type Cancellable struct {
	Exit          bool
	IgnoreRunners bool

	mutex       *sync.Mutex
	files       map[string]struct{}
	cancelFuncs []context.CancelFunc
	runners     []chan struct{}
}

// NewCancellable creates a cancellable process
func NewCancellable() *Cancellable {
	return &Cancellable{
		Exit:        true,
		files:       map[string]struct{}{},
		mutex:       new(sync.Mutex),
		cancelFuncs: []context.CancelFunc{},
		runners:     make([]chan struct{}, 0),
	}
}

// AddFile adds a temporary filename to be reaped if the action is canceled.
func (c *Cancellable) AddFile(filename string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.files[filename] = struct{}{}
}

// RemoveFile removes a file from the temporary file list.
func (c *Cancellable) RemoveFile(filename string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.files, filename)
}

// AddFunc adds a cancel func to the list.
func (c *Cancellable) AddFunc(f context.CancelFunc) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cancelFuncs = append(c.cancelFuncs, f)
}

// AddRunner adds a chan struct{} to the list of runners. See BuildConfig for more.
func (c *Cancellable) AddRunner(run chan struct{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.runners = append(c.runners, run)
}

// SignalHandler is the signal handler that will be used throughout box to
// cancel things.
func (c *Cancellable) SignalHandler(signals chan os.Signal) {
	for {
		<-signals

		c.mutex.Lock()
		fmt.Println("\n\n!!! SIGINT or SIGTERM received, crashing containers...")
		for _, cancel := range c.cancelFuncs {
			cancel()
		}

		if !c.IgnoreRunners {
			for _, runner := range c.runners {
				<-runner
			}
		}

		for fn := range c.files {
			fmt.Fprintf(os.Stderr, "Cleaning up temporary file %q", fn)

			if err := os.Remove(fn); err != nil {
				fmt.Fprintf(os.Stderr, ": %v", err)
			}

			fmt.Fprintln(os.Stderr)
		}

		if c.Exit {
			os.Exit(1)
		}
		c.mutex.Unlock()
	}
}
