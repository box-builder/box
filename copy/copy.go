package copy

import (
	"fmt"
	"io"
	"time"

	"github.com/box-builder/box/logger"
	"github.com/box-builder/progress"
	"github.com/docker/docker/pkg/term"
	"github.com/fatih/color"
)

// NoTTY turns the progress meters off
var NoTTY bool

// NoOut turns copy output off entirely
var NoOut bool

const (
	megaByte = float64(1024 * 1024)
	interval = 10 * time.Millisecond
)

// WithProgress implements io.Copy with a buffered reader, then measures
// progress throughout the copy process. The buffer is set at a reasonable size
// for reasonable performance. On error, if io.EOF is not returned then the
// error is returned. Otherwise, it is nil.
func WithProgress(writer io.Writer, reader io.Reader, logger *logger.Logger, prefix string) error {
	var printed bool

	defer color.Unset()
	defer func(printed *bool) {
		if *printed && !NoOut && !NoTTY {
			fmt.Println()
		}
	}(&printed)

	endChan := make(chan struct{})

	// if there is no terminal, this will be non-nil; we will not print progress
	// below if this is the case.
	if _, termErr := term.GetWinsize(0); termErr == nil && !NoOut && !NoTTY {
		pr := progress.NewReader(prefix, reader, interval)
		count := float64(0)

		go func(pr *progress.Reader, printed *bool) {
			<-pr.C
			for tick := range pr.C {
				*printed = true
				count += float64(tick.Value)
				logger.Progress(tick.Artifact, count/megaByte)
			}
			close(endChan)
		}(pr, &printed)

		if _, err := io.Copy(writer, pr); err != nil {
			return err
		}
		close(pr.C)
		<-endChan
		return nil
	}

	_, err := io.Copy(writer, reader)
	return err
}
