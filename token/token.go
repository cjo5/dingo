package token

import (
	"fmt"
	"strconv"
)

// ID of token.
type ID int

// Position of token in a file.
type Position struct {
	Filename string
	Line     int
	Column   int
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// Token struct.
type Token struct {
	ID      ID
	Literal string
	Pos     Position
}

// List of tokens.
//
const (
	Invalid ID = iota
	EOF
	Comment
	MultiComment

	Ident
	LitInteger
	LitString

	Lparen
	Rparen
	Lbrace
	Rbrace
	Lbrack
	Rbrack
	Dot
	Comma
	Semicolon
	Colon
	Arrow

	And
	Or

	Add
	Sub
	Mul
	Div
	Mod

	assignBeg
	Assign
	AddAssign
	SubAssign
	MulAssign
	DivAssign
	ModAssign
	assignEnd

	Land
	Lor
	Lnot

	Eq
	Neq
	Gt
	GtEq
	Lt
	LtEq

	keywordBeg

	If
	Else
	Elif
	For
	While
	Return
	Continue
	Break
	Print
	Module
	Var
	Val
	Func

	False
	True

	keywordEnd
)

var tokens = [...]string{
	Invalid: "invalid",
	EOF:     "eof",
	Comment: "comment",

	Ident:      "ident",
	LitInteger: "litInteger",
	LitString:  "litString",

	Lparen:    "(",
	Rparen:    ")",
	Lbrace:    "{",
	Rbrace:    "}",
	Lbrack:    "[",
	Rbrack:    "]",
	Dot:       ".",
	Comma:     ",",
	Semicolon: ";",
	Colon:     ":",
	Arrow:     "->",

	Add: "+",
	Sub: "-",
	Mul: "*",
	Div: "/",
	Mod: "%",

	Assign:    "=",
	AddAssign: "+=",
	SubAssign: "-=",
	MulAssign: "*=",
	DivAssign: "/=",
	ModAssign: "%=",

	And: "&",
	Or:  "|",

	Land: "&&",
	Lor:  "||",
	Lnot: "!",

	Eq:   "==",
	Neq:  "!=",
	Gt:   ">",
	GtEq: ">=",
	Lt:   "<",
	LtEq: "<=",

	If:       "if",
	Else:     "else",
	Elif:     "elif",
	For:      "for",
	While:    "while",
	Return:   "return",
	Continue: "continue",
	Break:    "break",
	Print:    "print",
	Module:   "module",
	Var:      "var",
	Val:      "val",
	Func:     "fun",

	True:  "true",
	False: "false",
}

var keywords map[string]ID

func init() {
	keywords = make(map[string]ID)
	for i := keywordBeg + 1; i < keywordEnd; i++ {
		keywords[tokens[i]] = i
	}
}

// Lookup returns the identifier's token ID.
//
func Lookup(ident string) ID {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return Ident
}

func (tok ID) String() string {
	s := ""
	if 0 <= tok && tok < ID(len(tokens)) {
		s = tokens[tok]
	}
	if s == "" {
		s = "token(" + strconv.Itoa(int(tok)) + ")"
	}
	return s
}

func (t Token) String() string {
	s := fmt.Sprintf("%s: %v", t.Pos, t.ID)
	if t.ID == Ident || t.ID == LitString || t.ID == LitInteger {
		s += ":" + t.Literal
	}
	return s
}

// IsValid returns true if it's a valid token.
func (t Token) IsValid() bool {
	return t.Pos.Line > 0
}

// IsAssignOperator returns true if the token represents an assignment operator ('=', '+=', '-=', '*=', '/=', '%=').
func (t Token) IsAssignOperator() bool {
	return assignBeg < t.ID && t.ID < assignEnd
}

func (t Token) OneOf(ids ...ID) bool {
	for _, id := range ids {
		if t.ID == id {
			return true
		}
	}
	return false
}

// Synthetic creates an artificial token that does not have a representation in the source code.
func Synthetic(id ID, literal string) Token {
	return Token{ID: id, Literal: literal, Pos: Position{Filename: "", Line: -1, Column: -1}}
}
