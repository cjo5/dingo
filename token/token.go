package token

import (
	"fmt"
	"strconv"
	"strings"
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

// The list of tokens.
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
	TRUE
	FALSE
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

	operatorEend

	keywordBeg
	IF
	ELSE
	FOR
	WHILE
	CONTINUE
	BREAK
	keywordEnd
)

var tokens = [...]string{
	ILLEGAL: "ILLEGAL",
	EOF:     "EOF",
	COMMENT: "COMMENT",

	IDENT:  "IDENT",
	INT:    "INT",
	FLOAT:  "FLOAT",
	STRING: "STRING",
	CHAR:   "CHAR",

	LPAREN:    "LPAREN",
	RPAREN:    "RPAREN",
	LBRACE:    "LBRACE",
	RBRACE:    "RBRACE",
	LBRACK:    "LBRACK",
	RBRACK:    "RBRACK",
	DOT:       "DOT",
	COMMA:     "COMMA",
	SEMICOLON: "SEMICOLON",
	COLON:     "COLON",

	ADD: "ADD",
	SUB: "SUB",
	MUL: "MUL",
	DIV: "DIV",
	MOD: "MOD",

	ASSIGN:     "ASSIGN",
	ADD_ASSIGN: "ADD_ASSIGN",
	SUB_ASSIGN: "SUB_ASSIGN",
	MUL_ASSIGN: "MUL_ASSIGN",
	DIV_ASSIGN: "DIV_ASSIGN",
	MOD_ASSIGN: "MOD_ASSIGN",

	AND: "AND",
	OR:  "OR",

	LAND: "LAND",
	LOR:  "LOR",
	LNOT: "LNOT",

	EQ:   "EQ",
	NEQ:  "NEQ",
	GT:   "GT",
	GTEQ: "GTEQ",
	LT:   "LT",
	LTEQ: "LTEQ",

	IF:       "if",
	ELSE:     "else",
	FOR:      "for",
	WHILE:    "while",
	CONTINUE: "continue",
	BREAK:    "break",
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
		s = strings.ToUpper(tokens[tok])
	}
	if s == "" {
		s = "token(" + strconv.Itoa(int(tok)) + ")"
	}
	return s
}

func (t Token) String() string {
	s := fmt.Sprintf("%d:%d: %v", t.Line, t.Column, t.ID)

	if len(t.Literal) > 0 {
		s += ":" + t.Literal
	}

	return s
}

// IsValid returns true if it's a valid true.
func (t Token) IsValid() bool {
	return t.Line > 0
}
