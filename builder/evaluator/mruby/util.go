package mruby

import (
	"fmt"

	gm "github.com/mitchellh/go-mruby"
	"github.com/pkg/errors"
)

func (m *MRuby) createException(origerr error) gm.Value {
	if origerr == nil {
		return nil
	}

	val, err := m.mrb.Class("Exception", nil).New(gm.String(origerr.Error()))
	if err != nil {
		panic(fmt.Sprintf("could not construct exception for return: %v", err))
	}

	return val
}

func iterateRubyHash(arg *gm.MrbValue, fn func(*gm.MrbValue, *gm.MrbValue) error) error {
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

func checkArgs(args []*gm.MrbValue, l int) error {
	if len(args) != l {
		return errors.Errorf("Expected %d arg(s), got %d", l, len(args))
	}

	return nil
}

func extractStringOrArray(m *gm.Mrb, args []*gm.MrbValue) ([]*gm.MrbValue, error) {
	if len(args) == 1 && args[0].Type() == gm.TypeArray {
		var values []*gm.MrbValue
		ary := args[0].Array()
		for i := 0; i < ary.Len(); i++ {
			val, err := ary.Get(i)
			if err != nil {
				return nil, err
			}
			values = append(values, val)
		}

		return values, nil
	}

	return args, nil
}

func extractStringArgs(args []*gm.MrbValue) []string {
	strArgs := []string{}

	for _, arg := range args {
		if arg != nil && arg.Type() != gm.TypeProc {
			strArgs = append(strArgs, arg.String())
		}
	}

	return strArgs
}

// coerceHash converts a mruby hash into map[string]interface{}. The caller is
// expected to be responsible for coercing sub-types of the map out. Note that
// this will attempt to unwind arrays and hashes that are stored within it.
//
// If the value is not hash or array, it is assumed to be a type convertable to
// string.
func coerceHash(hash *gm.Hash) (map[string]interface{}, error) {
	retval := map[string]interface{}{}

	keys, err := hash.Keys()
	if err != nil {
		return nil, err
	}

	keyary := keys.Array()
	for i := 0; i < keyary.Len(); i++ {
		arg, err := keyary.Get(i)
		if err != nil {
			return nil, err
		}

		typ := arg.Type()

		if typ == gm.TypeArray || typ == gm.TypeHash {
			return nil, errors.New("invalid type in key")
		}

		val, err := hash.Get(arg)
		if err != nil {
			return nil, err
		}

		typ = val.Type()

		// XXX due to the recursion here we may have an endless stack if the data
		// is apprpropriately configured.
		switch typ {
		case gm.TypeArray:
			ret, err := coerceArray(val.Array())
			if err != nil {
				return nil, err
			}

			retval[arg.String()] = ret
		case gm.TypeHash:
			ret, err := coerceHash(val.Hash())
			if err != nil {
				return nil, err
			}

			retval[arg.String()] = ret
		default:
			retval[arg.String()] = val.String()
		}
	}

	return retval, nil
}

// coerceArray converts a mruby array into interface{}, which encapsulates a
// []interface{}. This makes it a little easier to embed in the
// map[string]interface{} yielded by coerceHash. Note that any inner hashes and
// arrays will also be converted.
//
// If the value is not hash or array, it is assumed to be a type convertable to
// string.
func coerceArray(array *gm.Array) (interface{}, error) {
	retval := []interface{}{}

	for i := 0; i < array.Len(); i++ {
		val, err := array.Get(i)
		if err != nil {
			return nil, err
		}
		switch val.Type() {
		case gm.TypeArray:
			ary, err := coerceArray(val.Array())
			if err != nil {
				return nil, err
			}
			retval = append(retval, ary)
		case gm.TypeHash:
			ret, err := coerceHash(val.Hash())
			if err != nil {
				return nil, err
			}

			retval = append(retval, ret)
		default:
			retval = append(retval, val.String())
		}
	}

	return retval, nil
}
