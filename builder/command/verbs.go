package command

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// Label corresponds to the `label` verb.
func (i *Interpreter) Label(labelMap map[string]string) error {
	for key, value := range labelMap {
		i.exec.Config().Labels[key] = value
	}

	return i.makeLayer(false)
}

// Debug corresponds to the `debug` verb.
func (i *Interpreter) Debug(shell string) error {
	i.exec.SetStdin(true)
	defer i.exec.SetStdin(false)

	i.exec.Config().TemporaryCommand([]string{}, []string{shell})

	return i.makeLayer(true)
}

// SetExec corresponds to the `set_exec` verb.
func (i *Interpreter) SetExec(execTargets map[string][]string) error {
	for key, cmds := range execTargets {
		switch key {
		case "entrypoint":
			i.exec.Config().Entrypoint.Image = cmds
		case "cmd":
			i.exec.Config().Cmd.Image = cmds
		default:
			return fmt.Errorf("set_exec only accepts cmd and entrypoint as keys")
		}
	}

	return i.makeLayer(false)
}

// WorkDir is the `workdir` verb.
func (i *Interpreter) WorkDir(dir string) error {
	if !path.IsAbs(dir) {
		return errors.Errorf("path %q is not absolute in workdir", dir)
	}

	i.exec.Config().WorkDir.Image = dir

	return i.makeLayer(false)
}

// User is the `user` verb.
func (i *Interpreter) User(username string) error {
	i.exec.Config().User.Image = username
	return i.makeLayer(false)
}

// Tag is the `tag` verb.
func (i *Interpreter) Tag(name string) error {
	if err := i.exec.Commit("", nil); err != nil {
		return err
	}
	return i.exec.Image().Tag(name)
}

// Entrypoint is the `entrypoint` verb.
func (i *Interpreter) Entrypoint(stringArgs []string) error {
	i.exec.Config().Entrypoint.Image = stringArgs
	return i.makeLayer(false)
}

// WithUser is the `with_user` verb.
func (i *Interpreter) WithUser(username string, run func() error) error {
	i.exec.Config().User.Temporary = username
	defer func() { i.exec.Config().User.Temporary = "" }()

	return run()
}

// Inside is the `inside` verb.
func (i *Interpreter) Inside(p string, run func() error) error {
	var currentDir string

	if !path.IsAbs(p) {
		currentDir = i.exec.Config().WorkDir.Temporary
		if currentDir == "" {
			currentDir = i.exec.Config().WorkDir.Image
		}

		if currentDir != "" {
			currentDir = path.Join(currentDir, p)
		} else {
			currentDir = p
		}
	}

	if currentDir == "" {
		currentDir = p
	}

	if !path.IsAbs(filepath.Clean(currentDir)) {
		return errors.Errorf("path %q is not absolute in workdir", p)
	}

	i.exec.Config().WorkDir.Temporary = currentDir
	defer func() { i.exec.Config().WorkDir.Temporary = "" }()

	return run()
}

// Env corresponds to the `env` verb.
func (i *Interpreter) Env(env map[string]string) error {
	newEnv := map[string]string{}

	for _, part := range i.exec.Config().Env {
		parts := strings.SplitN(part, "=", 2)
		newEnv[parts[0]] = parts[1]
	}

	for key, value := range env {
		newEnv[key] = value
	}

	rebuiltEnv := []string{}

	for key, value := range newEnv {
		rebuiltEnv = append(rebuiltEnv, fmt.Sprintf("%s=%s", key, value))
	}

	i.exec.Config().Env = rebuiltEnv

	return i.makeLayer(false)
}

// Cmd corresponds to the `cmd` verb.
func (i *Interpreter) Cmd(cmds []string) error {
	i.exec.Config().Cmd.Image = cmds
	return i.makeLayer(false)
}
