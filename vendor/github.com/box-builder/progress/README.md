## Package progress - progress meters through a channel, wrapping an io.Reader

[![Build Status](http://jenkins.hollensbe.org:8080/buildStatus/icon?job=progress-master)](http://jenkins.hollensbe.org:8080/job/progress-master/)
[![GoDoc](https://godoc.org/github.com/box-builder/progress?status.svg)](https://godoc.org/github.com/box-builder/progress)

### Example:

```go
func progress(reader io.Reader) {
	r = NewReader("an_file.tar", reader, 100*time.Millisecond)
	defer r.Close()

	var count uint64
	// This goroutine consumes the channel and concatenates the count to display
	// the total as it increases.
	go func(r *Reader) {
		for tick := range r.C {
			count += tick.Value
			fmt.Printf("\r%d", count)
		}

		fmt.Println()
	}(r)

	// use r like you normally would; progress will be reported to C every 100
	// milliseconds.
	if _, err := io.Copy(ioutil.Discard, r); err != nil {
		panic(err)
	}
}
```

### Author

Erik Hollensbe <github@hollensbe.org>
