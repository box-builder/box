package docker

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/erikh/box/builder/config"
	"github.com/erikh/box/builder/executor"
	"github.com/erikh/box/builder/signal"
	"github.com/erikh/box/copy"
	"github.com/erikh/box/image"
	"github.com/erikh/box/log"
)

// Docker implements an executor that talks to docker to achieve its goals.
type Docker struct {
	client       *client.Client
	config       *config.Config
	from         string
	useCache     bool
	tty          bool
	stdin        bool
	layers       []string
	skipLayers   []string
	doSkipLayers bool
	layerSet     map[string]struct{}
}

// NewDocker constructs a new docker instance, for executing against docker
// engines.
func NewDocker(useCache, tty bool) (*Docker, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return &Docker{
		tty:        tty,
		useCache:   useCache,
		client:     client,
		config:     config.NewConfig(),
		layerSet:   map[string]struct{}{},
		layers:     []string{},
		skipLayers: []string{},
	}, nil
}

// addImage adds layers to the layer list from a provided image, in order of
// appearance. Any existing layers are skipped over, removing them from the list.
func (d *Docker) addImage(image string) error {
	d.config.Image = image

	resp, _, err := d.client.ImageInspectWithRaw(context.Background(), image)
	if err != nil {
		return err
	}

	for _, layer := range resp.RootFS.Layers {
		if _, ok := d.layerSet[layer]; !ok {
			if !d.doSkipLayers {
				// XXX this really worries me. Pretty sure there's a potential cache
				// fail/poison here, but I have to debug it.
				// BETA QUALITY YO
				d.layers = append(d.layers, layer)
			} else {
				// this is principally an optimization so we can determine later if we
				// need to reconstruct the image.
				d.skipLayers = append(d.skipLayers, layer)
			}

			d.layerSet[layer] = struct{}{}
		}
	}

	return nil
}

// SetSkipLayers toggles whether or not to skip layers when building the
// final image.
func (d *Docker) SetSkipLayers(ok bool) {
	d.doSkipLayers = ok
}

// SetStdin turns on the stdin features during run invocations. It is used to
// facilitate debugging.
func (d *Docker) SetStdin(on bool) {
	d.stdin = on
}

// ImageID returns the image identifier of the most recent layer.
func (d *Docker) ImageID() string {
	return d.config.Image
}

// UseCache determines if the cache should be considered or not.
func (d *Docker) UseCache(arg bool) {
	d.useCache = arg
}

// GetCache gets the current value of whether or not to use the cache
func (d *Docker) GetCache() bool {
	return d.useCache
}

// UseTTY determines whether or not to allow docker to use a TTY for both run
// and pull operations.
func (d *Docker) UseTTY(arg bool) {
	d.tty = arg
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
	id, err := d.Create()
	if err != nil {
		return err
	}

	signal.SetSignal(func() { d.Destroy(id) })
	defer signal.SetSignal(nil)
	defer d.Destroy(id)

	if hook != nil {
		// FIXME this cache key handling is terrible.
		tmp, err := hook(id)
		if err != nil {
			return err
		}

		if tmp != "" {
			cacheKey = tmp
		}
	}

	commitResp, err := d.client.ContainerCommit(context.Background(), id, types.ContainerCommitOptions{Config: d.config.ToDocker(false, d.tty, d.stdin), Comment: cacheKey})
	if err != nil {
		return fmt.Errorf("Error during commit: %v", err)
	}

	// try a clean remove first, otherwise the defer above will take over in a last-ditch attempt
	err = d.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("Could not remove intermediate container %q: %v", id, err)
	}

	return d.addImage(commitResp.ID)
}

