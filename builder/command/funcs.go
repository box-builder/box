package command

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
)

// Var corresponds to the `var` func.
func (i *Interpreter) Var(key string) (string, error) {
	val, ok := i.vars[key]
	if !ok {
		return "", fmt.Errorf("value for key %q does not exist", key)
	}

	fmt.Println(val)

	return val, nil
}

// Save corresponds to the `save` func.
func (i *Interpreter) Save(file, kind, tag string) error {
	if tag != "" {
		if err := i.exec.Image().Tag(tag); err != nil {
			return err
		}
	}

	if file != "" {
		if tag == "" {
			// since OCI images always require a tag, we need to set it to something
			// if nothing's provided; this will be the filename's basename minus the
			// extension.
			tag = strings.TrimSuffix(path.Base(file), path.Ext(file))
		}
		return i.exec.Image().Save(file, kind, tag)
	}

	return nil
}

// GetEnv gets a value from the local environment.
func (i *Interpreter) GetEnv(arg string) string {
	return os.Getenv(arg)
}

// Read reads a file from inside the container, and returns its contents.
func (i *Interpreter) Read(filename string) (string, error) {
	content, err := i.exec.CopyOneFileFromContainer(filename)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (i *Interpreter) getID(id, filename, typeName string) (string, error) {
	content, err := i.exec.CopyOneFileFromContainer(filename)
	if err != nil {
		return "", err
	}

	entries := strings.Split(string(content), "\n")
	for _, ent := range entries {
		parts := strings.Split(ent, ":")
		if parts[0] == id {
			return parts[2], nil
		}
	}

	return "", errors.Errorf("could not find %s %q", typeName, id)
}

// GetUID gets the UID for a user inside the container image currently in process.
func (i *Interpreter) GetUID(id string) (string, error) {
	return i.getID(id, "/etc/passwd", "user")
}

// GetGID gets the GID for a group inside the container image currently in process.
func (i *Interpreter) GetGID(id string) (string, error) {
	return i.getID(id, "/etc/group", "group")
}

// Skip is the `skip` function.
func (i *Interpreter) Skip(run func() error) error {
	i.exec.Layers().SetSkipLayers(true)
	defer func() { i.exec.Layers().SetSkipLayers(false) }()

	return run()
}
