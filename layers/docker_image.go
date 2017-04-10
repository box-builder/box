package layers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/box-builder/box/copy"
	"github.com/box-builder/box/image"
	om "github.com/box-builder/overmount"
	"github.com/box-builder/overmount/imgio"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// DockerImage is the Image interface applied to docker.
type DockerImage struct {
	imageConfig *ImageConfig
	client      *client.Client
}

// NewDockerImage contypes a new DockerImage
func NewDockerImage(imageConfig *ImageConfig) (*DockerImage, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return &DockerImage{
		imageConfig: imageConfig,
		client:      client,
	}, nil
}

func (d *DockerImage) ociSave(filename, tag string) error {
	repo, err := om.NewRepository(path.Join(os.Getenv("HOME"), ".overmount"), true)
	if err != nil {
		return err
	}

	img, err := imgio.NewDocker(d.client)
	if err != nil {
		return err
	}

	reader, err := d.client.ImageSave(d.imageConfig.Globals.Context, []string{d.imageConfig.Config.Image})
	if err != nil {
		return err
	}

	layers, err := repo.Import(img, reader)
	if err != nil {
		return err
	}

	if len(layers) != 1 {
		return errors.New("image query expected one, returned more than one image")
	}

	imageContent, err := repo.Export(imgio.NewOCI(), layers[0], []string{tag})
	if err != nil {
		return err
	}

	w, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer w.Close()

	return copy.WithProgress(w, imageContent, d.imageConfig.Globals.Logger, fmt.Sprintf("Saving %q", filename))
}

func (d *DockerImage) dockerSave(f io.WriteCloser, filename, tag string) error {
	r, err := d.client.ImageSave(d.imageConfig.Globals.Context, []string{d.imageConfig.Config.Image, tag})
	if err != nil {
		return err
	}

	return copy.WithProgress(f, r, d.imageConfig.Globals.Logger, fmt.Sprintf("Saving %q to disk", filename))
}

// Save saves an image to the provided filename.
func (d *DockerImage) Save(filename, kind, tag string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	abs, err := filepath.Abs(filename)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(wd, abs)
	if err != nil {
		return err
	}

	if strings.HasPrefix(rel, "../") {
		return fmt.Errorf("relative path %q for save falls below the working directory, cannot save", rel)
	}

	switch kind {
	case "", "docker":
		f, err := os.Create(rel)
		if err != nil {
			return err
		}
		defer f.Close()
		return d.dockerSave(f, filename, tag)
	case "oci":
		return d.ociSave(filename, tag)
	default:
		return fmt.Errorf("image kind %q is not valid", kind)
	}
}

// Flatten copies a tarred up series of files (passed in through the
// io.Reader handle) to the image where they are untarred.
func (d *DockerImage) Flatten(tw io.Reader) error {
	imgName, err := image.NewImage(d.imageConfig.Globals, nil, d.imageConfig.Config, nil).Flatten(tw)
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
		err := copy.WithProgress(w, out, d.imageConfig.Globals.Logger, "Loading image into docker")
		if err != nil {
			w.CloseWithError(err)
		} else {
			w.Close()
		}
	}()

	resp, err := d.client.ImageLoad(d.imageConfig.Globals.Context, r, true)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	res := map[string]string{}

	if err := json.Unmarshal(content, &res); err != nil {
		parts := strings.SplitN(string(content), ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("Invalid value returned from docker: %s", string(content))
		}

		d.imageConfig.Config.Image = strings.TrimSpace(parts[1])
		return nil
	}

	if stream, ok := res["stream"]; ok {
		// FIXME this is absolutely terrible
		if strings.HasPrefix(stream, "Loaded image ID: ") {
			d.imageConfig.Config.Image = strings.TrimSpace(strings.TrimPrefix(stream, "Loaded image ID: "))
			return nil
		}
	}

	return errors.New("invalid image ID returned")
}

// Tag an image with the provided string.
func (d *DockerImage) Tag(tag string) error {
	return d.client.ImageTag(d.imageConfig.Globals.Context, d.imageConfig.Config.Image, tag)
}

// CheckCache consults the cache and returns true or false depending on whether
// there was a match. If there was an error consulting the cache, it will be
// returned as the second argument.
func (d *DockerImage) CheckCache(cacheKey string) (bool, error) {
	if !d.imageConfig.Globals.Cache {
		return false, nil
	}

	images, err := d.client.ImageList(context.Background(), types.ImageListOptions{All: true})
	if err != nil {
		return false, err
	}

	for _, img := range images {
		if (img.ParentID != "" && img.ParentID == d.imageConfig.Config.Image) || img.ParentID == "" {
			inspect, _, err := d.client.ImageInspectWithRaw(context.Background(), img.ID)
			if err != nil {
				return false, err
			}

			if inspect.Comment == cacheKey {
				d.imageConfig.Globals.Logger.CacheHit(img.ID)
				d.imageConfig.Config.FromDocker(inspect.Config)
				d.imageConfig.Config.Image = img.ID
				return true, d.imageConfig.Layers.AddImage(img.ID)
			}
		}
	}

	return false, nil
}

// ImageID returns the image identifier of the most recent layer.
func (d *DockerImage) ImageID() string {
	return d.imageConfig.Config.Image
}