// CheckCache consults the cache and returns true or false depending on whether
// there was a match. If there was an error consulting the cache, it will be
// returned as the second argument.
func (d *Docker) CheckCache(cacheKey string) (bool, error) {
	if !d.useCache {
		return false, nil
	}

	images, err := d.client.ImageList(context.Background(), types.ImageListOptions{All: true})
	if err != nil {
		return false, err
	}

	for _, img := range images {
		if (img.ParentID != "" && img.ParentID == d.config.Image) || img.ParentID == "" {
			inspect, _, err := d.client.ImageInspectWithRaw(context.Background(), img.ID)
			if err != nil {
				return false, err
			}

			if inspect.Comment == cacheKey {
				log.CacheHit(img.ID)
				d.config.FromDocker(inspect.Config)
				return true, d.addImage(img.ID)
			}
		}
	}

	return false, nil
}

// CopyOneFileFromContainer copies a file from the container and returns its content.
// An error is returned, if any.
func (d *Docker) CopyOneFileFromContainer(fn string) ([]byte, error) {
	id, err := d.Create()
	if err != nil {
		return nil, err
	}

	defer d.Destroy(id)

	rc, _, err := d.client.CopyFromContainer(context.Background(), id, fn)
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
		context.Background(),
		d.config.ToDocker(true, d.tty, d.stdin),
		nil,
		nil,
		"",
	)

	return cont.ID, err
}

// Destroy destroys a container for the given id.
func (d *Docker) Destroy(id string) error {
	return d.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{Force: true})
}

// CopyFromContainer copies a series of files in a similar fashion to
// CopyToContainer, just in reverse.
func (d *Docker) CopyFromContainer(id, path string) (io.Reader, int64, error) {
	rc, stat, err := d.client.CopyFromContainer(context.Background(), id, path)
	return rc, stat.Size, err
}

// CopyToContainer copies files from the tarfile specified in reader to the
// containerto the container so it can then be committed. It does not close the
// reader.
func (d *Docker) CopyToContainer(id string, r io.Reader) error {
	return d.client.CopyToContainer(context.Background(), id, "/", r, types.CopyToContainerOptions{})
}

// Flatten copies a tarred up series of files (passed in through the
// io.Reader handle) to the image where they are untarred.
func (d *Docker) Flatten(id string, size int64, tw io.Reader) error {
	imgName, err := image.Flatten(d.config, id, size, tw)
	if err != nil {
		return err
	}

	out, err := os.Open(imgName)
	if err != nil {
		return err
	}

	defer out.Close()
	defer os.Remove(out.Name())

	r, w := io.Pipe()

	go func() {
		err := copy.WithProgress(w, out, "Loading image into docker")
		if err != nil {
			w.CloseWithError(err)
		} else {
			w.Close()
		}
	}()

	resp, err := d.client.ImageLoad(context.Background(), r, true)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	parts := strings.SplitN(string(content), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("Invalid value returned from docker: %s", string(content))
	}

	d.config.Image = strings.TrimSpace(parts[1])
	return nil
}

// Tag an image with the provided string.
func (d *Docker) Tag(tag string) error {
	return d.client.ImageTag(context.Background(), d.config.Image, tag)
}

// Fetch retrieves a docker image, overwrites the container configuration, and returns its id.
func (d *Docker) Fetch(name string) (string, error) {
	inspect, _, err := d.client.ImageInspectWithRaw(context.Background(), name)
	if err != nil {
		reader, err := d.client.ImagePull(context.Background(), name, types.ImagePullOptions{})
		if err != nil {
			return "", err
		}

		if !d.tty {
			fmt.Printf("Pulling %q...", name)
		}

		if _, err := printPull(d.tty, reader); err != nil {
			return "", err
		}

		if !d.tty {
			fmt.Println("done.")
		}

		// this will fallthrough to the assignment below
		inspect, _, err = d.client.ImageInspectWithRaw(context.Background(), name)
		if err != nil {
			return "", err
		}
	}

	d.config.FromDocker(inspect.Config)
	d.config.Image = inspect.ID
	d.layers = inspect.RootFS.Layers
	return inspect.ID, nil
}
