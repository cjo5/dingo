package parser

import (
	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/token"
)

// Scanner struct.
type scanner struct {
	src        []byte
	filename   string
	errors     *common.ErrorList
	ch         rune
	chOffset   int
	lineOffset int
	lineCount  int
	readOffset int
}

// Init a scanner with the source code to scan.
func (s *scanner) init(src []byte, filename string, errors *common.ErrorList) {
	s.src = src
	s.filename = filename
	s.errors = errors
	s.ch = ' '
	s.chOffset = 0
	s.readOffset = 0
	s.lineOffset = 0
	s.lineCount = 1
	s.next()
}

// Scan returns the next token from the byte stream.
func (s *scanner) scan() token.Token {
	s.skipWhitespace()

	pos := s.newPos()
	tok := token.Token{
		Pos:     pos,
		Literal: "",
	}

	switch ch1 := s.ch; {
	case isLetter(ch1):
		tok.ID, tok.Literal = s.scanIdent()
	case isDigit(ch1):
		tok.ID, tok.Literal = s.scanNumber()
	case ch1 == '"':
		tok.Literal = s.scanString(pos)
		tok.ID = token.LitString
	default:
		startOffset := s.chOffset
		s.next()

		switch ch1 {
		case -1:
			tok.ID = token.EOF
		case '(':
			tok.ID = token.Lparen
		case ')':
			tok.ID = token.Rparen
		case '{':
			tok.ID = token.Lbrace
		case '}':
			tok.ID = token.Rbrace
		case '.':
			tok.ID = token.Dot
		case ',':
			tok.ID = token.Comma
		case ';':
			tok.ID = token.Semicolon
		case ':':
			tok.ID = token.Colon
		case '+':
			tok.ID = s.scanOptionalEqual(token.Add, token.AddAssign)
		case '-':
			tok.ID = s.scanOptionalEqual(token.Sub, token.SubAssign)
		case '*':
			tok.ID = s.scanOptionalEqual(token.Mul, token.MulAssign)
		case '/':
			if s.ch == '/' {
				// Single-line comment
				s.next()
				for s.ch != '\n' && s.ch != -1 {
					s.next()
				}
				s.next()
				tok.ID = token.Comment
			} else if s.ch == '*' {
				// Multi-line comment
				s.next()
				found := false
				for s.ch != -1 {
					ch := s.ch
					s.next()
					if ch == '*' {
						if s.ch == '/' {
							s.next()
							found = true
							break
						}
					}
				}
				if !found {
					s.error(pos, "multi-line comment not closed")
				}
				tok.ID = token.MultiComment
			} else {
				tok.ID = s.scanOptionalEqual(token.Div, token.DivAssign)
			}
		case '%':
			tok.ID = s.scanOptionalEqual(token.Mod, token.ModAssign)
		case '=':
			tok.ID = s.scanOptionalEqual(token.Assign, token.Eq)
		case '&':
			tok.ID = s.scanOptionalEqual(token.And, token.Land)
		case '|':
			tok.ID = s.scanOptionalEqual(token.Or, token.Lor)
		case '!':
			tok.ID = s.scanOptionalEqual(token.Lnot, token.Neq)
		case '>':
			tok.ID = s.scanOptionalEqual(token.Gt, token.GtEq)
		case '<':
			tok.ID = s.scanOptionalEqual(token.Lt, token.LtEq)
		default:
			tok.ID = token.Invalid
		}

		tok.Literal = string(s.src[startOffset:s.chOffset])
	}

	return tok
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

// Only supports ASCII characters for now
//
func (s *scanner) next() {
	if s.readOffset < len(s.src) {
		s.chOffset = s.readOffset
		s.ch = rune(s.src[s.chOffset])
		s.readOffset++
		if s.ch == '\n' {
			s.lineOffset = s.readOffset
			s.lineCount++
		}
	} else {
		s.chOffset = s.readOffset
		s.ch = -1
	}
}

func (s *scanner) newPos() token.Position {
	return token.Position{Filename: s.filename, Line: s.lineCount, Column: (s.chOffset - s.lineOffset) + 1}
}

func (s *scanner) error(pos token.Position, msg string) {
	s.errors.Add(pos, msg)
}

func (s *scanner) skipWhitespace() {
	for s.ch == ' ' || s.ch == '\t' || s.ch == '\n' {
		s.next()
	}
}

func (s *scanner) scanOptionalEqual(tok0 token.ID, tok1 token.ID) token.ID {
	if s.ch == '=' {
		s.next()
		return tok1
	}
	return tok0
}

func (s *scanner) scanIdent() (token.ID, string) {
	startOffset := s.chOffset

	for isLetter(s.ch) || isDigit(s.ch) {
		s.next()
	}

	lit := string(s.src[startOffset:s.chOffset])
	tok := token.Ident

	if len(lit) > 1 {
		tok = token.Lookup(lit)
	}

	return tok, lit
}

// Only supports integers for now.
func (s *scanner) scanNumber() (token.ID, string) {
	startOffset := s.chOffset
	for isDigit(s.ch) {
		s.next()
	}
	return token.LitInteger, string(s.src[startOffset:s.chOffset])
}

func (s *scanner) scanString(pos token.Position) string {
	startOffset := s.chOffset
	s.next()

	for {
		ch := s.ch

		if ch == '\n' {
			s.error(pos, "string literal not terminated")
			break
		}

		s.next()
		if ch == '"' {
			break
		}
		if ch == '\\' {
			s.next() // TODO: handle multi-char escape
		}
	}

	return string(s.src[startOffset:s.chOffset])
}
