package builder

import (
	"archive/tar"
	"bufio"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/engine-api/types"
	mruby "github.com/mitchellh/go-mruby"
)

func createException(m *mruby.Mrb, msg string) mruby.Value {
	val, err := m.Class("Exception", nil).New(mruby.String(msg))
	if err != nil {
		panic(fmt.Sprintf("could not construct exception for return: %v", err))
	}

	return val
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

func tarPath(rel, target string) (string, error) {
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

			p, err := os.Open(path)
			if err != nil {
				return err
			}

			if header.Typeflag == tar.TypeReg {
				_, err = io.Copy(tw, p)
				if err != nil && err != io.EOF {
					p.Close()
					return err
				}

				p.Close()
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	} else if !fi.IsDir() {
		header, err := tar.FileInfoHeader(fi, target)
		if err != nil {
			return "", err
		}

		header.Name = target
		header.Linkname = target

		if err := tw.WriteHeader(header); err != nil {
			return "", err
		}

		p, err := os.Open(rel)
		if err != nil {
			return "", err
		}
		_, err = io.Copy(tw, p)
		if err != nil && err != io.EOF {
			p.Close()
			return "", err
		}
		p.Close()
	}

	tw.Close()
	f.Close()

	return f.Name(), nil
}

func sumFile(fn string) (string, error) {
	f, err := os.Open(fn)
	if err != nil {
		return "", err
	}

	hash := sha512.New512_256()
	_, err = io.Copy(hash, f)
	if err != nil && err != io.EOF {
		f.Close()
		return "", err
	}
	cacheKey := fmt.Sprintf("box:copy %s", hex.EncodeToString(hash.Sum(nil)))
	f.Close()

	return cacheKey, nil
}

func runHook(b *Builder, id string) (string, error) {
	cearesp, err := b.client.ContainerAttach(context.Background(), id, types.ContainerAttachOptions{Stream: true, Stdout: true, Stderr: true})
	if err != nil {
		return "", fmt.Errorf("Could not attach to container: %v", err)
	}

	err = b.client.ContainerStart(context.Background(), id, types.ContainerStartOptions{})
	if err != nil {
		return "", fmt.Errorf("Could not start container: %v", err)
	}

	fmt.Println("------ BEGIN OUTPUT ------")

	_, err = io.Copy(os.Stdout, cearesp.Reader)
	if err != nil && err != io.EOF {
		return "", err
	}

	fmt.Println("------ END OUTPUT ------")

	stat, err := b.client.ContainerWait(context.Background(), id)
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

func iterateRubyHash(arg *mruby.MrbValue, fn func(*mruby.MrbValue, *mruby.MrbValue) error) error {
	hash := arg.Hash()

	// mruby does not expose native maps, just ruby primitives, so we have to
	// iterate through it with indexing functions instead of typical idioms.
	keys, err := hash.Keys()
	if err != nil {
		return err
	}

	for i := 0; i < keys.Array().Len(); i++ {
		key, err := keys.Array().Get(i)
		if err != nil {
			return err
		}

		value, err := hash.Get(key)
		if err != nil {
			return err
		}

		if err := fn(key, value); err != nil {
			return err
		}
	}

	return nil
}

func checkArgs(args []*mruby.MrbValue, l int) error {
	if len(args) != l {
		return fmt.Errorf("Expected %d arg, got %d", l, len(args))
	}

	return nil
}

func checkImage(b *Builder) error {
	if b.ImageID() != "" {
		return nil
	}

	return errors.New("from has not been called, no image can be used for this operation")
}

func standardCheck(b *Builder, args []*mruby.MrbValue, l int) error {
	if err := checkArgs(args, l); err != nil {
		return err
	}

	return checkImage(b)
}
