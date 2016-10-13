package builder

import (
	"bufio"
	"bytes"
	"context"
	"io"

	. "gopkg.in/check.v1"

	"github.com/docker/engine-api/types"
)

func runBuilder(script string) (*Builder, error) {
	b, err := NewBuilder()
	if err != nil {
		return nil, err
	}

	_, err = b.Run(script)
	return b, err
}

func readContainerFile(c *C, b *Builder, fn string) []byte {
	return runContainerCommand(c, b, []string{"cat " + fn})
}

func runContainerCommand(c *C, b *Builder, cmd []string) []byte {
	b.config.Cmd = cmd
	id, err := b.createEmptyContainer()
	c.Assert(err, IsNil)
	resp, err := b.client.ContainerAttach(context.Background(), id, types.ContainerAttachOptions{Stream: true, Stdout: true, Stdin: true})
	c.Assert(err, IsNil)

	err = b.client.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
	c.Assert(err, IsNil)

	buf := new(bytes.Buffer)

	n, err := io.Copy(buf, resp.Reader)
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

	status, err := b.client.ContainerWait(context.Background(), id)
	c.Assert(err, IsNil)
	c.Assert(status, Equals, 0)

	return result
}
