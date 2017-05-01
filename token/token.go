package token

import (
	"fmt"
	"strconv"
)

// TokenID is the type of token.
type TokenID int

// Token struct.
type Token struct {
	ID      TokenID
	Literal string
	Offset  int
	Line    int
	Column  int
}

// List of tokens.
//
const (
	Illegal TokenID = iota
	Eof
	Comment

	literalBeg
	Ident
	Int
	Float
	String
	Char
	literalEnd

	operatorBeg

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

	And
	Or

	Add
	Sub
	Mul
	Div
	Mod

	Assign
	AddAssign
	SubAssign
	MulAssign
	DivAssign
	ModAssign

	Land
	Lor
	Lnot

	Eq
	Neq
	Gt
	GtEq
	Lt
	LtEq

	operatorEnd

	keywordBeg
	If
	Else
	Elif
	For
	While
	Continue
	Break
	Print
	Module
	Var
	// TODO: true and false should be builtin types instead of keywords
	True
	False
	keywordEnd
)

var tokens = [...]string{
	Illegal: "ILLEGAL",
	Eof:     "eof",
	Comment: "comment",

	Ident:  "ident",
	Int:    "int",
	Float:  "float",
	String: "string",
	Char:   "char",

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
	Continue: "continue",
	Break:    "break",
	Print:    "print",
	Module:   "module",
	Var:      "var",
	True:     "true",
	False:    "false",
}

var keywords map[string]TokenID

func init() {
	keywords = make(map[string]TokenID)
	for i := keywordBeg + 1; i < keywordEnd; i++ {
		keywords[tokens[i]] = i
	}
}

// Lookup returns the identifier's token type.
//
func Lookup(ident string) TokenID {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return Ident
}

func (tok TokenID) String() string {
	s := ""
	if 0 <= tok && tok < TokenID(len(tokens)) {
		s = tokens[tok]
	}
	if s == "" {
		s = "token(" + strconv.Itoa(int(tok)) + ")"
	}
	return s
}

func (t Token) String() string {
	s := fmt.Sprintf("%d:%d: %v", t.Line, t.Column, t.ID)
	if t.ID == Ident || t.ID == String || t.ID == Int {
		s += ":" + t.Literal
	}
	return s
}

func (t Token) Pos() string {
	return fmt.Sprintf("%d:%d", t.Line, t.Column)
}

// IsValid returns true if it's a valid true.
func (t Token) IsValid() bool {
	return t.Line > 0
}
