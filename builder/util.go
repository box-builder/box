package builder

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	mruby "github.com/mitchellh/go-mruby"
)

func createException(m *mruby.Mrb, msg string) mruby.Value {
	val, err := m.Class("Exception", nil).New(mruby.String(msg))
	if err != nil {
		panic(fmt.Sprintf("could not construct exception for return: %v", err))
	}

	return val
}

func extractStringArgs(args []*mruby.MrbValue) []string {
	strArgs := []string{}

	for _, arg := range args {
		if arg != nil && arg.Type() != mruby.TypeProc {
			strArgs = append(strArgs, arg.String())
		}
	}

	return strArgs
}

func iterateRubyHash(arg *mruby.MrbValue, fn func(*mruby.MrbValue, *mruby.MrbValue) error) error {
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

func checkArgs(args []*mruby.MrbValue, l int) error {
	if len(args) != l {
		return fmt.Errorf("Expected %d arg, got %d", l, len(args))
	}

	return nil
}

func checkImage(b *Builder) error {
	if b.ImageID() != "" {
		return nil
	}

	return errors.New("from has not been called, no image can be used for this operation")
}

func standardCheck(b *Builder, args []*mruby.MrbValue, l int) error {
	if err := checkArgs(args, l); err != nil {
		return err
	}

	return checkImage(b)
}

// coerceArray converts a mruby array into interface{}, which encapsulates a
// []interface{}. This makes it a little easier to embed in the
// map[string]interface{} yielded by coerceHash. Note that any inner hashes and
// arrays will also be converted.
//
// If the value is not hash or array, it is assumed to be a type convertable to
// string.
func coerceArray(array *mruby.Array) (interface{}, error) {
	retval := []interface{}{}

	for i := 0; i < array.Len(); i++ {
		val, err := array.Get(i)
		if err != nil {
			return nil, err
		}
		switch val.Type() {
		case mruby.TypeArray:
			ary, err := coerceArray(val.Array())
			if err != nil {
				return nil, err
			}
			retval = append(retval, ary)
		case mruby.TypeHash:
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

// coerceHash converts a mruby hash into map[string]interface{}. The caller is
// expected to be responsible for coercing sub-types of the map out. Note that
// this will attempt to unwind arrays and hashes that are stored within it.
//
// If the value is not hash or array, it is assumed to be a type convertable to
// string.
func coerceHash(hash *mruby.Hash) (map[string]interface{}, error) {
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

		if typ == mruby.TypeArray || typ == mruby.TypeHash {
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
		case mruby.TypeArray:
			ret, err := coerceArray(val.Array())
			if err != nil {
				return nil, err
			}

			retval[arg.String()] = ret
		case mruby.TypeHash:
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

// readLines reads lines from the file and returns them as []string and any error.
func readLines(filename string) ([]string, error) {
	di, err := os.Open(filename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		content, err := ioutil.ReadAll(di)
		di.Close()
		if err != nil {
			return nil, err
		}

		return strings.Split(strings.TrimSpace(string(content)), "\n"), nil
	}

	return []string{}, nil
}

func readDockerIgnore() ([]string, error) {
	return readLines(".dockerignore")
}

// interfaceListToString converts []interface{} to []string by way of interface{}
func interfaceListToString(list interface{}) ([]string, error) {
	strList := []string{}

	ifList, ok := list.([]interface{})
	if ok {
		for _, item := range ifList {
			str, ok := item.(string)
			if !ok {
				return nil, errors.New("list is not a list of strings")
			}

			strList = append(strList, str)
		}
	} else {
		return nil, errors.New("object is not a list")
	}

	return strList, nil
}
