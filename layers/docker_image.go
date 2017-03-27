package layers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/box-builder/box/copy"
	"github.com/box-builder/box/image"
	"github.com/box-builder/box/tar"
	ccopy "github.com/containers/image/copy"
	"github.com/containers/image/docker/daemon"
	"github.com/containers/image/oci/layout"
	"github.com/containers/image/signature"
	ctypes "github.com/containers/image/types"
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
	ref, err := daemon.ParseReference(d.imageConfig.Config.Image)
	if err != nil {
		return err
	}

	tmpdir, err := ioutil.TempDir("", "image-")
	if err != nil {
		return err
	}

	tgt, err := layout.NewReference(tmpdir, tag)
	if err != nil {
		return err
	}

	pc, err := signature.NewPolicyContext(&signature.Policy{
		Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()},
	})

	if err != nil {
		return err
	}

	progressChan := make(chan ctypes.ProgressProperties)

	go func() {
		var last string
		for prog := range progressChan {
			digest := prog.Artifact.Digest.String()

			if digest == last {
				fmt.Print("\r")
			} else if last != "" {
				fmt.Println()
			}

			d.imageConfig.Globals.Logger.Progress(strings.SplitN(digest, ":", 2)[1][:12], float64(prog.Offset/megaByte))
			last = digest
		}

		fmt.Println()
	}()

	_, err = ccopy.Image(pc, tgt, ref, &ccopy.Options{
		RemoveSignatures: true,
		ProgressInterval: 100 * time.Millisecond,
		Progress:         progressChan,
	})

	if err != nil {
		return err
	}

	file, _, err := tar.Archive(d.imageConfig.Globals.Context, tmpdir, "", nil, d.imageConfig.Globals.Logger)
	if err != nil {
		return err
	}

	// manually copy the file
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	w, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer w.Close()

	return copy.WithProgress(w, r, d.imageConfig.Globals.Logger, fmt.Sprintf("Saving %q", filename))
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
func (d *DockerImage) Flatten(id string, size int64, tw io.Reader) error {
	imgName, err := image.Flatten(d.imageConfig.Config, id, size, tw, d.imageConfig.Globals.Logger)
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
