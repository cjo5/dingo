package scanner

import (
	"fmt"

	"github.com/jhnl/interpreter/token"
)

type Error struct {
	Tok token.Token
	Msg string
}

func (e Error) Error() string {
	return e.Tok.Pos() + ": " + e.Msg
}

type ErrorList []*Error

func (e *ErrorList) Add(tok token.Token, msg string) {
	err := &Error{Tok: tok, Msg: msg}
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
