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
	ILLEGAL TokenID = iota
	EOF
	COMMENT

	literalBeg
	IDENT
	INT
	FLOAT
	STRING
	CHAR
	literalEnd

	operatorBeg

	LPAREN
	RPAREN
	LBRACE
	RBRACE
	LBRACK
	RBRACK
	DOT
	COMMA
	SEMICOLON
	COLON

	AND
	OR

	ADD
	SUB
	MUL
	DIV
	MOD

	ASSIGN
	ADD_ASSIGN
	SUB_ASSIGN
	MUL_ASSIGN
	DIV_ASSIGN
	MOD_ASSIGN

	LAND
	LOR
	LNOT

	EQ
	NEQ
	GT
	GTEQ
	LT
	LTEQ

	operatorEnd

	keywordBeg
	IF
	ELSE
	ELIF
	FOR
	WHILE
	CONTINUE
	BREAK
	PRINT
	MODULE
	VAR
	// TODO: true and false should be builtin types instead of keywords
	TRUE
	FALSE
	keywordEnd
)

var tokens = [...]string{
	ILLEGAL: "ILLEGAL",
	EOF:     "eof",
	COMMENT: "comment",

	IDENT:  "ident",
	INT:    "int",
	FLOAT:  "float",
	STRING: "string",
	CHAR:   "char",

	LPAREN:    "(",
	RPAREN:    ")",
	LBRACE:    "{",
	RBRACE:    "}",
	LBRACK:    "[",
	RBRACK:    "]",
	DOT:       ".",
	COMMA:     ",",
	SEMICOLON: ";",
	COLON:     ":",

	ADD: "+",
	SUB: "-",
	MUL: "*",
	DIV: "/",
	MOD: "%",

	ASSIGN:     "=",
	ADD_ASSIGN: "+=",
	SUB_ASSIGN: "-=",
	MUL_ASSIGN: "*=",
	DIV_ASSIGN: "/=",
	MOD_ASSIGN: "%=",

	AND: "&",
	OR:  "|",

	LAND: "&&",
	LOR:  "||",
	LNOT: "!",

	EQ:   "==",
	NEQ:  "!=",
	GT:   ">",
	GTEQ: ">=",
	LT:   "<",
	LTEQ: "<=",

	IF:       "if",
	ELSE:     "else",
	ELIF:     "elif",
	FOR:      "for",
	WHILE:    "while",
	CONTINUE: "continue",
	BREAK:    "break",
	PRINT:    "print",
	MODULE:   "module",
	VAR:      "var",
	TRUE:     "true",
	FALSE:    "false",
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
	return IDENT
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
	if t.ID == IDENT || t.ID == STRING || t.ID == INT {
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
