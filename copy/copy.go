package copy

import (
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/pkg/term"
	"github.com/erikh/box/logger"
	"github.com/fatih/color"
)

// NoTTY turns the progress meters off
var NoTTY bool

// NoOut turns copy output off entirely
var NoOut bool

const (
	megaByte   = float64(1024 * 1024)
	readerSize = 65536
	interval   = 10 * time.Millisecond
)

// WithProgress implements io.Copy with a buffered reader, then measures
// progress throughout the copy process. The buffer is set at a reasonable size
// for reasonable performance. On error, if io.EOF is not returned then the
// error is returned. Otherwise, it is nil.
func WithProgress(writer io.Writer, reader io.Reader, logger *logger.Logger, prefix string) error {
	var printed bool

	defer color.Unset()
	defer func() {
		if printed && !NoOut && !NoTTY {
			fmt.Println()
		}
	}()

	// if there is no terminal, this will be non-nil; we will not print progress
	// below if this is the case.
	_, termErr := term.GetWinsize(0)

	count := float64(0)
	buf := make([]byte, readerSize)
	t := time.Now()
	for {
		rn, rerr := reader.Read(buf)
		if rerr != nil && rerr != io.EOF {
			return rerr
		}

		count += float64(rn)

		if termErr == nil && !NoOut && !NoTTY && time.Since(t) > interval {
			printed = true
			logger.Progress(prefix, count/megaByte)
			t = time.Now()
		}

		_, werr := writer.Write(buf[:rn])
		if werr != nil && werr != io.EOF {
			return werr
		}

		if rerr == io.EOF || rn == 0 {
			return nil
		}
	}
}
