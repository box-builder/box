package tar

import (
	"io"

	"github.com/docker/docker/pkg/archive"
)

// Unarchive unpacks the reader into the destination directory. An error is
// yielded if this operation cannot occur or failed.
func Unarchive(reader io.Reader, dest string) error {
	options := &archive.TarOptions{WhiteoutFormat: archive.OverlayWhiteoutFormat}
	return archive.Unpack(reader, dest, options)
}
