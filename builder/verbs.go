package builder

import (
	"archive/tar"
	"bufio"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/engine-api/types"
	mruby "github.com/mitchellh/go-mruby"
)

// Definition is a jump table definition used for programming the DSL into the
// mruby interpreter.
type Definition struct {
	Func    Func
	ArgSpec mruby.ArgSpec
}

var jumpTable = map[string]Definition{
	"flatten":    {flatten, mruby.ArgsNone()},
	"tag":        {tag, mruby.ArgsReq(1)},
	"copy":       {copy, mruby.ArgsReq(2)},
	"from":       {from, mruby.ArgsReq(1)},
	"run":        {run, mruby.ArgsAny()},
	"with_user":  {withUser, mruby.ArgsBlock() | mruby.ArgsReq(1)},
	"inside":     {inside, mruby.ArgsBlock() | mruby.ArgsReq(1)},
	"env":        {env, mruby.ArgsAny()},
	"cmd":        {cmd, mruby.ArgsAny()},
	"entrypoint": {entrypoint, mruby.ArgsAny()},
}

// Func is a builder DSL function used to interact with docker.
type Func func(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value)

func flatten(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	cont, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	rc, err := b.client.ContainerExport(context.Background(), cont.ID)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	f, err := ioutil.TempFile("", "box-flatten.")
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer os.Remove(f.Name())
	if _, err := io.Copy(f, rc); err != nil && err != io.EOF {
		f.Close()
		return nil, createException(m, err.Error())
	}
	f.Close()

	f, err = os.Open(f.Name())
	if err != nil {
		return nil, createException(m, err.Error())
	}

	b.config.Image = ""

	cont2, err := b.client.ContainerCreate(
		context.Background(),
		b.config,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer b.client.ContainerRemove(context.Background(), cont2.ID, types.ContainerRemoveOptions{})

	if err := b.client.CopyToContainer(context.Background(), cont2.ID, "/", f, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true}); err != nil {
		return nil, createException(m, err.Error())
	}

	commitResp, err := b.client.ContainerCommit(context.Background(), cont2.ID, types.ContainerCommitOptions{Config: b.config})
	if err != nil {
		return nil, createException(m, err.Error())
	}

	b.imageID = commitResp.ID
	fmt.Printf("+++ Flattened Image: %s\n", b.imageID)
	return nil, nil
}

func tag(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	if len(args) != 1 {
		return nil, createException(m, "tag call expects one argument!")
	}

	if err := b.client.ImageTag(context.Background(), b.imageID, args[0].String()); err != nil {
		return nil, createException(m, err.Error())
	}

	fmt.Printf("+++ Tagged: %q\n", args[0].String())

	return nil, nil
}

