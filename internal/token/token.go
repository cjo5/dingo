package token

import (
	"strconv"
)

// Token represents a syntax atom.
type Token int

// List of tokens.
//
const (
	Invalid Token = iota
	EOF
	Comment
	MultiComment

	// Identifier and literals
	Ident
	Integer
	Float
	Char
	String

	// Speecial chars
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

	// Arithmetic
	Add
	Sub
	Mul
	Div
	Mod
	Inc
	Dec

	// Relational
	Eq
	Neq
	Gt
	GtEq
	Lt
	LtEq

	Assign
	AddAssign
	SubAssign
	MulAssign
	DivAssign
	ModAssign

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
	Importlocal
	Var
	Val
	AliasType
	Func
	Struct
	Public
	Private
	Extern

	ParentMod
	SelfMod

	Land
	Lor
	Lnot

	True
	False
	Null
	keywordEnd
)

// Alias
const (
	Addr        Token = And
	Deref       Token = Mul
	Placeholder Token = Underscore
	Pointer     Token = And
	RootMod     Token = Dot
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
	And:        "&",

	Add: "+",
	Sub: "-",
	Mul: "*",
	Div: "/",
	Mod: "%",
	Inc: "++",
	Dec: "--",

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

	If:          "if",
	Else:        "else",
	Elif:        "elif",
	For:         "for",
	While:       "while",
	Return:      "return",
	Defer:       "defer",
	Continue:    "continue",
	Break:       "break",
	As:          "as",
	Lenof:       "len",
	Sizeof:      "sizeof",
	Module:      "module",
	Include:     "include",
	Import:      "import",
	Importlocal: "importlocal",
	Var:         "var",
	Val:         "val",
	AliasType:   "typealias",
	Func:        "fun",
	Struct:      "struct",
	Public:      "pub",
	Private:     "priv",
	Extern:      "extern",

	ParentMod: "msuper",
	SelfMod:   "mself",

	Land: "and",
	Lor:  "or",
	Lnot: "not",

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

// IsAssignOp returns true if the token represents an assignment operator.
func (tok Token) IsAssignOp() bool {
	switch tok {
	case Assign, AddAssign, SubAssign, MulAssign, DivAssign, ModAssign:
		return true
	}
	return false
}

// IsBinaryOp return true if the token represents a binary operator.
func (tok Token) IsBinaryOp() bool {
	switch tok {
	case Add, Sub, Mul, Div, Mod,
		Eq, Neq, Gt, GtEq, Lt, LtEq,
		Land, Lor:
		return true
	}
	return false
}

// OneOf returns true if token one of the IDs match.
func (tok Token) OneOf(ids ...Token) bool {
	for _, id := range ids {
		if tok == id {
			return true
		}
	}
	return false
}

// Is returns true if ID matches.
func (tok Token) Is(other Token) bool {
	return tok == other
}
