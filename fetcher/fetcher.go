package fetcher

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/box-builder/box/builder/config"
	"github.com/box-builder/box/pull"
	btypes "github.com/box-builder/box/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// Docker does stuff
func Docker(context context.Context, globals *btypes.Global, client *client.Client, config *config.Config, name string) (string, []string, error) {
	if !strings.Contains(name, ":") {
		// if we don't have a sub-tag, we need to add :latest to avoid pulling the whole repo.
		name += ":latest"
	}

	inspect, _, err := client.ImageInspectWithRaw(context, name)
	if err != nil {
		reader, err := client.ImagePull(context, name, types.ImagePullOptions{})
		if err != nil {
			return "", nil, err
		}

		if !globals.TTY {
			globals.Logger.Print(fmt.Sprintf("Pulling %q... ", name))

			if _, err := io.Copy(ioutil.Discard, reader); err != io.EOF && err != nil {
				return "", nil, err
			}

			fmt.Fprintln(globals.Logger.Output(), "done.")
		} else {
			pull.NewProgress(globals.TTY, reader).Process()
		}

		select {
		case <-context.Done():
			if context.Err() != nil {
				return "", nil, context.Err()
			}
		default:
		}

		// this will fallthrough to the assignment below
		inspect, _, err = client.ImageInspectWithRaw(context, name)
		if err != nil {
			return "", nil, err
		}

		select {
		case <-context.Done():
			if context.Err() != nil {
				return "", nil, context.Err()
			}
		default:
		}
	}

	config.FromDocker(inspect.Config)
	config.Image = inspect.ID

	return inspect.ID, inspect.RootFS.Layers, nil
}
