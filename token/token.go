package token

import (
	"fmt"
	"strconv"
)

// ID of token.
type ID int

// List of tokens.
//
const (
	Invalid ID = iota
	EOF
	Comment
	MultiComment

	Ident
	Integer
	Float
	String

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
	Import
	Var
	Val
	Static
	Func
	Struct
	Public
	Internal
	Private

	False
	True

	keywordEnd
)

var tokens = [...]string{
	Invalid: "invalid",
	EOF:     "eof",
	Comment: "comment",

	Ident:   "ident",
	Integer: "integer",
	Float:   "float",
	String:  "string",

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
	Import:   "import",
	Var:      "var",
	Val:      "val",
	Static:   "static",
	Func:     "fun",
	Struct:   "struct",
	Public:   "pub",
	Internal: "int",
	Private:  "priv",

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

// Position of token in a file.
type Position struct {
	Line   int
	Column int
}

func NewPosition(line, column int) Position {
	return Position{Line: line, Column: column}
}

// NoPosition means it wasn't part of a file.
var NoPosition = Position{-1, -1}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// IsValid returns true if it's a valid file position.
func (p Position) IsValid() bool {
	return p.Line > -1
}

// Token struct.
type Token struct {
	ID      ID
	Literal string
	Pos     Position
}

// Synthetic creates a token that does not have a representation in the source code.
func Synthetic(id ID, literal string) Token {
	return Token{ID: id, Literal: literal, Pos: Position{Line: -1, Column: -1}}
}

func (t Token) String() string {
	if t.ID == Ident || t.ID == String || t.ID == Integer || t.ID == Float {
		return fmt.Sprintf("%s: %s", t.Pos, t.Literal)
	}
	return fmt.Sprintf("%s: %v", t.Pos, t.ID)
}

func (t Token) IsValid() bool {
	return t.Pos.IsValid()
}

// IsAssignOperator returns true if the token represents an assignment operator ('=', '+=', '-=', '*=', '/=', '%=').
func (t Token) IsAssignOperator() bool {
	return assignBeg < t.ID && t.ID < assignEnd
}

// OneOf returns true if token one of the IDs matches.
func (t Token) OneOf(ids ...ID) bool {
	for _, id := range ids {
		if t.ID == id {
			return true
		}
	}
	return false
}

// Is returns true if ID matches.
func (t Token) Is(id ID) bool {
	return t.ID == id
}
