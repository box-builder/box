package builder

import (
	"errors"
	"fmt"

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
