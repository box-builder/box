package config

import (
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
)

// CommandState is an abstraction to facilitate Cmd and Entrypoint handling and
// inheritance. It is split so that the docker configuration can use it as
// needed to provide temporary build-time instructions vs. image-level
// instructions. See ToDocker() for code that uses this struct.
type CommandState struct {
	Temporary []string
	Image     []string
}

// Config is a basic configuration of an image at each step. It is kept in sync
// by commit routines in the executor. Setting properties here will propogate
// them to various image-manipulating commands when needed.
type Config struct {
	Image      string       // Image Identifier, may be different across executors.
	User       string       // the currently configured user for this image.
	WorkDir    string       // the current working directory on entering a container
	Cmd        CommandState // the secondary execution form, it is provided to images if given to docker run, otherwise this is used.
	Entrypoint CommandState // the primary execution form, the first arguments and the exec() jumping-off point.
	Env        []string
}

// NewConfig initializes a new configuration.
func NewConfig() *Config {
	return &Config{
		Env:     []string{},
		User:    "root",
		WorkDir: "/",
		Image:   "",
	}
}

// TemporaryCommand is used to manage run and debug statements and similar
// effects where the results should not be recorded in the committed container.
func (c *Config) TemporaryCommand(entrypoint, cmd []string) {
	c.Entrypoint.Temporary = entrypoint
	c.Cmd.Temporary = cmd
}

// ToDocker outputs a docker configuration suitable for running images.
func (c *Config) ToDocker(temporary, tty, stdin bool) *container.Config {
	var cmd, entrypoint []string

	if temporary {
		cmd = c.Cmd.Temporary
		entrypoint = c.Entrypoint.Temporary
	} else {
		cmd = c.Cmd.Image
		entrypoint = c.Entrypoint.Image
	}

	if len(cmd) == 0 && len(entrypoint) == 0 {
		cmd = []string{"/bin/sh"}
	}

	return &container.Config{
		Tty:          tty,
		AttachStderr: true,
		AttachStdout: true,
		AttachStdin:  stdin,
		OpenStdin:    stdin,
		Image:        c.Image,
		Env:          c.Env,
		Entrypoint:   entrypoint,
		Cmd:          cmd,
		User:         c.User,
		WorkingDir:   c.WorkDir,
	}
}

// FromDocker sets *Config properties from a docker *container.Config
func (c *Config) FromDocker(cont *container.Config) {
	c.Image = cont.Image
	c.Env = cont.Env
	c.Entrypoint.Image = cont.Entrypoint
	c.Cmd.Image = cont.Cmd
	c.User = cont.User
	c.WorkDir = cont.WorkingDir
}

// ToImage returns the config as an image manifest.
func (c *Config) ToImage(layers []string) map[string]interface{} {
	shaLayers := []string{}
	for _, layer := range layers {
		shaLayers = append(shaLayers, fmt.Sprintf("sha256:%v", layer))
	}

	fields := map[string]interface{}{}
	fields["config"] = c.ToDocker(false, false, false)
	fields["created"] = time.Now().Format("2006-01-02T15:04:05Z07:00")
	fields["architecture"] = "amd64"
	fields["os"] = "linux"
	fields["history"] = []map[string]interface{}{{}}
	fields["rootfs"] = map[string]interface{}{
		"diff_ids": shaLayers,
		"type":     "layers",
	}

	return fields
}
