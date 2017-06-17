package fetcher

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/box-builder/box/builder/config"
	"github.com/box-builder/box/logger"
	"github.com/box-builder/box/pull"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
)

// Docker does stuff
func Docker(context context.Context, logger *logger.Logger, client *client.Client, tty bool, config *config.Config, name string) (string, []string, error) {
	inspect, _, err := client.ImageInspectWithRaw(context, name)
	if err != nil {
		reader, err := client.ImagePull(context, name, types.ImagePullOptions{})
		if err != nil {
			return "", nil, err
		}

		if !tty {
			logger.Print(fmt.Sprintf("Pulling %q... ", name))

			if _, err := io.Copy(ioutil.Discard, reader); err != io.EOF && err != nil {
				return "", nil, err
			}

			fmt.Fprintln(logger.Output(), "done.")
		} else {
			pull.NewProgress(tty, reader).Process()
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

	config.FromDocker(false, inspect.Config)
	config.Image = inspect.ID

	return inspect.ID, inspect.RootFS.Layers, nil
}
