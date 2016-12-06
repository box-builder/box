package docker

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/engine-api/types"
	"github.com/erikh/box/builder/signal"
	"github.com/erikh/box/log"
)

func (d *Docker) stdinCopy(conn net.Conn, errChan chan error, stopChan chan struct{}) {
	if d.stdin {
		state, err := term.SetRawTerminal(0)
		if err != nil {
			errChan <- fmt.Errorf("Could not attach terminal to container: %v", err)
		}

		defer term.RestoreTerminal(0, state)

		doCopy(conn, os.Stdin, errChan, stopChan)
	}
}

func (d *Docker) handleRunError(id string, errChan chan error, cancel context.CancelFunc) {
	err, ok := <-errChan
	if ok {
		fmt.Printf("\n\n+++ Run Error: %#v\n", err)
		cancel()
		d.Destroy(id)
	}
}

// RunHook is the run hook for docker agents.
func (d *Docker) RunHook(id string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	signal.SetSignal(func() {
		cancel()
		d.Destroy(id)
	})
	defer signal.SetSignal(nil)

	stopChan := make(chan struct{})

	errChan := make(chan error, 1)
	defer close(errChan)
	go d.handleRunError(id, errChan, cancel)

	cearesp, err := d.client.ContainerAttach(ctx, id, types.ContainerAttachOptions{Stream: true, Stdin: d.stdin, Stdout: true, Stderr: true})
	if err != nil {
		return "", fmt.Errorf("Could not attach to container: %v", err)
	}

	go d.stdinCopy(cearesp.Conn, errChan, stopChan)

	defer cearesp.Close()

	stat, err := d.startAndWait(ctx, id, cearesp.Reader, errChan, stopChan)
	if err != nil {
		return "", err
	}

	if stat != 0 {
		return "", fmt.Errorf("Command exited with status %d for container %q", stat, id)
	}

	return "", nil
}

func doCopy(wtr io.Writer, rdr io.Reader, errChan chan error, stopChan chan struct{}) {
	// repeat copy until error is returned. if error is not io.EOF, forward
	// to channel. Return on any error.
	for {
		select {
		case <-stopChan:
			return
		default:
		}

		if _, err := io.Copy(wtr, rdr); err == nil {
			continue
		} else if _, ok := err.(*net.OpError); ok {
			continue
		} else if err != io.EOF {
			select {
			case <-stopChan:
			case errChan <- err:
			default:
			}
		}

		return
	}
}

func (d *Docker) startAndWait(ctx context.Context, id string, reader io.Reader, errChan chan error, stopChan chan struct{}) (int, error) {
	err := d.client.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return -1, fmt.Errorf("Could not start container: %v", err)
	}

	if !d.stdin {
		log.BeginOutput()
	}

	if !d.tty {
		go func() {
			// docker mux's the streams, and requires this stdcopy library to unpack them.
			_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, reader)
			if err != nil && err != io.EOF {
				select {
				case <-stopChan:
				default:
					errChan <- err
				}
			}
		}()
	} else if d.tty {
		go doCopy(os.Stdout, reader, errChan, stopChan)
	}

	defer close(stopChan)

	stat, err := d.client.ContainerWait(ctx, id)
	if err != nil {
		return -1, err
	}

	if !d.stdin {
		log.EndOutput()
	}

	return stat, nil
}
