package log

import (
	"fmt"

	"github.com/fatih/color"
)

func printGood() {
	color.New(color.FgGreen).Printf("+++ ")
}

func printNotice() {
	color.New(color.FgYellow).Printf("--- ")
}

// BuildStep logs a build step.
func BuildStep(step, command string) {
	printGood()
	color.New(color.Bold, color.FgWhite).Printf("Execute: ")
	color.Green(fmt.Sprintf("%s %s", step, command))
	color.Unset()
}

// CacheHit logs a cache hit.
func CacheHit(imageID string) {
	printGood()
	color.New(color.FgWhite, color.Bold, color.BgRed).Printf("Cache hit:")
	color.New(color.FgCyan).Printf(" using %q\n", imageID)
	color.Unset()
}

// CopyPath logs a copied path
func CopyPath(file1, file2 string) {
	printNotice()
	color.New(color.FgRed).Printf("COPY: ")
	color.Unset()
	fmt.Printf("%q -> %q\n", file1, file2)
}

// Tag logs a tag
func Tag(name string) {
	printGood()
	color.New(color.FgYellow).Printf("Tagged: ")
	color.Unset()
	fmt.Println(name)
}

// EvalResponse logs the eval response
func EvalResponse(response string) {
	printGood()
	color.New(color.FgWhite, color.Bold).Printf("Eval Response:")
	color.Unset()
	fmt.Println("", response) // dat whitespace
}

// Finish logs the finish.
func Finish(response string) {
	printGood()
	color.New(color.FgRed, color.Bold).Printf("Finish: ")
	color.Unset()
	fmt.Println(response)
}
