package command

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/box-builder/box/copy"
	"github.com/box-builder/box/signal"
)

// Flatten implements `flatten`
func (i *Interpreter) Flatten() error {
	id, err := i.exec.Create()
	if err != nil {
		return err
	}

	defer i.exec.Destroy(id)

	rc, size, err := i.exec.CopyFromContainer(id, "/")
	if err != nil {
		return err
	}

	f, err := ioutil.TempFile("", "box-flatten.")
	if err != nil {
		return err
	}

	signal.Handler.AddFile(f.Name())
	defer signal.Handler.RemoveFile(f.Name())
	defer os.Remove(f.Name())

	if err := copy.WithProgress(f, rc, i.globals.Logger, "Downloading image contents to host"); err != nil && err != io.EOF {
		f.Close()
		return err
	}
	f.Close()

	f, err = os.Open(f.Name())
	if err != nil {
		return err
	}

	defer f.Close()

	return i.exec.Image().Flatten(id, size, f)
}
