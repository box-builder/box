package overmount

import (
	"os"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/pkg/errors"
)

func edit(lockfile string, mutex *sync.Mutex, editFunc func() error) (retErr error) {
	mutex.Lock()
	defer mutex.Unlock()

	f, err := os.Create(lockfile)
	if err != nil {
		return err
	}

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return err
	}

	defer func() {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_UN); err != nil {
			retErr = errors.Wrap(retErr, err.Error())
		}
		f.Close()
	}()

	return editFunc()
}

// checkDir validates the directory is not a symlink and exists.
func checkDir(path string, wrapErr error) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0700); err != nil {
				return errors.Wrapf(wrapErr, "unable to mkdir: %v", err.Error())
			}
			return nil
		}
		return err
	}

	if !fi.IsDir() {
		return errors.Wrap(wrapErr, "not a directory")
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		// here we attempt to remove a whole class of potential bugs.
		return errors.Wrap(wrapErr, "cannot operate on a symlink")
	}

	return nil
}
