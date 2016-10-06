package main

import (
	"context"
	"fmt"

	mruby "github.com/mitchellh/go-mruby"
)

func entrypoint(b *Builder, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	stringArgs := []string{}
	for _, arg := range m.GetArgs() {
		stringArgs = append(stringArgs, arg.String())
	}

	b.config.Entrypoint = stringArgs

	resp, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return mruby.String(fmt.Sprintf("Error creating intermediate container: %v", err)), nil
	}

	b.id = resp.ID

	return nil, nil
}
