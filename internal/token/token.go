package token

import (
	"bytes"
	"fmt"
	"strconv"
)

// Token represents a syntax unit.
type Token int

// List of tokens.
//
const (
	Invalid Token = iota
	EOF
	Comment
	MultiComment

	Ident
	Integer
	Float
	Char
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
	Underscore
	Directive

	And
	Lnot

	binopBeg
	Add
	Sub
	Mul
	Div
	Mod

	Land
	Lor

	Eq
	Neq
	Gt
	GtEq
	Lt
	LtEq
	binopEnd

	assignBeg
	Assign
	AddAssign
	SubAssign
	MulAssign
	DivAssign
	ModAssign
	assignEnd

	Inc
	Dec

	keywordBeg
	If
	Else
	Elif
	For
	While
	Return
	Defer
	Continue
	Break
	As
	Lenof
	Sizeof
	Module
	Include
	Import
	Const
	Var
	Val
	AliasType
	Func
	Struct
	Public
	Private
	Extern

	True
	False
	Null
	keywordEnd
)

// Alias
const (
	Pointer Token = And
	Deref   Token = Mul
	Addr    Token = And
)

var tokens = [...]string{
	Invalid: "invalid",
	EOF:     "eof",
	Comment: "comment",

	Ident:   "ident",
	Integer: "integer",
	Float:   "float",
	Char:    "char",
	String:  "string",

	Lparen:     "(",
	Rparen:     ")",
	Lbrace:     "{",
	Rbrace:     "}",
	Lbrack:     "[",
	Rbrack:     "]",
	Dot:        ".",
	Comma:      ",",
	Semicolon:  ";",
	Colon:      ":",
	Underscore: "_",
	Directive:  "@",

	And:  "&",
	Lnot: "!",

	Add: "+",
	Sub: "-",
	Mul: "*",
	Div: "/",
	Mod: "%",

	Land: "&&",
	Lor:  "||",

	Eq:   "==",
	Neq:  "!=",
	Gt:   ">",
	GtEq: ">=",
	Lt:   "<",
	LtEq: "<=",

	Assign:    "=",
	AddAssign: "+=",
	SubAssign: "-=",
	MulAssign: "*=",
	DivAssign: "/=",
	ModAssign: "%=",

	Inc: "++",
	Dec: "--",

	If:        "if",
	Else:      "else",
	Elif:      "elif",
	For:       "for",
	While:     "while",
	Return:    "return",
	Defer:     "defer",
	Continue:  "continue",
	Break:     "break",
	As:        "as",
	Lenof:     "len",
	Sizeof:    "sizeof",
	Module:    "module",
	Include:   "include",
	Import:    "import",
	Const:     "const",
	Var:       "var",
	Val:       "val",
	AliasType: "alias",
	Func:      "fun",
	Struct:    "struct",
	Public:    "pub",
	Private:   "priv",
	Extern:    "extern",

	True:  "true",
	False: "false",
	Null:  "null",
}

var keywords map[string]Token

func init() {
	keywords = make(map[string]Token)
	for i := keywordBeg + 1; i < keywordEnd; i++ {
		keywords[tokens[i]] = i
	}
}

// Lookup returns the identifier token.
//
func Lookup(ident string) Token {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return Ident
}

// Quote escapes control characters.
func Quote(literal string) string {
	s := strconv.Quote(literal)
	i := len(s) - 1
	return s[1:i]
}

func (tok Token) String() string {
	s := ""
	if 0 <= tok && tok < Token(len(tokens)) {
		s = tokens[tok]
	}
	if s == "" {
		s = "token(" + strconv.Itoa(int(tok)) + ")"
	}
	return s
}

// IsAssignOp returns true if the token represents an assignment operator:
// ('=', '+=', '-=', '*=', '/=', '%=')
func (t Token) IsAssignOp() bool {
	return assignBeg < t && t < assignEnd
}

// IsBinaryOp return true if the token represents a binary operator:
// ('+', '-', '*', '/', '%', '||', '&&', '!=', '==', '>', '>=', '<', '<=')
func (t Token) IsBinaryOp() bool {
	return binopBeg < t && t < binopEnd
}

// OneOf returns true if token one of the IDs match.
func (t Token) OneOf(ids ...Token) bool {
	for _, id := range ids {
		if t == id {
			return true
		}
	}
	return false
}

// Is returns true if ID matches.
func (t Token) Is(id Token) bool {
	return t == id
}

// Position of token in a file.
type Position struct {
	Filename string
	Offset   int
	Line     int
	Column   int
}

func NewPosition1(filename string) Position {
	return Position{Filename: filename, Offset: 0, Line: -1, Column: -1}
}

// NoPosition means it wasn't part of a file.
var NoPosition = Position{}

func (p Position) String() string {
	var buf bytes.Buffer
	if len(p.Filename) > 0 {
		buf.WriteString(p.Filename)
	}

	if p.Line > -1 {
		if buf.Len() > 0 {
			buf.WriteString(":")
		}
		buf.WriteString(fmt.Sprintf("%d:%d", p.Line, p.Column))
	}

	if buf.Len() > 0 {
		return buf.String()
	}

	return "-"
}

// IsValid returns true if it's a valid file position.
func (p Position) IsValid() bool {
	return p.Line > 0
}
