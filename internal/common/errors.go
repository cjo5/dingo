package common

import (
	"fmt"

	"bytes"
	"sort"

	"github.com/cjo5/dingo/internal/token"
)

// MessageID represents the type of error message.
type MessageID int

// The messages IDs.
const (
	ErrorMsg MessageID = iota
	WarningMsg
)

func (id MessageID) String() string {
	switch id {
	case ErrorMsg:
		return "error"
	case WarningMsg:
		return "warning"
	}
	return ""
}

type Error struct {
	Pos     token.Position
	EndPos  token.Position
	ID      MessageID
	Msg     string
	Context []string
}

type ErrorList struct {
	Warnings []*Error
	Errors   []*Error
}

func NewError(pos token.Position, endPos token.Position, id MessageID, msg string) *Error {
	return &Error{Pos: pos, EndPos: endPos, ID: id, Msg: msg}
}

func (e Error) Error() string {
	msg := ""

	id := ""
	if e.ID == ErrorMsg {
		id = BoldRed(e.ID.String())
	} else {
		id = BoldPurple(e.ID.String())
	}

	if e.Pos.IsValid() {
		msg = fmt.Sprintf("%s: %s: %s", e.Pos, id, e.Msg)
	} else if len(e.Pos.Filename) > 0 {
		msg = fmt.Sprintf("%s: %s: %s", e.Pos.Filename, id, e.Msg)
	} else {
		msg = fmt.Sprintf("%s: %s", id, e.Msg)
	}

	var buf bytes.Buffer
	buf.WriteString(msg)

	for _, l := range e.Context {
		buf.WriteString("\n")
		buf.WriteString(l)
	}

	return buf.String()
}

func (e *ErrorList) Add(pos token.Position, format string, args ...interface{}) {
	err := NewError(pos, pos, ErrorMsg, fmt.Sprintf(format, args...))
	e.Errors = append(e.Errors, err)
}

func (e *ErrorList) AddRange(pos token.Position, endPos token.Position, format string, args ...interface{}) {
	err := NewError(pos, endPos, ErrorMsg, fmt.Sprintf(format, args...))
	e.Errors = append(e.Errors, err)
}

func (e *ErrorList) AddContext(pos token.Position, context []string, format string, args ...interface{}) {
	err := NewError(pos, pos, ErrorMsg, fmt.Sprintf(format, args...))
	err.Context = context
	e.Errors = append(e.Errors, err)
}

func (e *ErrorList) AddWarning(pos token.Position, format string, args ...interface{}) {
	err := NewError(pos, pos, WarningMsg, fmt.Sprintf(format, args...))
	e.Warnings = append(e.Warnings, err)
}

func (e *ErrorList) AddGeneric2(pos token.Position, err error) {
	switch t := err.(type) {
	case *ErrorList:
		e.Append(t)
	case *Error:
		if t.ID == WarningMsg {
			e.Warnings = append(e.Warnings, t)
		} else {
			e.Errors = append(e.Errors, t)
		}
	default:
		e.Add(pos, err.Error())
	}
}

func (e *ErrorList) AddGeneric1(err error) {
	e.AddGeneric2(token.NoPosition, err)
}

func (e *ErrorList) Append(other *ErrorList) {
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
	if e[i].Pos.Filename < e[j].Pos.Filename {
		return true
	} else if e[i].Pos.Filename == e[j].Pos.Filename {
		if e[i].Pos.Line < e[j].Pos.Line {
			return true
		} else if e[i].Pos.Line == e[j].Pos.Line {
			return e[i].Pos.Column < e[j].Pos.Column
		}
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
