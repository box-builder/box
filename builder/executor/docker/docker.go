package docker

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/box-builder/box/builder/config"
	"github.com/box-builder/box/builder/executor"
	"github.com/box-builder/box/layers"
	btypes "github.com/box-builder/box/types"
	"github.com/box-builder/box/util"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// Docker implements an executor that talks to docker to achieve its goals.
type Docker struct {
	globals *btypes.Global
	client  *client.Client
	config  *config.Config
	stdin   bool
	layers  layers.Layers
	image   layers.Image
}

// NewDocker contypes a new docker instance, for executing against docker
// engines.
func NewDocker(globals *btypes.Global) (*Docker, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	config := config.NewConfig()

	l, err := layers.NewDocker(globals)
	if err != nil {
		return nil, err
	}

	i, err := layers.NewDockerImage(&layers.ImageConfig{
		Config:  config,
		Layers:  l,
		Globals: globals,
	})
	if err != nil {
		return nil, err
	}

	return &Docker{
		globals: globals,
		client:  client,
		config:  config,
		layers:  l,
		image:   i,
	}, nil
}

// SetStdin turns on the stdin features during run invocations. It is used to
// facilitate debugging.
func (d *Docker) SetStdin(on bool) {
	d.stdin = on
}

// Image returns the layers.Image interface for working with Docker
func (d *Docker) Image() layers.Image {
	return d.image
}

// Layers returns the layers.Layers interface for working with Docker
func (d *Docker) Layers() layers.Layers {
	return d.layers
}

// LoadConfig loads the configuration into the executor.
func (d *Docker) LoadConfig(c *config.Config) error {
	d.config = c
	return nil
}

// Config returns the current *Config for the executor.
func (d *Docker) Config() *config.Config {
	return d.config
}

// Commit commits an entry to the layer list.
func (d *Docker) Commit(cacheKey string, hook executor.Hook) error {
	if err := util.CheckContext(d.globals.Context); err != nil {
		return err
	}

	id, err := d.Create()
	if err != nil {
		return err
	}

	defer d.Destroy(id)

	if hook != nil {
		// FIXME this cache key handling is terrible.
		tmp, err := hook(d.globals.Context, id)
		if err != nil {
			return err
		}

		if tmp != "" {
			cacheKey = tmp
		}
	}

	select {
	case <-d.globals.Context.Done():
		if d.globals.Context.Err() == context.Canceled {
			return d.globals.Context.Err()
		}
	default:
	}

	if err := util.CheckContext(d.globals.Context); err != nil {
		return err
	}

	commitResp, err := d.client.ContainerCommit(d.globals.Context, id, types.ContainerCommitOptions{Config: d.config.ToDocker(false, d.globals.TTY, d.stdin), Comment: cacheKey})
	if err != nil {
		return fmt.Errorf("Error during commit: %v", err)
	}

	// try a clean remove first, otherwise the defer above will take over in a last-ditch attempt
	err = d.client.ContainerRemove(d.globals.Context, id, types.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("Could not remove intermediate container %q: %v", id, err)
	}

	d.config.Image = commitResp.ID
	return d.Layers().AddImage(commitResp.ID)
}

// CopyOneFileFromContainer copies a file from the container and returns its content.
// An error is returned, if any.
func (d *Docker) CopyOneFileFromContainer(fn string) ([]byte, error) {
	id, err := d.Create()
	if err != nil {
		return nil, err
	}

	defer d.Destroy(id)

	rc, _, err := d.client.CopyFromContainer(d.globals.Context, id, fn)
	if err != nil {
		return nil, err
	}

	tr := tar.NewReader(rc)
	defer rc.Close()

	var header *tar.Header

	for {
		header, err = tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if header.Name == filepath.Base(fn) {
			break
		}
	}

	if header == nil || header.Name != filepath.Base(fn) {
		return nil, fmt.Errorf("Could not find %q in container", fn)
	}

	return ioutil.ReadAll(tr)
}

// Create creates a new container based on the existing configuration.
func (d *Docker) Create() (string, error) {
	cont, err := d.client.ContainerCreate(
		d.globals.Context,
		d.config.ToDocker(true, d.globals.TTY, d.stdin),
		nil,
		nil,
		"",
	)

	return cont.ID, err
}

// Destroy destroys a container for the given id.
func (d *Docker) Destroy(id string) error {
	// XXX do not use the stored context because it may already be canceled when we arrive at this code.
	return d.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{Force: true})
}

// CopyFromContainer copies a series of files in a similar fashion to
// CopyToContainer, just in reverse.
func (d *Docker) CopyFromContainer(id, path string) (io.Reader, int64, error) {
	rc, stat, err := d.client.CopyFromContainer(d.globals.Context, id, path)
	return rc, stat.Size, err
}

// CopyToContainer copies files from the tarfile specified in reader to the
// containerto the container so it can then be committed. It does not close the
// reader.
func (d *Docker) CopyToContainer(id string, r io.Reader) error {
	return d.client.CopyToContainer(d.globals.Context, id, "/", r, types.CopyToContainerOptions{})
}