func entrypoint(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	stringArgs := []string{}
	for _, arg := range m.GetArgs() {
		stringArgs = append(stringArgs, arg.String())
	}

	b.entrypoint = stringArgs
	b.config.Entrypoint = stringArgs

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func from(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.imageID = args[0].String()
	b.config.Tty = true
	b.config.AttachStdout = true
	b.config.AttachStderr = true

	idmap := map[string][]string{}
	idlist := []string{}

	inspect, _, err := b.client.ImageInspectWithRaw(context.Background(), args[0].String())
	if err != nil {
		reader, err := b.client.ImagePull(context.Background(), args[0].String(), types.ImagePullOptions{})
		if err != nil {
			return nil, createException(m, err.Error())
		}

		fmt.Println()

		buf := bufio.NewReader(reader)
		for {
			line, err := buf.ReadBytes('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, createException(m, err.Error())
			}

			var unpacked map[string]interface{}
			if err := json.Unmarshal(line, &unpacked); err != nil {
				return nil, createException(m, err.Error())
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

		// this will fallthrough to the assignment below
		inspect, _, err = b.client.ImageInspectWithRaw(context.Background(), args[0].String())
		if err != nil {
			return nil, createException(m, err.Error())
		}
	}

	b.imageID = inspect.ID

	return mruby.String(b.imageID), nil
}

func run(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	if b.imageID == "" {
		return nil, createException(m, "`from` must precede any `run` statements")
	}

	stringArgs := []string{}
	for _, arg := range m.GetArgs() {
		stringArgs = append(stringArgs, arg.String())
	}

	b.resetConfig()
	b.config.Entrypoint = []string{"/bin/sh", "-c"}
	b.config.Cmd = stringArgs
	b.config.WorkingDir = b.insideDir

	defer func() {
		b.resetConfig()
		b.config.Entrypoint = b.entrypoint
		b.config.Cmd = b.cmd
	}()

	hook := func(b *Builder, id string) (string, error) {
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

	if err := b.commit(cacheKey, hook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func withUser(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.config.User = args[0].String()
	val, err := m.Yield(args[1], args[0])
	b.config.User = ""

	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	return val, nil
}

func inside(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	b.insideDir = args[0].String()
	val, err := m.Yield(args[1], args[0])
	b.insideDir = ""

	if err != nil {
		return nil, createException(m, fmt.Sprintf("Could not yield: %v", err))
	}

	return val, nil
}

func env(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()
	hash := args[0].Hash()

	// mruby does not expose native maps, just ruby primitives, so we have to
	// iterate through it with indexing functions instead of typical idioms.
	keys, err := hash.Keys()
	if err != nil {
		return nil, createException(m, err.Error())
	}

	for i := 0; i < keys.Array().Len(); i++ {
		key, err := keys.Array().Get(i)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		value, err := hash.Get(key)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		b.config.Env = append(b.config.Env, fmt.Sprintf("%s=%s", key.String(), value.String()))
	}

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func cmd(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	stringArgs := []string{}
	for _, arg := range args {
		stringArgs = append(stringArgs, arg.String())
	}

	b.cmd = stringArgs
	b.config.Cmd = stringArgs

	if err := b.commit(cacheKey, nil); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}

func copy(b *Builder, cacheKey string, m *mruby.Mrb, self *mruby.MrbValue) (mruby.Value, mruby.Value) {
	args := m.GetArgs()

	if len(args) != 2 {
		return nil, createException(m, "Did not receive the proper number of arguments in copy")
	}

	source := filepath.Clean(args[0].String())
	target := filepath.Clean(args[1].String())

	wd, err := os.Getwd()
	if err != nil {
		return nil, createException(m, err.Error())
	}

	// FIXME do not allow traversing above the wd
	rel, err := filepath.Rel(wd, filepath.Join(wd, source))
	if err != nil {
		return nil, createException(m, err.Error())
	}

	fmt.Printf("+++ Copying: %q to %q\n", rel, target)

	fi, err := os.Lstat(rel)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	f, err := ioutil.TempFile("", "box-copy.")
	if err != nil {
		return nil, createException(m, err.Error())
	}

	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

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
			return nil, createException(m, err.Error())
		}

	} else {
		header, err := tar.FileInfoHeader(fi, target)
		if err != nil {
			return nil, createException(m, err.Error())
		}

		header.Name = target
		header.Linkname = target

		if err := tw.WriteHeader(header); err != nil {
			return nil, createException(m, err.Error())
		}

		f, err := os.Open(rel)
		if err != nil {
			return nil, createException(m, err.Error())
		}
		_, err = io.Copy(tw, f)
		if err != nil && err != io.EOF {
			f.Close()
			return nil, createException(m, err.Error())
		}
		f.Close()
	}

	tw.Flush()
	tw.Close()
	f.Close()

	f, err = os.Open(f.Name())
	if err != nil {
		return nil, createException(m, err.Error())
	}

	hash := sha512.New512_256()
	_, err = io.Copy(hash, f)
	if err != nil && err != io.EOF {
		f.Close()
		return nil, createException(m, err.Error())
	}
	cacheKey = fmt.Sprintf("box:copy %s", hex.EncodeToString(hash.Sum(nil)))
	f.Close()

	cached, err := b.consultCache(cacheKey)
	if err != nil {
		return nil, createException(m, err.Error())
	}

	if cached {
		return nil, nil
	}

	f, err = os.Open(f.Name())
	if err != nil {
		return nil, createException(m, err.Error())
	}

	hook := func(b *Builder, id string) (string, error) {
		defer f.Close()
		dir := b.insideDir
		if dir == "" {
			dir = "/"
		}

		return "", b.client.CopyToContainer(context.Background(), id, dir, f, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true})
	}

	if err := b.commit(cacheKey, hook); err != nil {
		return nil, createException(m, err.Error())
	}

	return nil, nil
}
