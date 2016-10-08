package main

import (
	"context"
	"fmt"

	"github.com/docker/engine-api/types"
	mruby "github.com/mitchellh/go-mruby"
)

func (b *Builder) commit() error {
	b.config.Image = b.imageID

	resp, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return err
	}

	commitResp, err := b.client.ContainerCommit(context.Background(), resp.ID, types.ContainerCommitOptions{Config: b.config})
	if err != nil {
		return fmt.Errorf("Error during commit: %v", err)
	}

	err = b.client.ContainerRemove(context.Background(), resp.ID, types.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("Could not remove intermediate container %q: %v", b.id, err)
	}

	b.id = resp.ID
	b.imageID = commitResp.ID

	return nil
}

func createException(m *mruby.Mrb, msg string) mruby.Value {
	val, err := m.Class("Exception", nil).New(mruby.String(msg))
	if err != nil {
		panic(fmt.Sprintf("could not construct exception for return: %v", err))
	}

	return val
}
