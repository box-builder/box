package mruby

import (
	"fmt"

	gm "github.com/mitchellh/go-mruby"
	"github.com/pkg/errors"
)

// verbDefinition is a jump table definition used for programming the DSL into the
// mruby interpreter.
type verbDefinition struct {
	verbFunc verbFunc
	argSpec  gm.ArgSpec
}

// verbFunc is a builder DSL function used to interact with docker.
type verbFunc func(args []*gm.MrbValue, self *gm.MrbValue) error

// verbJumpTable is the dispatch instructions sent to the builder at preparation time.
func (m *MRuby) verbJumpTable() map[string]*verbDefinition {
	return map[string]*verbDefinition{
		"after":      {m.after, gm.ArgsBlock()},
		"label":      {m.label, gm.ArgsReq(1)},
		"debug":      {m.debug, gm.ArgsNone()},
		"set_exec":   {m.setExec, gm.ArgsReq(1)},
		"workdir":    {m.workdir, gm.ArgsReq(1)},
		"user":       {m.user, gm.ArgsReq(1)},
		"flatten":    {m.flatten, gm.ArgsNone()},
		"tag":        {m.tag, gm.ArgsReq(1)},
		"entrypoint": {m.entrypoint, gm.ArgsAny()},
		"from":       {m.from, gm.ArgsReq(1)},
		"with_user":  {m.withUser, gm.ArgsBlock() | gm.ArgsReq(2)},
		"inside":     {m.inside, gm.ArgsBlock() | gm.ArgsReq(2)},
		"env":        {m.env, gm.ArgsAny()},
		"cmd":        {m.cmd, gm.ArgsAny()},
		"run":        {m.run, gm.ArgsAny()},
		"copy":       {m.doCopy, gm.ArgsReq(2)}, // see builder/copy.go
	}
}

func (m *MRuby) after(args []*gm.MrbValue, self *gm.MrbValue) error {
	if len(args) != 1 {
		return errors.New("invalid args to after")
	}

	m.afterFunc = args[0]

	return nil
}

func (m *MRuby) label(args []*gm.MrbValue, self *gm.MrbValue) error {
	if len(args) != 1 {
		return errors.New("label error: please supply a hash for the labels")
	}

	labels := map[string]string{}

	iterateRubyHash(args[0], func(key, value *gm.MrbValue) error {
		labels[key.String()] = value.String()
		return nil
	})

	return m.Interp.Label(labels)
}

func (m *MRuby) debug(args []*gm.MrbValue, self *gm.MrbValue) error {
	var shell string

	if len(args) > 0 {
		shell = args[0].String()
	} else {
		shell = "/bin/bash"
	}

	return m.Interp.Debug(shell)
}

func (m *MRuby) setExec(args []*gm.MrbValue, self *gm.MrbValue) error {
	if err := checkArgs(args, 1); err != nil {
		return err
	}

	setArgs := map[string][]string{}

	err := iterateRubyHash(args[0], func(key, value *gm.MrbValue) error {
		if value.Type() != gm.TypeArray {
			return fmt.Errorf("Value for key %q is not array, must be array", key.String())
		}

		strArgs := []string{}
		a := value.Array()

		for i := 0; i < a.Len(); i++ {
			val, err := a.Get(i)
			if err != nil {
				return err
			}
			strArgs = append(strArgs, val.String())
		}

		setArgs[key.String()] = strArgs
		return nil
	})

	if err != nil {
		return err
	}

	return m.Interp.SetExec(setArgs)
}

func (m *MRuby) workdir(args []*gm.MrbValue, self *gm.MrbValue) error {
	if err := checkArgs(args, 1); err != nil {
		return err
	}

	return m.Interp.WorkDir(args[0].String())
}

func (m *MRuby) user(args []*gm.MrbValue, self *gm.MrbValue) error {
	if err := checkArgs(args, 1); err != nil {
		return err
	}

	return m.Interp.User(args[0].String())
}

func (m *MRuby) flatten(args []*gm.MrbValue, self *gm.MrbValue) error {
	return m.Interp.Flatten()
}

func (m *MRuby) tag(args []*gm.MrbValue, self *gm.MrbValue) error {
	if err := checkArgs(args, 1); err != nil {
		return err
	}

	return m.Interp.Tag(args[0].String())
}

func (m *MRuby) entrypoint(args []*gm.MrbValue, self *gm.MrbValue) error {
	values, err := extractStringOrArray(m.mrb, args)
	if err != nil {
		return err
	}

	stringArgs := extractStringArgs(values)
	if len(stringArgs) == 0 {
		stringArgs = nil
	}

	return m.Interp.Entrypoint(stringArgs)
}

func (m *MRuby) from(args []*gm.MrbValue, self *gm.MrbValue) error {
	if err := checkArgs(args, 1); err != nil {
		return err
	}

	return m.Interp.From(args[0].String())
}

func (m *MRuby) withUser(args []*gm.MrbValue, self *gm.MrbValue) error {
	if err := checkArgs(args, 2); err != nil {
		return err
	}

	if args[1].Type() != gm.TypeProc {
		return errors.Errorf("Arg %q was not block!", args[1].String())
	}

	return m.Interp.WithUser(args[0].String(), func() error {
		_, err := m.mrb.Yield(args[1], args[0])
		return err
	})
}

func (m *MRuby) inside(args []*gm.MrbValue, self *gm.MrbValue) error {
	if err := checkArgs(args, 2); err != nil {
		return err
	}

	if args[1].Type() != gm.TypeProc {
		return errors.Errorf("Arg %q was not block!", args[1].String())
	}

	return m.Interp.Inside(args[0].String(), func() error {
		_, err := m.mrb.Yield(args[1], args[0])
		return err
	})
}

func (m *MRuby) env(args []*gm.MrbValue, self *gm.MrbValue) error {
	if err := checkArgs(args, 1); err != nil {
		return err
	}

	newEnv := map[string]string{}

	err := iterateRubyHash(args[0], func(key, value *gm.MrbValue) error {
		newEnv[key.String()] = value.String()
		return nil
	})
	if err != nil {
		return err
	}

	return m.Interp.Env(newEnv)
}

func (m *MRuby) cmd(args []*gm.MrbValue, self *gm.MrbValue) error {
	values, err := extractStringOrArray(m.mrb, args)
	if err != nil {
		return err
	}

	stringArgs := extractStringArgs(values)
	if len(stringArgs) == 0 {
		stringArgs = nil
	}

	return m.Interp.Cmd(stringArgs)
}

func (m *MRuby) run(args []*gm.MrbValue, self *gm.MrbValue) error {
	if len(args) < 1 {
		return errors.New("no command to run in run statement")
	} else if args[0].Type() != gm.TypeString {
		return errors.New("no command to run in run statement")
	}

	output := true

	if len(args) > 1 {
		if args[1].Type() == gm.TypeHash {
			hash, err := coerceHash(args[1].Hash())
			if err != nil {
				return err
			}

			outstr, ok := hash["output"].(string)
			if ok && outstr == "false" {
				output = false
			}
		} else {
			return errors.Errorf("invalid argument %q for run statement", args[1].String())
		}
	}

	return m.Interp.Run(args[0].String(), output)
}
