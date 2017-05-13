package mruby

import (
	"io/ioutil"

	gm "github.com/mitchellh/go-mruby"
	"github.com/pkg/errors"
)

type funcDefinition struct {
	fun     funcFunc
	argSpec gm.ArgSpec
}

type funcFunc func(args []*gm.MrbValue, self *gm.MrbValue) (gm.Value, gm.Value)

func (m *MRuby) funcJumpTable() map[string]*funcDefinition {
	return map[string]*funcDefinition{
		"var":    {m.varFunc, gm.ArgsReq(1)},
		"import": {m.importFunc, gm.ArgsReq(1)},
		"save":   {m.saveFunc, gm.ArgsReq(1)},
		"getenv": {m.getenv, gm.ArgsReq(1)},
		"getuid": {m.getuid, gm.ArgsReq(1)},
		"getgid": {m.getgid, gm.ArgsReq(1)},
		"read":   {m.read, gm.ArgsReq(1)},
		"skip":   {m.skip, gm.ArgsNone() | gm.ArgsBlock()},
	}
}

func (m *MRuby) varFunc(args []*gm.MrbValue, self *gm.MrbValue) (gm.Value, gm.Value) {
	if err := checkArgs(args, 1); err != nil {
		return nil, m.createException(err)
	}

	value, err := m.Interp.Var(args[0].String())
	if err != nil {
		return nil, m.createException(err)
	}

	return gm.String(value), nil
}

func (m *MRuby) importFunc(args []*gm.MrbValue, self *gm.MrbValue) (gm.Value, gm.Value) {
	if err := checkArgs(args, 1); err != nil {
		return nil, m.createException(err)
	}

	content, err := ioutil.ReadFile(args[0].String())
	if err != nil {
		return nil, m.createException(err)
	}

	if err := m.RunScript(string(content)); err != nil {
		return nil, m.createException(err)
	}

	return gm.String(m.Result().Value), nil
}

func (m *MRuby) saveFunc(args []*gm.MrbValue, self *gm.MrbValue) (gm.Value, gm.Value) {
	if err := checkArgs(args, 1); err != nil {
		return nil, m.createException(err)
	}

	var tag, file, kind string

	if keys, err := args[0].Hash().Keys(); err != nil || keys.Array().Len() == 0 {
		return nil, m.createException(errors.New("save must be called with parameters"))
	}

	err := iterateRubyHash(args[0], func(key, value *gm.MrbValue) error {
		switch key.String() {
		case "tag":
			tag = value.String()
		case "file":
			file = value.String()
		case "kind":
			kind = value.String()
		default:
			return errors.Errorf("%q is not a valid parameter to the save function", key.String())
		}

		return nil
	})
	if err != nil {
		return nil, m.createException(err)
	}

	return nil, m.createException(m.Interp.Save(file, kind, tag))
}

func (m *MRuby) getenv(args []*gm.MrbValue, self *gm.MrbValue) (gm.Value, gm.Value) {
	if err := checkArgs(args, 1); err != nil {
		return nil, m.createException(err)
	}

	return gm.String(m.Interp.GetEnv(args[0].String())), nil
}

func (m *MRuby) getuid(args []*gm.MrbValue, self *gm.MrbValue) (gm.Value, gm.Value) {
	if err := checkArgs(args, 1); err != nil {
		return nil, m.createException(err)
	}

	res, err := m.Interp.GetUID(args[0].String())
	return gm.String(res), m.createException(err)
}

func (m *MRuby) getgid(args []*gm.MrbValue, self *gm.MrbValue) (gm.Value, gm.Value) {
	if err := checkArgs(args, 1); err != nil {
		return nil, m.createException(err)
	}

	res, err := m.Interp.GetGID(args[0].String())
	return gm.String(res), m.createException(err)
}

func (m *MRuby) read(args []*gm.MrbValue, self *gm.MrbValue) (gm.Value, gm.Value) {
	if err := checkArgs(args, 1); err != nil {
		return nil, m.createException(err)
	}

	res, err := m.Interp.Read(args[0].String())
	return gm.String(res), m.createException(err)
}

func (m *MRuby) skip(args []*gm.MrbValue, self *gm.MrbValue) (gm.Value, gm.Value) {
	if err := checkArgs(args, 1); err != nil {
		return nil, m.createException(err)
	}

	return nil, m.createException(m.Interp.Skip(func() error {
		_, err := m.mrb.Yield(args[0])
		return err
	}))
}
