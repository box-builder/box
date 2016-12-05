package docker

import (
	"archive/tar"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/erikh/box/builder/config"
	"github.com/erikh/box/builder/executor"
	"github.com/erikh/box/builder/signal"
	"github.com/erikh/box/copy"
	"github.com/erikh/box/image"
	"github.com/erikh/box/log"
	"github.com/fatih/color"
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

// MakeImage makes the final image, skipping any layers as necessary. The
// layers must be pre-recorded within the executor. Note that if you have no
// layers to skip, this operation will need to do nothing, so it will do
// nothing.
//
// It returns an error condition, if any.
func (d *Docker) MakeImage() error {
	// this is prinicipally an optimization so we can determine later if we
	// need to reconstruct the image.
	if len(d.skipLayers) == 0 {
		return nil
	}

	rc, err := d.client.ImageSave(context.Background(), []string{d.ImageID()})
	if err != nil {
		return nil
	}

	tf, err := ioutil.TempFile("", "box-downloaded-image")
	if err != nil {
		return err
	}

	defer os.Remove(tf.Name())

	if err := copy.WithProgress(tf, rc, "Downloading layers"); err != nil {
		tf.Close()
		return err
	}

	tf.Close()

	layers, dir, err := image.Unpack(tf.Name())

	if err := os.Remove(tf.Name()); err != nil {
		return err
	}

	defer func() {
		if dir != "" {
			os.RemoveAll(dir)
		}
	}()

	if err != nil {
		return err
	}

	if len(layers) < len(d.layers) {
		return fmt.Errorf("error: image mismatch; layers recorded are more than layers in image: %d - %d", len(layers), len(d.layers))
	}

	commitLayers := []*image.Layer{}

	for i := 0; i < len(layers); i++ {
		if i >= len(d.layers) {
			break
		}

		commit := true

		for layers[i].LayerID() != d.layers[i] {
			if i == 0 {
				layers = layers[1:]
			} else {
				layers = append(layers[:i-1], layers[i:]...)
			}

			if len(layers) < i || len(d.layers) < i {
				commit = false
				break
			}
		}

		if commit {
			commitLayers = append(commitLayers, layers[i])
		}
	}

	fn, err := image.Make(d.Config(), commitLayers)
	if err != nil {
		return err
	}

	f, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	defer os.Remove(f.Name())

	imgresp, err := d.client.ImageLoad(context.Background(), f, false)
	if err != nil {
		return err
	}

	d.config.Image, err = printPull(d.tty, imgresp.Body)

	return err
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
				// this is prinicipally an optimization so we can determine later if we
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
	return d.client.CopyToContainer(context.Background(), id, "/", r, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true})
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

// RunHook is the run hook for docker agents.
func (d *Docker) RunHook(id string) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	signal.SetSignal(func() {
		cancel()
		d.Destroy(id)
	})

	stopChan := make(chan struct{})
	errChan := make(chan error, 1)

	defer close(errChan)

	go func() {
		err, ok := <-errChan
		if ok {
			fmt.Printf("\n\n+++ Run Error: %#v\n", err)
			cancel()
			d.Destroy(id)
		}
	}()

	defer signal.SetSignal(nil)

	cearesp, err := d.client.ContainerAttach(ctx, id, types.ContainerAttachOptions{Stream: true, Stdin: d.stdin, Stdout: true, Stderr: true})
	if err != nil {
		return "", fmt.Errorf("Could not attach to container: %v", err)
	}

	if d.stdin {
		state, err := term.SetRawTerminal(0)
		if err != nil {
			return "", fmt.Errorf("Could not attach terminal to container: %v", err)
		}

		defer term.RestoreTerminal(0, state)

		go doCopy(cearesp.Conn, os.Stdin, errChan, stopChan)
	}

	defer cearesp.Close()

	err = d.client.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return "", fmt.Errorf("Could not start container: %v", err)
	}

	if !d.stdin {
		color.New(color.FgRed, color.Bold, color.BgWhite).Printf("------ BEGIN OUTPUT ------")
		color.Unset()
		fmt.Println()
	}

	if !d.tty {
		go func() {
			// docker mux's the streams, and requires this stdcopy library to unpack them.
			_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, cearesp.Reader)
			if err != nil && err != io.EOF {
				select {
				case <-stopChan:
				default:
					errChan <- err
				}
			}
		}()
	} else if d.tty {
		go doCopy(os.Stdout, cearesp.Reader, errChan, stopChan)
	}

	defer close(stopChan)

	stat, err := d.client.ContainerWait(ctx, id)
	if err != nil {
		return "", err
	}

	if !d.stdin {
		color.New(color.FgRed, color.Bold, color.BgWhite).Printf("------- END OUTPUT -------")
		color.Unset()
		fmt.Println()
	}

	if stat != 0 {
		return "", fmt.Errorf("Command exited with status %d for container %q", stat, id)
	}

	return "", nil
}

type pullInfo struct {
	status   string
	progress float64
}

func printPull(tty bool, reader io.Reader) (string, error) {
	idmap := map[string]pullInfo{}
	idlist := []string{}
	var retval string

	buf := bufio.NewReader(reader)
	for retval == "" {
		line, err := buf.ReadBytes('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}

		var unpacked map[string]interface{}
		if err := json.Unmarshal(line, &unpacked); err != nil {
			return "", err
		}

		if stream, ok := unpacked["stream"].(string); ok {
			// FIXME this is absolutely terrible
			if strings.HasPrefix(stream, "Loaded image ID:") {
				retval = strings.TrimSpace(strings.TrimPrefix(stream, "Loaded image ID:"))
			}
		}

		progressCount := float64(0)
		progress, pok := unpacked["progressDetail"].(map[string]interface{})
		if pok {
			current, cok := progress["current"]
			total, tok := progress["total"]
			if cok && tok {
				progressCount = (current.(float64) / total.(float64)) * 100
			}
		}

		status, _ := unpacked["status"].(string)
		id, idok := unpacked["id"].(string)
		if idok {
			if _, ok := idmap[id]; !ok {
				idlist = append(idlist, id)
			}

			idmap[id] = pullInfo{status, progressCount}
		}

		if tty {
			for _, id := range idlist {
				if idmap[id].progress == 0 {
					fmt.Printf("\r\x1b[K%s %s\n", id, idmap[id].status)
				} else {
					fmt.Printf("\r\x1b[K%s %s %3.0f%%\n", id, idmap[id].status, idmap[id].progress)
				}
			}

			if !idok && status != "" {
				if pok { // image load only
					fmt.Printf("\r\x1b[K%s %3.0f%%", status, progressCount)
				} else {
					fmt.Printf("\r\x1b[K%s\n", status)
					fmt.Printf("\x1b[%dA", len(idmap)+1)
				}
			} else {
				if len(idmap) != 0 {
					fmt.Printf("\x1b[%dA", len(idmap))
				}
			}
		}
	}

	if tty {
		for i := 0; i < len(idmap)+1; i++ {
			fmt.Println()
		}

		fmt.Println("Loaded image", retval)
	}

	return retval, nil
}

func doCopy(wtr io.Writer, rdr io.Reader, errChan chan error, stopChan chan struct{}) {
	// repeat copy until error is returned. if error is not io.EOF, forward
	// to channel. Return on any error.
	for {
		select {
		case <-stopChan:
			return
		default:
		}

		if _, err := io.Copy(wtr, rdr); err == nil {
			continue
		} else if _, ok := err.(*net.OpError); ok {
			continue
		} else if err != io.EOF {
			select {
			case <-stopChan:
			case errChan <- err:
			default:
			}
		}

		return
	}
}
