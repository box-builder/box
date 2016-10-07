package main

import "context"

func (b *Builder) commit() (string, error) {
	resp, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}
