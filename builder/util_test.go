package builder

import (
	"bufio"
	"bytes"
	"context"
	"io"

	. "gopkg.in/check.v1"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/api/types"
)

func runBuilder(script string) (*Builder, error) {
	b, err := NewBuilder(term.IsTerminal(0), []string{})
	if err != nil {
		return nil, err
	}

	_, err = b.Run(script)
	return b, err
}

func readContainerFile(c *C, b *Builder, fn string) []byte {
	return runContainerCommand(c, b, []string{"cat", fn})
}

func runContainerCommand(c *C, b *Builder, cmd []string) []byte {
	b.exec.Config().Entrypoint = []string{}
	b.exec.Config().Cmd = cmd
	id, err := b.exec.Create()
	c.Assert(err, IsNil)
	resp, err := dockerClient.ContainerAttach(context.Background(), id, types.ContainerAttachOptions{Stream: true, Stdout: true, Stdin: true})
	c.Assert(err, IsNil)

	err = dockerClient.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
	c.Assert(err, IsNil)

	buf := new(bytes.Buffer)

	var n int64

	if term.IsTerminal(0) {
		n, err = io.Copy(buf, resp.Reader)
	} else {
		n, err = stdcopy.StdCopy(buf, buf, resp.Reader)
	}

	c.Assert(err, IsNil)
	c.Assert(n, Not(Equals), 0)

	nr := bufio.NewReader(buf)
	result := []byte{}

	for err == nil {
		var inner []byte
		inner, err = nr.ReadBytes('\n')
		if len(inner) >= 2 && inner[len(inner)-2] == '\r' {
			inner = append(inner[:len(inner)-2], '\n')
		}

		result = append(result, inner...)
	}

	status, err := dockerClient.ContainerWait(context.Background(), id)
	c.Assert(err, IsNil)
	c.Assert(status, Equals, 0)

	return result
}

func getParent(b *Builder, img string) (string, error) {
	inspect, _, err := dockerClient.ImageInspectWithRaw(context.Background(), img)
	return inspect.Parent, err
}
