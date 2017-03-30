// Package progress - progress meters through a channel, wrapping an io.Reader
//
// Example:
//
//
// func progress(reader io.Reader) {
// 	r = NewReader("an_file.tar", reader, 100*time.Millisecond)
// 	defer r.Close()
//
// 	var count uint64
// 	// This goroutine consumes the channel and concatenates the count to display
// 	// the total as it increases.
// 	go func(r *Reader) {
// 		for tick := range r.C {
// 			count += tick.Value
// 			fmt.Printf("\r%d", count)
// 		}
//
// 		fmt.Println()
// 	}(r)
//
// 	// use r like you normally would; progress will be reported to C every 100
// 	// milliseconds.
// 	if _, err := io.Copy(ioutil.Discard, r); err != nil {
// 		panic(err)
// 	}
// }
//
//
//
package progress

import (
	"io"
	"time"
)

// Tick is the information supplied to the consumer when progress events arrive
// such as reaching an interval. The time of the event is also recorded.
type Tick struct {
	Value    uint64
	Time     time.Time
	Artifact string
}

// Reader is a progress-aware reader that communicates its results over channel
// C. Channel C can then be read for information about the progress of the copy.
type Reader struct {
	C        chan *Tick
	interval time.Duration
	artifact string
	reader   io.Reader
	value    uint64
	lastTime time.Time
}

// NewReader creates a new reader for use, wrapping an io.Reader with a
// reporting duration and artifact information.
func NewReader(artifact string, reader io.Reader, interval time.Duration) *Reader {
	return &Reader{
		C:        make(chan *Tick, 1),
		interval: interval,
		artifact: artifact,
		reader:   reader,
	}
}

// Read bytes from the reader, reporting progress to the channel as data arrives.
func (r *Reader) Read(buf []byte) (int, error) {
	n, err := r.reader.Read(buf)
	r.value += uint64(n)
	if time.Since(r.lastTime) > r.interval {
		r.lastTime = time.Now()
		r.C <- &Tick{Artifact: r.artifact, Time: r.lastTime, Value: r.value}
		r.value = 0
	}
	return n, err
}

// Close closes the reader and channel.
func (r *Reader) Close() error {
	close(r.C)
	switch r.reader.(type) {
	case io.ReadCloser:
		return r.reader.(io.ReadCloser).Close()
	default:
		return nil
	}
}
