package report

import (
	"fmt"

	"github.com/jhnl/interpreter/token"
)

type Error struct {
	Pos token.Position
	Msg string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

type ErrorList []*Error

func (e *ErrorList) Add(pos token.Position, format string, args ...interface{}) {
	// TODO: Save all errors and filter redudant errors in final presentation
	if n := len(*e); n > 0 && (*e)[n-1].Pos.Line == pos.Line {
		return
	}
	err := &Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}
	*e = append(*e, err)
}

func (e ErrorList) Error() string {
	switch len(e) {
	case 0:
		return "no errors"
	case 1:
		return e[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", e[0].Error(), len(e)-1)
}
