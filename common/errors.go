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
	Warning
)

func (e ErrorID) String() string {
	switch e {
	case GenericError:
		return "error"
	case SyntaxError:
		return "syntax error"
	case Warning:
		return "warning"
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
	Warnings []*Error
	Errors   []*Error
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
}

func (e *ErrorList) AddTrace(filename string, pos token.Position, id ErrorID, trace Trace, format string, args ...interface{}) {
	err := NewError(filename, pos, id, fmt.Sprintf(format, args...))
	err.Trace = trace
	e.Errors = append(e.Errors, err)
}

func (e *ErrorList) AddWarning(filename string, pos token.Position, format string, args ...interface{}) {
	err := NewError(filename, pos, Warning, fmt.Sprintf(format, args...))
	e.Warnings = append(e.Warnings, err)
}

func (e *ErrorList) AddGeneric3(filename string, pos token.Position, err error) {
	switch t := err.(type) {
	case *ErrorList:
		e.Merge(t)
	case *Error:
		if t.ID == Warning {
			e.Warnings = append(e.Warnings, t)
		} else {
			e.Errors = append(e.Errors, t)
		}
	default:
		e.Add(filename, pos, GenericError, err.Error())
	}
}

func (e *ErrorList) AddGeneric1(err error) {
	e.AddGeneric3("", token.NoPosition, err)
}

func (e *ErrorList) Merge(other *ErrorList) {
	for _, warn := range other.Warnings {
		e.Warnings = append(e.Warnings, warn)
	}
	for _, err := range other.Errors {
		e.Errors = append(e.Errors, err)
	}
}

func (e *ErrorList) IsError() bool {
	return len(e.Errors) > 0
}

// Sort errors by filename and line numbers.
func (e *ErrorList) Sort() {
	sort.Stable(byFileAndLineNumber(e.Warnings))
	sort.Stable(byFileAndLineNumber(e.Errors))
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
