package token

import (
	"fmt"
	"strconv"
)

// TokenID is the type of token.
type TokenID int

// Position of token in a file.
type Position struct {
	Filename string
	Line     int
	Column   int
}

func (p Position) String() string {
	if len(p.Filename) == 0 {
		return fmt.Sprintf("%d:%d", p.Line, p.Column)
	}
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}

// Token struct.
type Token struct {
	ID      TokenID
	Literal string
	Pos     Position
}

// List of tokens.
//
const (
	Illegal TokenID = iota
	EOF
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

	operatorEnd

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
	Func
	// TODO: true and false should be builtin types instead of keywords
	True
	False
	keywordEnd
)

var tokens = [...]string{
	Illegal: "ILLEGAL",
	EOF:     "eof",
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
	Return:   "return",
	Continue: "continue",
	Break:    "break",
	Print:    "print",
	Module:   "module",
	Var:      "var",
	Func:     "fun",
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
	s := fmt.Sprintf("%s: %v", t.Pos, t.ID)
	if t.ID == Ident || t.ID == String || t.ID == Int {
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

func Synthetic(id TokenID, literal string) Token {
	return Token{ID: id, Literal: literal, Pos: Position{Filename: "", Line: -1, Column: -1}}
}
