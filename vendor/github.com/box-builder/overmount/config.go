package overmount

import (
	"time"
)

// ImageConfig is a portable, non-standard format used by overmount for the
// generation of other configuration formats used in images. It is an attempt
// to be abstract from the formats themselves. It is intentionally flat to
// avoid merging problems with newer editions of overmount.
//
// NOTE: some portions of this code are taken from docker/docker and opencontainers/image-spec.
type ImageConfig struct {
	// ID is a unique 64 character identifier of the image
	ID string `json:"id,omitempty"`

	// Parent is the ID of the parent image
	Parent string `json:"parent,omitempty"`

	// Comment is the commit message that was set when committing the image
	Comment string `json:"comment,omitempty"`

	// Created is the timestamp at which the image was created
	Created time.Time `json:"created"`

	// Container is the id of the container used to commit
	Container string `json:"container,omitempty"`

	// ContainerConfig is the configuration of the container that is committed into the image
	ContainerConfig interface{} `json:"container_config,omitempty"`

	// DockerVersion specifies the version of Docker that was used to build the image
	DockerVersion string `json:"docker_version,omitempty"`

	// Author is the name of the author that was specified when committing the image
	Author string `json:"author,omitempty"`

	// Architecture is the hardware that the image is built and runs on
	Architecture string `json:"architecture,omitempty"`

	// OS is the operating system used to build and run the image
	OS string `json:"os,omitempty"`

	// User defines the username or UID which the process in the container should run as.
	User string `json:"user,omitempty"`

	// ExposedPorts a set of ports to expose from a container running this image.
	ExposedPorts map[string]struct{} `json:"exposed_ports,omitempty"`

	// Env is a list of environment variables to be used in a container.
	Env []string `json:"env,omitempty"`

	// Entrypoint defines a list of arguments to use as the command to execute when the container starts.
	Entrypoint []string `json:"entrypoint,omitempty"`

	// Cmd defines the default arguments to the entrypoint of the container.
	Cmd []string `json:"cmd,omitempty"`

	// Volumes is a set of directories which should be created as data volumes in a container running this image.
	Volumes map[string]struct{} `json:"volumes,omitempty"`

	// WorkingDir sets the current working directory of the entrypoint process in the container.
	WorkingDir string `json:"working_dir,omitempty"`

	// Labels contains arbitrary metadata for the container.
	Labels map[string]string `json:"labels,omitempty"`

	// StopSignal contains the system call signal that will be sent to the container to exit.
	StopSignal string `json:"stopsignal,omitempty"`
}
