package copy

import (
	"bufio"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/pkg/term"
	"github.com/fatih/color"
)

// NoTTY turns the progress meters off
var NoTTY bool

// NoOut turns copy output off entirely
var NoOut bool

const megaByte = float64(1024 * 1024)
const readerSize = 4096

// WithProgress implements io.Copy with a buffered reader, then measures
// progress throughout the copy process. The buffer is set at a reasonable size
// for reasonable performance. On error, if io.EOF is not returned then the
// error is returned. Otherwise, it is nil.
func WithProgress(writer io.Writer, reader io.Reader, prefix string) error {
	defer color.Unset()

	var printed bool

	defer func() {
		if printed {
			fmt.Println()
		}
	}()

	wsz, _ := term.GetWinsize(0)

	rd := bufio.NewReaderSize(reader, readerSize)

	count := float64(0)
	buf := make([]byte, readerSize)
	t := time.Now()
	for {
		rn, err := rd.Read(buf)
		count += float64(rn)

		if err == io.EOF {
			if rn > 0 {
				goto write
			} else {
				return nil
			}
		}
		if err != nil {
			return err
		}

		if NoOut {
			goto write
		}

		if time.Since(t) > 100*time.Millisecond && !NoTTY && wsz.Width != 0 {
			printed = true
			fmt.Print("\r")

			mbs := fmt.Sprintf("%.02fMB", count/megaByte)

			color.New(color.FgWhite, color.Bold).Printf("+++ ")

			justifiedWidth := int(wsz.Width) - len(mbs) - 9
			if justifiedWidth < 0 {
				goto write
			}

			if len(prefix) > int(justifiedWidth) {
				prefix = prefix[:int(justifiedWidth)] + "..."
			}

			color.New(color.FgRed, color.Bold).Printf("%s: ", prefix)
			color.New(color.FgWhite).Print(mbs)

			t = time.Now()
		}

	write:
		_, werr := writer.Write(buf[:rn])
		if werr != nil {
			return werr
		}

		if err == io.EOF {
			return nil
		}
	}
}
