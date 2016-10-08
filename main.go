package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/erikh/box/builder"
)

func main() {
	b, err := builder.NewBuilder()
	if err != nil {
		panic(err)
	}
	defer b.Close()

	var content []byte

	if len(os.Args) == 2 {
		content, err = ioutil.ReadFile(os.Args[1])
	} else {
		content, err = ioutil.ReadAll(os.Stdin)
	}
	if err != nil {
		fmt.Printf("!!! Error: %v", err.Error())
		os.Exit(2)
	}

	response, err := b.Run(string(content))
	if err != nil {
		fmt.Printf("!!! Error: %v\n", err.Error())
		os.Exit(1)
	}

	if response.String() != "" {
		fmt.Printf("+++ Eval Response: %v\n", response)
	}

	id := b.ImageID()

	if strings.Contains(id, ":") {
		id = strings.SplitN(id, ":", 2)[1]
	}

	fmt.Printf("+++ Finish: %v\n", id)
}
