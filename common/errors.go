package common

import (
	"fmt"

	"bytes"
	"sort"

	"github.com/jhnl/dingo/token"
)

type Trace struct {
	Title string
	Lines []string
}

type ErrorID int

const (
	GenericError ErrorID = iota
	SyntaxError
	GenericWarning
)

func (e ErrorID) String() string {
	switch e {
	case GenericError:
		return "error"
	case GenericWarning:
		return "warning"
	case SyntaxError:
		return "syntax error"
	}
	return ""
}

type Error struct {
	Filename string
	Pos      token.Position
	ID       ErrorID
	Msg      string
	Trace    Trace
}

type ErrorList struct {
	Errors  []*Error
	isFatal bool
}

func NewTrace(title string, lines []string) Trace {
	return Trace{Title: title, Lines: lines}
}

func NewError(filename string, pos token.Position, id ErrorID, msg string) *Error {
	return &Error{Filename: filename, Pos: pos, ID: id, Msg: msg}
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
	msg := ""

	if e.Pos.IsValid() && len(e.Filename) > 0 {
		msg = fmt.Sprintf("%s:%s: %s: %s", e.Filename, e.Pos, e.ID, e.Msg)
	} else if len(e.Filename) > 0 {
		msg = fmt.Sprintf("%s: %s: %s", e.Filename, e.ID, e.Msg)
	} else {
		msg = fmt.Sprintf("%s: %s", e.ID, e.Msg)
	}

	if len(e.Trace.Lines) > 0 {
		var buf bytes.Buffer
		buf.WriteString(msg)
		e.Trace.write(&buf)
		msg = buf.String()
	}

	return msg
}

func (e *ErrorList) Add(filename string, pos token.Position, id ErrorID, format string, args ...interface{}) {
	err := NewError(filename, pos, id, fmt.Sprintf(format, args...))
	e.Errors = append(e.Errors, err)
	e.isFatal = true
}

func (e *ErrorList) AddTrace(filename string, pos token.Position, id ErrorID, trace Trace, format string, args ...interface{}) {
	err := NewError(filename, pos, id, fmt.Sprintf(format, args...))
	err.Trace = trace
	e.Errors = append(e.Errors, err)
	e.isFatal = true

}
func (e *ErrorList) AddGeneric(filename string, pos token.Position, err error) {
	if errList, ok := err.(*ErrorList); ok {
		e.Merge(errList)
	} else {
		e.Add(filename, pos, GenericError, err.Error())
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

// Sort errors by filename and line numbers.
func (e *ErrorList) Sort() {
	sort.Stable(byFileAndLineNumber(e.Errors))
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

type byFileAndLineNumber []*Error

func (e byFileAndLineNumber) Len() int      { return len(e) }
func (e byFileAndLineNumber) Swap(i, j int) { e[i], e[j] = e[j], e[i] }
func (e byFileAndLineNumber) Less(i, j int) bool {
	if e[i].Filename < e[j].Filename {
		return true
	} else if e[i].Filename == e[j].Filename {
		return e[i].Pos.Line < e[j].Pos.Line
	}
	return false
}

func (e ErrorList) Error() string {
	switch len(e.Errors) {
	case 0:
		return "no errors"
	case 1:
		return e.Errors[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", e.Errors[0].Error(), len(e.Errors)-1)
}
