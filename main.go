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
		fmt.Printf("!!! Error: %v", err.Error())
		os.Exit(2)
	}

	response, err := builder.Run(string(content))
	if err != nil {
		fmt.Printf("!!! Error: %v\n", err.Error())
		os.Exit(1)
	}

	if response.String() != "" {
		fmt.Printf("+++ Eval Response: %v\n", response)
	}

	if builder.imageID != "" {
		id := strings.SplitN(builder.imageID, ":", 2)[1]
		fmt.Printf("+++ Finish: %v\n", id)
	}
}
