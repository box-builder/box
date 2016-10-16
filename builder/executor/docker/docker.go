package docker

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/erikh/box/builder/config"
	"github.com/erikh/box/builder/executor"
)

// Docker implements an executor that talks to docker to achieve its goals.
type Docker struct {
	client *client.Client
	config *config.Config
}

// NewDocker constructs a new docker instance, for executing against docker
// engines.
func NewDocker() (*Docker, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return &Docker{
		client: client,
		config: config.NewConfig(),
	}, nil
}

// ImageID returns the image identifier of the most recent layer.
func (d *Docker) ImageID() string {
	return d.config.Image
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
	if os.Getenv("NO_CACHE") != "" {
		cacheKey = ""
	}

	id, err := d.Create()
	if err != nil {
		return err
	}

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_, ok := <-signals
		if ok {
			d.Destroy(id)
		}
	}()

	defer func() {
		d.Destroy(id)
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}()

	if hook != nil {
		tmp, err := hook(id)
		if err != nil {
			return err
		}

		if tmp != "" && os.Getenv("NO_CACHE") == "" {
			cacheKey = tmp
		}
	}

	commitResp, err := d.client.ContainerCommit(context.Background(), id, types.ContainerCommitOptions{Config: d.config.ToDocker(), Comment: cacheKey})
	if err != nil {
		return fmt.Errorf("Error during commit: %v", err)
	}

	// try a clean remove first, otherwise the defer above will take over in a last-ditch attempt
	err = d.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("Could not remove intermediate container %q: %v", id, err)
	}

	d.config.Image = commitResp.ID

	return nil
}

// CheckCache consults the cache and returns true or false depending on whether
// there was a match. If there was an error consulting the cache, it will be
// returned as the second argument.
func (d *Docker) CheckCache(cacheKey string) (bool, error) {
	if os.Getenv("NO_CACHE") == "" {
		if d.config.Image != "" {
			images, err := d.client.ImageList(context.Background(), types.ImageListOptions{All: true})
			if err != nil {
				return false, err
			}

			for _, img := range images {
				if img.ParentID == d.config.Image {
					inspect, _, err := d.client.ImageInspectWithRaw(context.Background(), img.ID)
					if err != nil {
						return false, err
					}

					if inspect.Comment == cacheKey {
						fmt.Printf("+++ Cache hit: using %q\n", img.ID)
						d.config.FromDocker(inspect.Config)
						d.config.Image = img.ID
						return true, nil
					}
				}
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
		d.config.ToDocker(),
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
func (d *Docker) CopyFromContainer(id, path string) (io.Reader, error) {
	rc, _, err := d.client.CopyFromContainer(context.Background(), id, path)
	return rc, err
}

// CopyToContainer copies a tarred up series of files (passed in through the
// io.Reader handle) to the container where they are untarred.
func (d *Docker) CopyToContainer(id, path string, tw io.Reader) error {
	return d.client.CopyToContainer(context.Background(), id, path, tw, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true})
}

// Tag an image with the provided string.
func (d *Docker) Tag(tag string) error {
	return d.client.ImageTag(context.Background(), d.config.Image, tag)
}

// Fetch retrieves a docker image and returns its id.
func (d *Docker) Fetch(name string) (string, error) {
	inspect, _, err := d.client.ImageInspectWithRaw(context.Background(), name)
	if err != nil {
		reader, err := d.client.ImagePull(context.Background(), name, types.ImagePullOptions{})
		if err != nil {
			return "", err
		}

		if err := printPull(reader); err != nil {
			return "", err
		}

		// this will fallthrough to the assignment below
		inspect, _, err = d.client.ImageInspectWithRaw(context.Background(), name)
		if err != nil {
			return "", err
		}
	}

	return inspect.ID, nil
}

// RunHook is the run hook for docker agents.
func (d *Docker) RunHook(id string) (string, error) {
	cearesp, err := d.client.ContainerAttach(context.Background(), id, types.ContainerAttachOptions{Stream: true, Stdout: true, Stderr: true})
	if err != nil {
		return "", fmt.Errorf("Could not attach to container: %v", err)
	}

	err = d.client.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
	if err != nil {
		return "", fmt.Errorf("Could not start container: %v", err)
	}

	fmt.Println("------ BEGIN OUTPUT ------")

	_, err = io.Copy(os.Stdout, cearesp.Reader)
	if err != nil && err != io.EOF {
		return "", err
	}

	fmt.Println("------ END OUTPUT ------")

	stat, err := d.client.ContainerWait(context.Background(), id)
	if err != nil {
		return "", err
	}

	if stat != 0 {
		return "", fmt.Errorf("Command exited with status %d for container %q", stat, id)
	}

	return "", nil
}

func printPull(reader io.Reader) error {
	idmap := map[string][]string{}
	idlist := []string{}

	fmt.Println()

	buf := bufio.NewReader(reader)
	for {
		line, err := buf.ReadBytes('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		var unpacked map[string]interface{}
		if err := json.Unmarshal(line, &unpacked); err != nil {
			return err
		}

		progress, ok := unpacked["progress"].(string)
		if !ok {
			progress = ""
		}

		status := unpacked["status"].(string)
		id, ok := unpacked["id"].(string)
		if !ok {
			fmt.Printf("\x1b[%dA", len(idmap)+1)
			fmt.Printf("\r\x1b[K%s\n", status)
		} else {
			fmt.Printf("\x1b[%dA", len(idmap))
			if _, ok := idmap[id]; !ok {
				idlist = append(idlist, id)
			}

			idmap[id] = []string{status, progress}
		}

		for _, id := range idlist {
			fmt.Printf("\r\x1b[K%s %s %s\n", id, idmap[id][0], idmap[id][1])
		}
	}

	return nil
}
