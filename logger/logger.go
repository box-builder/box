package logger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/pkg/term"
	"github.com/fatih/color"
)

// Logger implements a per-plan logger.
type Logger struct {
	output io.Writer
	// if recording, will fill this buffer with logger output instead of printing to stdio.
	// if not yet recording, this will be nil.
	buffer *bytes.Buffer
	plan   string
	notrim bool
}

// New contypes a new per-plan logger.
func New(plan string, notrim bool) *Logger {
	return &Logger{plan: plan, output: os.Stdout, notrim: notrim}
}

// Record starts recording to the output buffer, which will be returned by the
// Output() method.
func (l *Logger) Record() {
	l.buffer = new(bytes.Buffer)
	l.output, color.Output = l.buffer, l.buffer
}

// Output returns the output buffer.
func (l *Logger) Output() io.Writer {
	return l.output
}

// Print is a bare-bones print statement.
func (l *Logger) Print(str string) {
	fmt.Fprint(l.output, l.Plan(), str)
}

// Plan gets the plan name specified at construction time
func (l *Logger) Plan() string {
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
	line := l.Plan()

	line += color.New(color.Bold, color.FgRed).SprintFunc()("!!! ")
	line += color.New(color.FgWhite).SprintFunc()(fmt.Sprintf("Error: %v", err))
	fmt.Fprintln(l.output, line)
	color.Unset()
}

// BuildStep logs a build step.
func (l *Logger) BuildStep(step, command string) {
	line := l.Plan()
	line += l.Good("")

	line += color.New(color.Bold, color.FgWhite).SprintFunc()("Execute: ")
	line += color.New(color.FgGreen).SprintFunc()(fmt.Sprintf("%s %s", step, command))
	l.printLog(line)
}

// CacheHit logs a cache hit.
func (l *Logger) CacheHit(imageID string) {
	line := l.Plan()
	line += l.Good("")
	line += color.New(color.FgWhite, color.Bold, color.BgRed).SprintFunc()("Cache hit:")
	line += color.New(color.FgCyan).SprintFunc()(fmt.Sprintf(" using %q", strings.SplitN(imageID, ":", 2)[1][:12]))
	l.printLog(line)
}

// CopyPath logs a copied path
func (l *Logger) CopyPath(file1, file2 string) {
	line := l.Plan()
	line += l.Notice("")
	line += color.New(color.FgRed).SprintFunc()("COPY: ")
	line += fmt.Sprintf("%q -> %q\n", file1, file2)
	l.printLog(line)
}

// Tag logs a tag
func (l *Logger) Tag(name string) {
	line := l.Plan()
	line += l.Good("")
	line += color.New(color.FgYellow).SprintFunc()("Tagged:")
	l.printLog(line + " " + name)
}

// EvalResponse logs the eval response
func (l *Logger) EvalResponse(response string) {
	line := l.Plan()
	line += l.Good("")
	line += color.New(color.FgWhite, color.Bold).SprintFunc()("Eval Response:")
	l.printLog(line + " " + response)
}

// Finish logs the finish.
func (l *Logger) Finish(response string) {
	line := l.Plan()
	line += l.Good("")
	line += color.New(color.FgRed, color.Bold).SprintFunc()("Finish: ")
	l.printLog(line + " " + response)
}

// BeginOutput demarcates an output section
func (l *Logger) BeginOutput() {
	line := l.Plan()
	line += color.New(color.FgRed, color.Bold, color.BgWhite).SprintFunc()("------ BEGIN OUTPUT ------")
	l.printLog(line)
}

// EndOutput ends an output section
func (l *Logger) EndOutput() {
	line := l.Plan()
	line += color.New(color.FgRed, color.Bold, color.BgWhite).SprintFunc()("------- END OUTPUT -------")
	l.printLog(line)
}

// printLog prints a log message optionally trimming the line to terminal width
// if l.trim is true
func (l *Logger) printLog(line string) {
	if !l.notrim && term.IsTerminal(0) {
		fmt.Fprintln(l.output, trimColoredString(line, 0, true))
	} else {
		fmt.Fprintln(l.output, line)
	}
	color.Unset()
}

// trimColoredString trims a string to cap size
func trimColoredString(original string, cap int, dots bool) string {
	if cap == 0 {
		wsz, _ := term.GetWinsize(0)
		cap = int(wsz.Width)
	}

	if dots {
		cap -= 3
	}

	var skip bool
	var charCount int
	buf := ""
	for _, b := range original {
		if b == '\033' {
			skip = true
		}

		if b == '\n' {
			break
		}

		if !skip {
			charCount++
		} else if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') {
			skip = false
		}

		if charCount > cap && !skip {
			break
		}

		buf = string(append([]rune(buf), b))
	}

	if dots && charCount > cap {
		buf += "..."
	}

	return buf
}

// Progress is a representation of a progress meter.
func (l *Logger) Progress(prefix string, count float64) {
	out := fmt.Sprint("\r")
	wsz, _ := term.GetWinsize(0)

	mbs := fmt.Sprintf("%.02fMB", count)

	justifiedWidth := int(wsz.Width) - len(mbs) - 2 // ... below
	if justifiedWidth < 0 {
		return
	}

	out += trimColoredString(fmt.Sprintf("%s%s %s", l.Plan(), color.New(color.FgWhite, color.Bold).SprintFunc()("+++"), color.New(color.FgRed, color.Bold).SprintfFunc()("%s", prefix)), justifiedWidth, true)
	out += ": "
	out += color.New(color.FgWhite).SprintFunc()(mbs)
	fmt.Fprint(l.output, out)
}
