package docker

// p.s. this code is terrible. all of it is terrible.

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

type idInfo struct {
	idmap  map[string]pullInfo
	idlist []string
}

type progressInfo struct {
	idok          bool
	status        string
	cok           bool
	pok           bool
	tok           bool
	progressCount float64
}

func readProgress(reader io.Reader, readerFunc func(*idInfo, map[string]interface{}) (string, error)) (*idInfo, string, error) {
	retval := ""

	info := &idInfo{
		idlist: []string{},
		idmap:  map[string]pullInfo{},
	}

	buf := bufio.NewReader(reader)
	for retval == "" {
		line, err := buf.ReadBytes('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, "", err
		}

		var unpacked map[string]interface{}
		if err := json.Unmarshal(line, &unpacked); err != nil {
			return nil, "", err
		}

		retval, err = readerFunc(info, unpacked)
		if err != nil {
			return nil, "", err
		}
	}

	return info, retval, nil
}

func processProgressEntry(info *idInfo, unpacked map[string]interface{}) (string, *progressInfo) {
	if stream, ok := unpacked["stream"].(string); ok {
		// FIXME this is absolutely terrible
		if strings.HasPrefix(stream, "Loaded image ID:") {
			return strings.TrimSpace(strings.TrimPrefix(stream, "Loaded image ID:")), nil
		}
	}

	progInfo := &progressInfo{}

	var progress map[string]interface{}

	progress, progInfo.pok = unpacked["progressDetail"].(map[string]interface{})
	if progInfo.pok {
		var current, total float64

		current, progInfo.cok = progress["current"].(float64)
		total, progInfo.tok = progress["total"].(float64)
		if progInfo.cok && progInfo.tok {
			progInfo.progressCount = (current / total) * 100
		}
	}

	progInfo.status, _ = unpacked["status"].(string)
	var id string
	id, progInfo.idok = unpacked["id"].(string)
	if progInfo.idok {
		if _, ok := info.idmap[id]; !ok {
			info.idlist = append(info.idlist, id)
		}

		info.idmap[id] = pullInfo{progInfo.status, progInfo.progressCount}
	}

	return "", progInfo
}

func printProgress(progInfo *progressInfo, info *idInfo) {
	for _, id := range info.idlist {
		if info.idmap[id].progress == 0 {
			fmt.Printf("\r\x1b[K%s %s\n", id, info.idmap[id].status)
		} else {
			fmt.Printf("\r\x1b[K%s %s %3.0f%%\n", id, info.idmap[id].status, info.idmap[id].progress)
		}
	}

	if !progInfo.idok && progInfo.status != "" {
		if progInfo.pok { // image load only
			fmt.Printf("\r\x1b[K%s %3.0f%%", progInfo.status, progInfo.progressCount)
		} else {
			fmt.Printf("\r\x1b[K%s\n", progInfo.status)
			fmt.Printf("\x1b[%dA", len(info.idmap)+1)
		}
	} else {
		if len(info.idmap) != 0 {
			fmt.Printf("\x1b[%dA", len(info.idmap))
		}
	}
}

func printPull(tty bool, reader io.Reader) (string, error) {
	info, retval, err := readProgress(reader, func(info *idInfo, unpacked map[string]interface{}) (string, error) {
		retval, progInfo := processProgressEntry(info, unpacked)
		if retval != "" {
			return retval, nil
		}

		if tty {
			printProgress(progInfo, info)
		}

		return "", nil
	})

	if tty {
		for i := 0; i < len(info.idmap)+1; i++ {
			fmt.Println()
		}

	}

	if retval != "" {
		fmt.Println("Loaded image", retval)
	}

	return retval, err
}
