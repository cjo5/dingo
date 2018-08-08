package token

import (
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
