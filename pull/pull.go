package pull

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type pullInfo struct {
	status   string
	progress float64
}

// Progress ingests pull information from docker and reports it as necessary.
type Progress struct {
	tty           bool
	idmap         map[string]pullInfo
	idlist        []string
	idok          bool
	status        string
	cok           bool
	pok           bool
	tok           bool
	progressCount float64
	reader        io.Reader
}

// NewProgress returns a progress parser.
func NewProgress(tty bool, reader io.Reader) *Progress {
	return &Progress{
		tty:    tty,
		reader: reader,
		idlist: []string{},
		idmap:  map[string]pullInfo{},
	}
}

// Print prints the current state of the progress meter.
func (p *Progress) Print() {
	for _, id := range p.idlist {
		if p.idmap[id].progress == 0 {
			fmt.Printf("\r\x1b[K%s %s\n", id, p.idmap[id].status)
		} else {
			fmt.Printf("\r\x1b[K%s %s %3.0f%%\n", id, p.idmap[id].status, p.idmap[id].progress)
		}
	}

	if !p.idok && p.status != "" {
		if p.pok { // image load only
			fmt.Printf("\r\x1b[K%s %3.0f%%", p.status, p.progressCount)
		} else {
			fmt.Printf("\r\x1b[K%s\n", p.status)
			fmt.Printf("\x1b[%dA", len(p.idmap)+1)
		}
	} else {
		if len(p.idmap) != 0 {
			fmt.Printf("\x1b[%dA", len(p.idmap))
		}
	}
}

// Process processes the reader with a function to map the pull information.
func (p *Progress) Process() (string, error) {
	retval := ""

	buf := bufio.NewReader(p.reader)
	for retval == "" {
		line, err := buf.ReadBytes('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}

		var unpacked map[string]interface{}
		if err := json.Unmarshal(line, &unpacked); err != nil {
			return "", err
		}

		retval = p.processProgressEntry(unpacked)

		if p.tty {
			p.Print()
		}
	}

	for i := 0; i < len(p.idlist)+1; i++ {
		fmt.Println()
	}

	return retval, nil
}

func (p *Progress) processProgressEntry(unpacked map[string]interface{}) string {
	if stream, ok := unpacked["stream"].(string); ok {
		// FIXME this is absolutely terrible
		if strings.HasPrefix(stream, "Loaded image ID: ") {
			return strings.TrimSpace(strings.TrimPrefix(stream, "Loaded image ID: "))
		}
	}

	var progress map[string]interface{}

	progress, p.pok = unpacked["progressDetail"].(map[string]interface{})
	if p.pok {
		var current, total float64

		current, p.cok = progress["current"].(float64)
		total, p.tok = progress["total"].(float64)
		if p.cok && p.tok {
			p.progressCount = (current / total) * 100
		}
	}

	p.status, _ = unpacked["status"].(string)
	var id string
	id, p.idok = unpacked["id"].(string)
	if p.idok {
		if _, ok := p.idmap[id]; !ok {
			p.idlist = append(p.idlist, id)
		}

		p.idmap[id] = pullInfo{p.status, p.progressCount}
	}

	return ""
}
