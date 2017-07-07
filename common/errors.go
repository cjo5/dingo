package common

import (
	"fmt"

	"bytes"
	"path/filepath"
	"sort"

	"github.com/jhnl/interpreter/token"
)

// Cwd is the absolute path of the current working directory.
var Cwd = ""

type Trace struct {
	Title string
	Lines []string
}

type Error struct {
	Pos      token.Position
	Filename string
	Msg      string
	Trace    Trace
	Fatal    bool
}

type ErrorList struct {
	Errors  []*Error
	isFatal bool
}

func NewTrace(title string, lines []string) Trace {
	return Trace{Title: title, Lines: lines}
}

func NewError(filename string, pos token.Position, msg string, fatal bool) *Error {
	if filepath.IsAbs(filename) && len(Cwd) > 0 {
		rel, err := filepath.Rel(Cwd, filename)
		if err == nil {
			filename = rel
		}
	}
	return &Error{Filename: filename, Pos: pos, Msg: msg, Fatal: fatal}
}

func (t Trace) write(buf *bytes.Buffer) {
	indent := 0
	if len(t.Title) > 0 {
		buf.WriteString(fmt.Sprintf("\n    %s", t.Title))
		indent++
	}

	for _, l := range t.Lines {
		buf.WriteString("\n")
		for i := 0; i <= indent; i++ {
			buf.WriteString("    ")
		}
		buf.WriteString(fmt.Sprintf("=> %s", l))
	}
}

func (e Error) Error() string {
	errType := "warning"
	if e.Fatal {
		errType = "error"
	}

	msg := ""

	if e.Pos.IsValid() && len(e.Filename) > 0 {
		msg = fmt.Sprintf("[%s] %s:%s: %s", errType, e.Filename, e.Pos, e.Msg)
	} else if len(e.Filename) > 0 {
		msg = fmt.Sprintf("[%s] %s: %s", errType, e.Filename, e.Msg)
	} else {
		msg = fmt.Sprintf("[%s] %s", errType, e.Msg)
	}

	if len(e.Trace.Lines) > 0 {
		var buf bytes.Buffer
		buf.WriteString(msg)
		e.Trace.write(&buf)
		msg = buf.String()
	}

	return msg
}

func (e *ErrorList) Add(filename string, pos token.Position, format string, args ...interface{}) {
	err := NewError(filename, pos, fmt.Sprintf(format, args...), true)
	e.Errors = append(e.Errors, err)
	e.isFatal = true
}

func (e *ErrorList) AddNonFatal(filename string, pos token.Position, format string, args ...interface{}) {
	err := NewError(filename, pos, fmt.Sprintf(format, args...), false)
	e.Errors = append(e.Errors, err)
}

func (e *ErrorList) AddTrace(filename string, pos token.Position, trace Trace, format string, args ...interface{}) {
	err := NewError(filename, pos, fmt.Sprintf(format, args...), true)
	err.Trace = trace
	e.Errors = append(e.Errors, err)
	e.isFatal = true

}
func (e *ErrorList) AddGeneric(filename string, pos token.Position, err error) {
	if errList, ok := err.(*ErrorList); ok {
		e.Merge(errList)
	} else {
		e.Add(filename, pos, err.Error())
	}
}

func (e *ErrorList) Merge(other *ErrorList) {
	for _, err := range other.Errors {
		e.Errors = append(e.Errors, err)
	}
}

func (e *ErrorList) IsFatal() bool {
	return e.isFatal
}

func (e *ErrorList) Count() int {
	return len(e.Errors)
}

// Sort errors by filename.
func (e *ErrorList) Sort() {
	sort.Stable(byFilename(e.Errors))
}

// Filter remove errors that are in the same file and line.
// Sort must be called before.
func (e *ErrorList) Filter() {
	for i, n := 0, len(e.Errors); i < n; i++ {
		err1 := e.Errors[i]
		if i > 0 && err1.Pos.IsValid() {
			err2 := e.Errors[i-1]
			if err1.Filename == err2.Filename && err1.Pos.Line == err2.Pos.Line {
				e.Errors = append(e.Errors[:i], e.Errors[i+1:]...)
				i--
				n--
			}
		}
	}
}

type byFilename []*Error

func (e byFilename) Len() int           { return len(e) }
func (e byFilename) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
func (e byFilename) Less(i, j int) bool { return e[i].Filename < e[j].Filename }

func (e ErrorList) Error() string {
	switch len(e.Errors) {
	case 0:
		return "no errors"
	case 1:
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", e.Errors[0].Error(), len(e.Errors)-1)
}
