package logger

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

// Logger implements a per-plan logger.
type Logger struct {
	output io.Writer
	// if recording, will fill this buffer with logger output instead of printing to stdio.
	// if not yet recording, this will be nil.
	buffer *bytes.Buffer
	plan   string
}

// New constructs a new per-plan logger.
func New(plan string) *Logger {
	return &Logger{plan: plan, output: os.Stdout}
}

// Record starts recording to the output buffer, which will be returned by the
// Output() method.
func (l *Logger) Record() {
	l.buffer = new(bytes.Buffer)
	l.output, color.Output = l.buffer, l.buffer
}

// Output returns the output buffer.
func (l *Logger) Output() *bytes.Buffer {
	return l.buffer
}

// Print is a bare-bones print statement.
func (l *Logger) Print(str string) {
	fmt.Fprint(l.output, str)
}

func (l *Logger) getPlan() string {
	str := color.New(color.Bold, color.FgBlue).SprintFunc()("[")
	str += color.New(color.FgBlue).SprintFunc()(l.plan)
	str += color.New(color.Bold, color.FgBlue).SprintFunc()("] ")
	return str
}

// Good reports a nice status in green to indicate successes.
func (l *Logger) Good(str string) string {
	return color.New(color.FgGreen).SprintFunc()(fmt.Sprintf("+++ %s", str))
}

// Notice is an arbitrary message explaining what the heck is going on.
func (l *Logger) Notice(str string) string {
	return color.New(color.FgYellow).SprintFunc()(fmt.Sprintf("--- %s", str))
}

// Error prints an error to the terminal all fancy-like.
func (l *Logger) Error(err interface{}) {
	line := l.getPlan()

	line += color.New(color.Bold, color.FgRed).SprintFunc()("!!! ")
	line += color.New(color.FgWhite).SprintFunc()(fmt.Sprintf("Error: %v", err))
	fmt.Fprintln(l.output, line)
	color.Unset()
}

// BuildStep logs a build step.
func (l *Logger) BuildStep(step, command string) {
	line := l.getPlan()
	line += l.Good("")

	line += color.New(color.Bold, color.FgWhite).SprintFunc()("Execute: ")
	line += color.New(color.FgGreen).SprintFunc()(fmt.Sprintf("%s %s", step, command))
	fmt.Fprintln(l.output, line)
	color.Unset()
}

// CacheHit logs a cache hit.
func (l *Logger) CacheHit(imageID string) {
	line := l.getPlan()
	line += l.Good("")
	line += color.New(color.FgWhite, color.Bold, color.BgRed).SprintFunc()("Cache hit:")
	line += color.New(color.FgCyan).SprintFunc()(fmt.Sprintf(" using %q", imageID))
	fmt.Fprintln(l.output, line)
	color.Unset()
}

// CopyPath logs a copied path
func (l *Logger) CopyPath(file1, file2 string) {
	line := l.getPlan()
	line += l.Notice("")
	line += color.New(color.FgRed).SprintFunc()("COPY: ")
	line += fmt.Sprintf("%q -> %q\n", file1, file2)
	fmt.Fprintln(l.output, line)
	color.Unset()
}

// Tag logs a tag
func (l *Logger) Tag(name string) {
	line := l.getPlan()
	line += l.Good("")
	line += color.New(color.FgYellow).SprintFunc()("Tagged:")
	fmt.Fprintln(l.output, line, name)
}

// EvalResponse logs the eval response
func (l *Logger) EvalResponse(response string) {
	line := l.getPlan()
	line += l.Good("")
	line += color.New(color.FgWhite, color.Bold).SprintFunc()("Eval Response:")
	fmt.Fprintln(l.output, line, "", response) // dat whitespace
	color.Unset()
}

// Finish logs the finish.
func (l *Logger) Finish(response string) {
	line := l.getPlan()
	line += l.Good("")
	line += color.New(color.FgRed, color.Bold).SprintFunc()("Finish: ")
	fmt.Fprintln(l.output, line, response)
}

// BeginOutput demarcates an output section
func (l *Logger) BeginOutput() {
	line := l.getPlan()
	line += color.New(color.FgRed, color.Bold, color.BgWhite).SprintFunc()("------ BEGIN OUTPUT ------")
	fmt.Fprintln(l.output, line)
}

// EndOutput ends an output section
func (l *Logger) EndOutput() {
	line := l.getPlan()
	line += color.New(color.FgRed, color.Bold, color.BgWhite).SprintFunc()("------- END OUTPUT -------")
	fmt.Fprintln(l.output, line)
}
