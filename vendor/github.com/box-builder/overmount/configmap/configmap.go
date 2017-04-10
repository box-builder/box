package configmap

import (
	"encoding/json"

	"github.com/box-builder/overmount"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/image"
	"github.com/docker/go-connections/nat"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

func remarshalContainerConfig(cc interface{}) (container.Config, error) {
	content, err := json.Marshal(cc)
	config := container.Config{}
	if err != nil {
		return config, err
	}

	return config, json.Unmarshal(content, &config)
}

func convertFromPortSet(set nat.PortSet) map[string]struct{} {
	myMap := map[string]struct{}{}

	for key := range set {
		myMap[string(key)] = struct{}{}
	}

	return myMap
}

func convertToPortSet(myMap map[string]struct{}) nat.PortSet {
	set := nat.PortSet{}

	for key := range myMap {
		pb, err := nat.ParsePortSpec(key)
		if err != nil {
			continue
		}

		set[pb[0].Port] = struct{}{}
	}

	return set
}

// FromDockerV1 converts a docker image configuration to an overmount one.
func FromDockerV1(img *image.V1Image) *overmount.ImageConfig {
	return &overmount.ImageConfig{
		ID:              img.ID,     // FIXME should be calculated
		Parent:          img.Parent, // FIXME should be calculated
		Comment:         img.Comment,
		Created:         img.Created,
		Container:       img.Container, // FIXME should be overridden
		ContainerConfig: img.ContainerConfig,
		DockerVersion:   img.DockerVersion,
		Author:          img.Author,
		Architecture:    img.Architecture,
		OS:              img.OS,
		User:            img.Config.User,
		ExposedPorts:    convertFromPortSet(nat.PortSet(img.Config.ExposedPorts)),
		Env:             img.Config.Env,
		Entrypoint:      img.Config.Entrypoint,
		Cmd:             img.Config.Cmd,
		Volumes:         img.Config.Volumes,
		WorkingDir:      img.Config.WorkingDir,
		Labels:          img.Config.Labels,
		StopSignal:      img.Config.StopSignal,
	}
}

// ToDockerV1 converts an overmount image configuration to a docker one.
func ToDockerV1(config *overmount.ImageConfig) (*image.V1Image, error) {
	cc, err := remarshalContainerConfig(config.ContainerConfig)
	if err != nil {
		return nil, err
	}

	return &image.V1Image{
		ID:              config.ID,     // FIXME should be calculated
		Parent:          config.Parent, // FIXME should be calculated
		Comment:         config.Comment,
		Created:         config.Created,
		Container:       config.Container, // FIXME should be overridden
		ContainerConfig: cc,
		DockerVersion:   config.DockerVersion,
		Author:          config.Author,
		Architecture:    config.Architecture,
		OS:              config.OS,
		Config: &container.Config{
			User:         config.User,
			ExposedPorts: convertToPortSet(config.ExposedPorts),
			Env:          config.Env,
			Entrypoint:   config.Entrypoint,
			Cmd:          config.Cmd,
			Volumes:      config.Volumes,
			WorkingDir:   config.WorkingDir,
			Labels:       config.Labels,
			StopSignal:   config.StopSignal,
		},
	}, nil
}

// FromOCIV1 converts an oci image configuration to an overmount one.
func FromOCIV1(img *v1.Image) *overmount.ImageConfig {
	return nil
}

// ToOCIV1 converts an overmount image configuration to an OCI one.
func ToOCIV1(config *overmount.ImageConfig) *v1.Image {
	return &v1.Image{
		Created:      &config.Created,
		Author:       config.Author,
		Architecture: config.Architecture,
		OS:           config.OS,
		Config: v1.ImageConfig{
			User:         config.User,
			ExposedPorts: config.ExposedPorts,
			Env:          config.Env,
			Entrypoint:   config.Entrypoint,
			Cmd:          config.Cmd,
			Volumes:      config.Volumes,
			WorkingDir:   config.WorkingDir,
			Labels:       config.Labels,
			StopSignal:   config.StopSignal,
		},
	}
}
