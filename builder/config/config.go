package config

import (
	"time"

	"github.com/docker/docker/api/types/container"
)

// Config is a basic configuration of an image at each step. It is kept in sync
// by commit routines in the executor. Setting properties here will propogate
// them to various image-manipulating commands when needed.
type Config struct {
	Image      string   // Image Identifier, may be different across executors.
	User       string   // the currently configured user for this image.
	WorkDir    string   // the current working directory on entering a container
	Cmd        []string // the secondary execution form, it is provided to images if given to docker run, otherwise this is used.
	Entrypoint []string // the primary execution form, the first arguments and the exec() jumping-off point.
	Env        []string
}

// NewConfig initializes a new configuration.
func NewConfig() *Config {
	return &Config{
		Env:     []string{},
		Cmd:     []string{"/bin/sh"},
		User:    "root",
		WorkDir: "/",
		Image:   "",
	}
}

// ToDocker outputs a docker configuration suitable for running images.
func (c *Config) ToDocker(tty, stdin bool) *container.Config {
	return &container.Config{
		Tty:          tty,
		AttachStderr: true,
		AttachStdout: true,
		AttachStdin:  stdin,
		OpenStdin:    stdin,
		Image:        c.Image,
		Env:          c.Env,
		Entrypoint:   c.Entrypoint,
		Cmd:          c.Cmd,
		User:         c.User,
		WorkingDir:   c.WorkDir,
	}
}

// FromDocker sets *Config properties from a docker *container.Config
func (c *Config) FromDocker(cont *container.Config) {
	c.Image = cont.Image
	c.Env = cont.Env
	c.Entrypoint = cont.Entrypoint
	c.Cmd = cont.Cmd
	c.User = cont.User
	c.WorkDir = cont.WorkingDir
}

// ToImage returns the config as an image manifest.
func (c *Config) ToImage(layers []string) map[string]interface{} {
	fields := map[string]interface{}{}
	fields["config"] = c.ToDocker(false, false)
	fields["created"] = time.Now().Format("2006-01-02T15:04:05Z07:00")
	fields["architecture"] = "amd64"
	fields["os"] = "linux"
	fields["history"] = []map[string]interface{}{{}}
	fields["rootfs"] = map[string]interface{}{
		"diff_ids": layers,
		"type":     "layers",
	}

	return fields
}
