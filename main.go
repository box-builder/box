package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func main() {
	builder, err := NewBuilder()
	if err != nil {
		panic(err)
	}

	defer builder.Close()

	var content []byte

	if len(os.Args) == 2 {
		content, err = ioutil.ReadFile(os.Args[1])
	} else {
		content, err = ioutil.ReadAll(os.Stdin)
	}
	if err != nil {
		panic(fmt.Sprintf("Could not read input: %v", err))
	}

	response, err := builder.Run(string(content))
	if err != nil {
		panic(fmt.Sprintf("Could not execute ruby: %v", err))
	}

	if response.String() != "" {
		fmt.Printf("+++ Eval: %v\n", response)
	}

	if builder.imageID != "" {
		id := strings.SplitN(builder.imageID, ":", 2)[1]
		fmt.Printf("+++ Finish: %v\n", id)
	}
}
