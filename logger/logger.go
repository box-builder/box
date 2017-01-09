package logger

import (
	"fmt"

	"github.com/fatih/color"
)

// Logger implements a per-plan logger.
type Logger struct {
	plan string
}

// New constructs a new per-plan logger.
func New(plan string) *Logger {
	return &Logger{plan: plan}
}

func (l *Logger) getPlan() string {
	str := color.New(color.Bold, color.FgBlue).SprintFunc()("[")
	str += color.New(color.FgBlue).SprintFunc()(l.plan)
	str += color.New(color.Bold, color.FgBlue).SprintFunc()("] ")
	return str
}

func good() string {
	return color.New(color.FgGreen).SprintFunc()("+++ ")
}

func notice() string {
	return color.New(color.FgYellow).SprintFunc()("--- ")
}

// Error prints an error to the terminal all fancy-like.
func (l *Logger) Error(err interface{}) {
	line := l.getPlan()

	line += color.New(color.Bold, color.FgRed).SprintFunc()("!!! ")
	line += color.New(color.FgWhite).SprintFunc()(fmt.Sprintf("Error: %v", err))
	fmt.Println(line)
	color.Unset()
}

// BuildStep logs a build step.
func (l *Logger) BuildStep(step, command string) {
	line := l.getPlan()
	line += good()

	line += color.New(color.Bold, color.FgWhite).SprintFunc()("Execute: ")
	line += color.New(color.FgGreen).SprintFunc()(fmt.Sprintf("%s %s", step, command))
	fmt.Println(line)
	color.Unset()
}

// CacheHit logs a cache hit.
func (l *Logger) CacheHit(imageID string) {
	line := l.getPlan()
	line += good()
	line += color.New(color.FgWhite, color.Bold, color.BgRed).SprintFunc()("Cache hit:")
	line += color.New(color.FgCyan).SprintFunc()(fmt.Sprintf(" using %q", imageID))
	fmt.Println(line)
	color.Unset()
}

// CopyPath logs a copied path
func (l *Logger) CopyPath(file1, file2 string) {
	line := l.getPlan()
	line += notice()
	line += color.New(color.FgRed).SprintFunc()("COPY: ")
	line += fmt.Sprintf("%q -> %q\n", file1, file2)
	fmt.Println(line)
	color.Unset()
}

// Tag logs a tag
func (l *Logger) Tag(name string) {
	line := l.getPlan()
	line += good()
	line += color.New(color.FgYellow).SprintFunc()("Tagged:")
	fmt.Println(line, name)
}

// EvalResponse logs the eval response
func (l *Logger) EvalResponse(response string) {
	line := l.getPlan()
	line += good()
	line += color.New(color.FgWhite, color.Bold).SprintFunc()("Eval Response:")
	fmt.Println(line, "", response) // dat whitespace
	color.Unset()
}

// Finish logs the finish.
func (l *Logger) Finish(response string) {
	line := l.getPlan()
	line += good()
	line += color.New(color.FgRed, color.Bold).SprintFunc()("Finish: ")
	fmt.Println(line, response)
}

// BeginOutput demarcates an output section
func (l *Logger) BeginOutput() {
	line := l.getPlan()
	line += color.New(color.FgRed, color.Bold, color.BgWhite).SprintFunc()("------ BEGIN OUTPUT ------")
	fmt.Println(line)
}

// EndOutput ends an output section
func (l *Logger) EndOutput() {
	line := l.getPlan()
	line += color.New(color.FgRed, color.Bold, color.BgWhite).SprintFunc()("------- END OUTPUT -------")
	fmt.Println(line)
}
