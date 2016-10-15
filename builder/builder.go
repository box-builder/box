package builder

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	mruby "github.com/mitchellh/go-mruby"
)

// Builder implements the builder core.
type Builder struct {
	mrb        *mruby.Mrb
	client     *client.Client
	config     *container.Config
	user       string
	workdir    string
	cmd        []string
	entrypoint []string
	insideDir  string
}

// NewBuilder creates a new builder. Returns error on docker or mruby issues.
func NewBuilder() (*Builder, error) {
	client, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	builder := &Builder{mrb: mruby.NewMrb(), client: client, config: &container.Config{}}

	for name, def := range verbJumpTable {
		builder.AddVerb(name, def.verbFunc, def.argSpec)
	}

	for name, def := range funcJumpTable {
		inner := def.fun
		fn := func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
			return inner(builder, m, self)
		}

		builder.mrb.TopSelf().SingletonClass().DefineMethod(name, fn, def.argSpec)
	}

	builder.entrypoint = []string{"/bin/sh", "-c"}
	builder.cmd = []string{"/bin/sh"}
	builder.user = "root"
	builder.workdir = "/"
	builder.resetConfig()

	return builder, nil
}

// ImageID returns the latest known Image identifier that we committed. At the
// end of the run this will be the golden docker image.
func (b *Builder) ImageID() string {
	return b.config.Image
}

// AddVerb adds a function to the mruby dispatch as well as adding hooks around
// the call to ensure containers are committed and intermediate layers are
// cleared.
func (b *Builder) AddVerb(name string, fn verbFunc, args mruby.ArgSpec) {
	builderFunc := func(m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
		strArgs := extractStringArgs(m)
		cacheKey := strings.Join(append([]string{name}, strArgs...), ", ")
		fmt.Printf("+++ Execute: %s %s\n", name, strings.Join(strArgs, ", "))
		cached, err := b.consultCache(cacheKey)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		if !cached {
			return fn(b, cacheKey, m, self)
		}

		return nil, nil
	}

	b.mrb.TopSelf().SingletonClass().DefineMethod(name, builderFunc, args)
}

// Run the script.
func (b *Builder) Run(script string) (*mruby.MrbValue, error) {
	return b.mrb.LoadString(script)
}

// Close tears down all functions of the builder, preparing it for exit.
func (b *Builder) Close() error {
	b.mrb.Close()
	return nil
}

func (b *Builder) resetConfig() {
	b.config.WorkingDir = b.workdir
	b.config.User = b.user
	b.config.Cmd = b.cmd
	b.config.Entrypoint = b.entrypoint
}

func (b *Builder) createEmptyContainer() (string, error) {
	cont, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)

	return cont.ID, err
}

func (b *Builder) containerContent(fn string) ([]byte, error) {
	id, err := b.createEmptyContainer()
	if err != nil {
		return nil, err
	}

	defer b.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{Force: true})

	rc, _, err := b.client.CopyFromContainer(context.Background(), id, fn)
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

func (b *Builder) commit(cacheKey string, hook func(b *Builder, id string) (string, error)) error {
	if os.Getenv("NO_CACHE") != "" {
		cacheKey = ""
	}

	id, err := b.createEmptyContainer()
	if err != nil {
		return err
	}

	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_, ok := <-signals
		if ok {
			b.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{Force: true})
		}
	}()

	defer func() {
		b.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{Force: true})
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}()

	if hook != nil {
		tmp, err := hook(b, id)
		if err != nil {
			return err
		}

		if tmp != "" && os.Getenv("NO_CACHE") == "" {
			cacheKey = tmp
		}
	}

	b.resetConfig()

	commitResp, err := b.client.ContainerCommit(context.Background(), id, types.ContainerCommitOptions{Config: b.config, Comment: cacheKey})
	if err != nil {
		return fmt.Errorf("Error during commit: %v", err)
	}

	// try a clean remove first, otherwise the defer above will take over in a last-ditch attempt
	err = b.client.ContainerRemove(context.Background(), id, types.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("Could not remove intermediate container %q: %v", id, err)
	}

	b.config.Image = commitResp.ID

	return nil
}

func (b *Builder) consultCache(cacheKey string) (bool, error) {
	if os.Getenv("NO_CACHE") == "" {
		if b.config.Image != "" {
			images, err := b.client.ImageList(context.Background(), types.ImageListOptions{All: true})
			if err != nil {
				return false, err
			}

			for _, img := range images {
				if img.ParentID == b.config.Image {
					inspect, _, err := b.client.ImageInspectWithRaw(context.Background(), img.ID)
					if err != nil {
						return false, err
					}

					if inspect.Comment == cacheKey {
						fmt.Printf("+++ Cache hit: using %q\n", img.ID)
						b.config = inspect.Config
						b.user = b.config.User
						b.workdir = b.config.WorkingDir
						b.cmd = b.config.Cmd
						b.entrypoint = b.config.Entrypoint
						b.config.Image = img.ID

						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}
