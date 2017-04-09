package overmount

import (
	"fmt"
	"os"

	"github.com/pkg/errors"

	"golang.org/x/sys/unix"
)

func forceUnmount(target string) (retErr error) {
	defer func() {
		if retErr == nil {
			retErr = os.RemoveAll(target)
		}
	}()

	return unix.Unmount(target, 0) // showing restraint
}

// makeMountOptions makes the lower,upper,work filesystem options.
func (m *Mount) makeMountOptions() (string, error) {
	if m.lower == "" {
		return "", errors.Wrap(ErrMountCannotProceed, "No lower dir specified (only one layer?)")
	}

	return fmt.Sprintf("upperdir=%s,lowerdir=%s,workdir=%s", m.upper, m.lower, m.work), nil
}

// Open an overlay mount at (*Mount).Target; returns any errors.
func (m *Mount) Open() error {
	opts, err := m.makeMountOptions()
	if err != nil {
		return err
	}

	if err := unix.Mount("overlay", m.target, "overlay", 0, opts); err != nil {
		return err
	}

	m.mounted = true
	return nil
}

// Close a mount and remove the work directory. The target directory is left untouched.
func (m *Mount) Close() error {
	if err := unix.Unmount(m.target, 0); err != nil {
		return err
	}

	if err := os.RemoveAll(m.work); err != nil {
		return err
	}

	m.mounted = false
	return nil
}

// Mounted returns true if the volume is currently mounted.
func (m *Mount) Mounted() bool {
	return m.mounted
}

// Equals compares two mounts to see if they're equivalent
func (m *Mount) Equals(m2 *Mount) bool {
	return m.target == m2.target && m.upper == m2.upper && m.lower == m2.lower && m.work == m2.work
}
