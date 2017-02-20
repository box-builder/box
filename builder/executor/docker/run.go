package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
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

func (d *Docker) handleRunError(ctx context.Context, id string, errChan chan error) {
	select {
	case <-ctx.Done():
		if ctx.Err() != nil {
			d.logger.Error(ctx.Err())
		}
		d.Destroy(id)
	case err, ok := <-errChan:
		if ok {
			d.logger.Error(err)
			d.Destroy(id)
		}
	}
}

// RunHook is the run hook for docker agents.
func (d *Docker) RunHook(ctx context.Context, id string) (string, error) {
	stopChan := make(chan struct{})
	errChan := make(chan error, 1)
	defer close(errChan)

	go d.handleRunError(ctx, id, errChan)

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
		} else if err != nil {
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
	defer close(stopChan)

	err := d.client.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return -1, fmt.Errorf("Could not start container: %v", err)
	}

	var writer io.Writer = os.Stdout

	if !d.stdin && d.showRun {
		d.logger.BeginOutput()
		defer d.logger.EndOutput()
	}

	if !d.showRun {
		writer = bytes.NewBuffer([]byte{})
	}

	if !d.tty {
		go func() {
			// docker mux's the streams, and requires this stdcopy library to unpack them.
			_, err = stdcopy.StdCopy(writer, writer, reader)
			if err != nil && err != io.EOF {
				select {
				case <-stopChan:
				default:
					errChan <- err
				}
			}
		}()
	} else {
		go doCopy(writer, reader, errChan, stopChan)
	}

	stat, err := d.client.ContainerWait(ctx, id)
	if err != nil {
		if wbuf, ok := writer.(*bytes.Buffer); ok {
			fmt.Print(wbuf)
		}
		return -1, err
	}

	return int(stat), nil
}
