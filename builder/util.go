package builder

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/engine-api/types"
	mruby "github.com/mitchellh/go-mruby"
)

func (b *Builder) commit(cacheKey string, hook func(b *Builder, id string) (string, error)) error {
	if os.Getenv("NO_CACHE") != "" {
		cacheKey = ""
	}

	b.config.Image = b.imageID

	resp, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return err
	}

	if hook != nil {
		tmp, err := hook(b, resp.ID)
		if err != nil {
			return err
		}

		if tmp != "" && os.Getenv("NO_CACHE") == "" {
			cacheKey = tmp
		}
	}

	commitResp, err := b.client.ContainerCommit(context.Background(), resp.ID, types.ContainerCommitOptions{Config: b.config, Comment: cacheKey})
	if err != nil {
		return fmt.Errorf("Error during commit: %v", err)
	}

	err = b.client.ContainerRemove(context.Background(), resp.ID, types.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("Could not remove intermediate container %q: %v", resp.ID, err)
	}

	b.imageID = commitResp.ID

	return nil
}

func createException(m *mruby.Mrb, msg string) mruby.Value {
	val, err := m.Class("Exception", nil).New(mruby.String(msg))
	if err != nil {
		panic(fmt.Sprintf("could not construct exception for return: %v", err))
	}

	return val
}

func (b *Builder) resetConfig() {
	b.config.WorkingDir = "/"
	b.config.User = "root"
	b.config.Cmd = nil
	b.config.Entrypoint = []string{"/bin/sh", "-c"}
}

func extractStringArgs(m *mruby.Mrb) []string {
	args := m.GetArgs()
	strArgs := []string{}
	for _, arg := range args {
		if arg.Type() != mruby.TypeProc {
			strArgs = append(strArgs, arg.String())
		}
	}

	return strArgs
}

func (b *Builder) consultCache(cacheKey string) (bool, error) {
	if os.Getenv("NO_CACHE") == "" {
		if b.imageID != "" {
			images, err := b.client.ImageList(context.Background(), types.ImageListOptions{All: true})
			if err != nil {
				return false, err
			}

			for _, img := range images {
				if img.ParentID == b.imageID {
					inspect, _, err := b.client.ImageInspectWithRaw(context.Background(), img.ID)
					if err != nil {
						return false, err
					}

					if inspect.Comment == cacheKey {
						fmt.Printf("+++ Cache hit: using %q\n", img.ID)
						b.imageID = img.ID
						b.config = inspect.Config
						b.entrypoint = inspect.Config.Entrypoint
						b.cmd = inspect.Config.Cmd

						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}

func (b *Builder) tarPath(rel, target string) (string, error) {
	fi, err := os.Lstat(rel)
	if err != nil {
		return "", err
	}

	f, err := ioutil.TempFile("", "box-copy.")
	if err != nil {
		return "", err
	}

	tw := tar.NewWriter(f)

	if fi.IsDir() {
		err := filepath.Walk(rel, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if fi.IsDir() {
				return nil
			}

			fmt.Printf("--- Copy: %s -> %s\n", path, filepath.Join(target, path))

			header, err := tar.FileInfoHeader(fi, filepath.Join(target, path))
			if err != nil {
				return err
			}

			header.Linkname = filepath.Join(target, path)
			header.Name = filepath.Join(target, path)

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			f, err := os.Open(path)
			if err != nil {
				return err
			}

			_, err = io.Copy(tw, f)
			if err != nil && err != io.EOF {
				f.Close()
				return err
			}

			f.Close()
			return nil
		})
		if err != nil {
			return "", err
		}

	} else {
		header, err := tar.FileInfoHeader(fi, target)
		if err != nil {
			return "", err
		}

		header.Name = target
		header.Linkname = target

		if err := tw.WriteHeader(header); err != nil {
			return "", err
		}

		f, err := os.Open(rel)
		if err != nil {
			return "", err
		}
		_, err = io.Copy(tw, f)
		if err != nil && err != io.EOF {
			f.Close()
			return "", err
		}
		f.Close()
	}

	tw.Flush()
	tw.Close()
	f.Close()

	return f.Name(), nil
}
