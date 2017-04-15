package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
)

func (d *Docker) stdinCopy(conn net.Conn, errChan chan error) (io.WriteCloser, *term.State) {
	if d.stdin {
		state, err := term.SetRawTerminal(0)
		if err != nil {
			errChan <- fmt.Errorf("Could not attach terminal to container: %v", err)
			return nil, nil
		}

		r, w := io.Pipe()
		go io.Copy(w, os.Stdin)
		go doCopy(conn, r, errChan)
		return w, state
	}

	return nil, nil
}

func (d *Docker) handleRunError(ctx context.Context, id string, errChan chan error) {
	select {
	case <-ctx.Done():
		if ctx.Err() != nil {
			d.globals.Logger.Error(ctx.Err())
		}
		d.Destroy(id)
	case err, ok := <-errChan:
		if ok {
			d.globals.Logger.Error(err)
			d.Destroy(id)
		}
	}
}

// RunHook is the run hook for docker agents.
func (d *Docker) RunHook(ctx context.Context, id string) (string, error) {
	errChan := make(chan error, 1)
	defer close(errChan)

	go d.handleRunError(ctx, id, errChan)

	cearesp, err := d.client.ContainerAttach(ctx, id, types.ContainerAttachOptions{Stream: true, Stdin: d.stdin, Stdout: true, Stderr: true})
	if err != nil {
		return "", fmt.Errorf("Could not attach to container: %v", err)
	}
	defer cearesp.Close()

	w, state := d.stdinCopy(cearesp.Conn, errChan)
	if w != nil {
		defer w.Close()
	}

	if state != nil {
		defer term.RestoreTerminal(0, state)
	}

	stat, err := d.startAndWait(ctx, id, cearesp.Conn, errChan)
	if err != nil {
		return "", err
	}

	if stat != 0 {
		return "", fmt.Errorf("Command exited with status %d for container %q", stat, id)
	}

	return "", nil
}

func doCopy(wtr io.Writer, rdr io.Reader, errChan chan error) {
repeat:
	_, err := io.Copy(wtr, rdr)

	if _, ok := err.(*net.OpError); ok { // EINTR basically
		goto repeat
	} else if err != nil {
		select {
		case errChan <- err:
		default:
		}
	}
}

func (d *Docker) startAndWait(ctx context.Context, id string, reader io.Reader, errChan chan error) (int, error) {
	err := d.client.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return -1, fmt.Errorf("Could not start container: %v", err)
	}

	var writer io.Writer = os.Stdout

	if !d.stdin && d.globals.ShowRun {
		d.globals.Logger.BeginOutput()
		defer d.globals.Logger.EndOutput()
	}

	var buf *bytes.Buffer

	if !d.globals.ShowRun {
		buf = bytes.NewBuffer([]byte{})
		reader = io.TeeReader(reader, buf)
		writer = ioutil.Discard
	}

	if !d.globals.TTY {
		go func() {
			// docker mux's the streams, and requires this stdcopy library to unpack them.
			_, err = stdcopy.StdCopy(writer, writer, reader)
			if err != nil && err != io.EOF {
				select {
				default:
					errChan <- err
				}
			}
		}()
	} else {
		go doCopy(writer, reader, errChan)
	}

	stat, err := d.client.ContainerWait(ctx, id)
	if err != nil {
		if buf != nil {
			fmt.Println(buf)
		}
		return -1, err
	}

	return int(stat), nil
}
