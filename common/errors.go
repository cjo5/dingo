package common

import (
	"fmt"
	"io/ioutil"
	"strings"

	"bytes"
	"sort"

	"github.com/jhnl/dingo/token"
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
		id = BoldYellow(e.ID.String())
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

func (e *ErrorList) AddGeneric3(pos token.Position, err error) {
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
	e.AddGeneric3(token.NoPosition, err)
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

func (e *ErrorList) LoadContext() {
	loadContext1(e.Warnings)
	loadContext1(e.Errors)
}

// TODO: Rewrite
func loadContext1(errors []*Error) {
	var buf []byte
	filename := ""

	for _, e := range errors {
		if len(e.Context) > 0 {
			continue
		}

		if e.Pos.IsValid() {
			if e.Pos.Filename != filename {
				buf = nil
				filename = e.Pos.Filename
				if len(filename) > 0 {
					buf2, err := ioutil.ReadFile(filename)
					if err == nil {
						buf = buf2
					}
				}
			}

			if e.Pos.Offset <= len(buf) {
				// Find line start and end
				start := e.Pos.Offset - 1
				end := e.Pos.Offset
				foundStart := false
				foundEnd := false

				for !(foundStart && foundEnd) {
					if !foundStart {
						if start <= 0 {
							start = 0
							foundStart = true
						} else if buf[start] == '\n' {
							start++
							foundStart = true
						} else {
							start--
						}
					}

					if !foundEnd {
						if end >= len(buf) {
							end = len(buf)
							foundEnd = true
						} else if buf[end] == '\n' {
							if end > 0 && buf[end-1] == '\r' {
								end--
							}
							foundEnd = true
						} else {
							end++
						}
					}
				}

				if start < end {
					var indent bytes.Buffer
					spaces := 0

					for _, ch := range buf[start:end] {
						if ch == ' ' {
							indent.WriteString(" ")
							spaces++
						} else if ch == '\t' {
							indent.WriteString("    ")
							spaces++
						} else {
							break
						}
					}

					line := string(buf[start:end])
					line = strings.Trim(line, " \t")
					lineLen := len(line)

					maxLen := 200

					if lineLen > maxLen {
						line = line[:maxLen+1]
						lineLen = len(line)
						line += "..."
					}

					if lineLen > 0 {
						lineLen++ // include newline in line length

						line = indent.String() + line
						e.Context = append(e.Context, line)

						col := (e.Pos.Column - 1) - spaces
						if col >= 0 && col <= lineLen {
							mark := indent.String() + strings.Repeat(" ", col)
							endCol := (e.EndPos.Column - 1 - spaces) - col
							if e.EndPos.Line == e.Pos.Line && endCol > 1 && endCol < lineLen {
								mark += BoldGreen(strings.Repeat("~", endCol))
							} else {
								mark += BoldGreen("^")
							}
							e.Context = append(e.Context, mark)
						}
					}
				}
			}
		}
	}
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
