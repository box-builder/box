package layers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/erikh/box/copy"
	"github.com/erikh/box/image"
)

// DockerImage is the Image interface applied to docker.
type DockerImage struct {
	imageConfig *ImageConfig
	client      *client.Client
	context     context.Context
}

// NewDockerImage constructs a new DockerImage
func NewDockerImage(context context.Context, imageConfig *ImageConfig) (*DockerImage, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	return &DockerImage{
		imageConfig: imageConfig,
		client:      client,
		context:     context,
	}, nil
}

// Flatten copies a tarred up series of files (passed in through the
// io.Reader handle) to the image where they are untarred.
func (d *DockerImage) Flatten(id string, size int64, tw io.Reader) error {
	imgName, err := image.Flatten(d.imageConfig.Config, id, size, tw, d.imageConfig.Logger)
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
		err := copy.WithProgress(w, out, d.imageConfig.Logger, "Loading image into docker")
		if err != nil {
			w.CloseWithError(err)
		} else {
			w.Close()
		}
	}()

	resp, err := d.client.ImageLoad(d.context, r, true)
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
	return d.client.ImageTag(d.context, d.imageConfig.Config.Image, tag)
}

// CheckCache consults the cache and returns true or false depending on whether
// there was a match. If there was an error consulting the cache, it will be
// returned as the second argument.
func (d *DockerImage) CheckCache(cacheKey string) (bool, error) {
	if !d.imageConfig.UseCache {
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
				d.imageConfig.Logger.CacheHit(img.ID)
				d.imageConfig.Config.FromDocker(inspect.Config)
				d.imageConfig.Config.Image = img.ID
				return true, d.imageConfig.Layers.AddImage(img.ID)
			}
		}
	}

	return false, nil
}

// UseCache determines if the cache should be considered or not.
func (d *DockerImage) UseCache(arg bool) {
	d.imageConfig.UseCache = arg
}

// GetCache gets the current value of whether or not to use the cache
func (d *DockerImage) GetCache() bool {
	return d.imageConfig.UseCache
}

// ImageID returns the image identifier of the most recent layer.
func (d *DockerImage) ImageID() string {
	return d.imageConfig.Config.Image
}
