package parser

import "github.com/jhnl/interpreter/token"

type Scanner struct {
	src        []byte
	ch         rune
	chOffset   int
	lineOffset int
	lineCount  int
	readOffset int
}

func (s *Scanner) Init(src []byte) {
	s.src = src
	s.ch = ' '
	s.chOffset = 0
	s.lineOffset = 0
	s.lineCount = 1
	s.readOffset = 0

	s.next()
}

// Scan returns the next token from the byte stream.
func (s *Scanner) Scan() (token.Token, *string) {
	s.skipWhitespace()

	var errMsg string
	tok := token.Token{
		Offset:  s.chOffset,
		Line:    s.lineCount,
		Column:  (s.chOffset - s.lineOffset) + 1,
		Literal: "",
	}

	startOffset := s.chOffset

	switch ch1 := s.ch; {
	case isLetter(ch1):
		tok.ID, tok.Literal = s.scanIdent()
	case isDigit(ch1):
		tok.ID, tok.Literal = s.scanNumber()
	case ch1 == '"':
		tok.Literal, errMsg = s.scanString()
		tok.ID = token.String
	default:
		s.next()
		ch2 := s.ch

		switch ch1 {
		case -1:
			tok.ID = token.Eof
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
			tok.ID = token.Add
			if ch2 == '=' {
				s.next()
				tok.ID = token.AddAssign
			}
		case '-':
			tok.ID = token.Sub
			if ch2 == '=' {
				s.next()
				tok.ID = token.SubAssign
			}
		case '*':
			tok.ID = token.Mul
			if ch2 == '=' {
				s.next()
				tok.ID = token.MulAssign
			}
		case '/':
			tok.ID = token.Div
			if ch2 == '=' {
				s.next()
				tok.ID = token.DivAssign
			}
		case '%':
			tok.ID = token.Mod
			if ch2 == '=' {
				s.next()
				tok.ID = token.ModAssign
			}
		case '=':
			tok.ID = token.Assign
			if ch2 == '=' {
				s.next()
				tok.ID = token.Eq
			}
		case '&':
			tok.ID = token.And
			if ch2 == '&' {
				s.next()
				tok.ID = token.Land
			}
		case '|':
			tok.ID = token.Or
			if ch2 == '|' {
				s.next()
				tok.ID = token.Lor
			}
		case '!':
			tok.ID = token.Lnot
			if ch2 == '=' {
				s.next()
				tok.ID = token.Neq
			}
		case '>':
			tok.ID = token.Gt
			if ch2 == '=' {
				s.next()
				tok.ID = token.GtEq
			}
		case '<':
			tok.ID = token.Lt
			if ch2 == '=' {
				s.next()
				tok.ID = token.LtEq
			}
		default:
			tok.ID = token.Illegal
		}

		tok.Literal = string(s.src[startOffset:s.chOffset])
	}

	if errMsg != "" {
		return tok, &errMsg
	}

	return tok, nil
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

// Only supports ASCII characters for now
//
func (s *Scanner) next() {
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

func (s *Scanner) skipWhitespace() {
	for s.ch == ' ' || s.ch == '\t' || s.ch == '\n' {
		s.next()
	}
}

func (s *Scanner) scanIdent() (token.TokenID, string) {
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
func (s *Scanner) scanNumber() (token.TokenID, string) {
	startOffset := s.chOffset

	for isDigit(s.ch) {
		s.next()
	}

	return token.Int, string(s.src[startOffset:s.chOffset])
}

func (s *Scanner) scanString() (string, string) {
	var errMsg string
	startOffset := s.chOffset
	s.next()

	for {
		ch := s.ch

		if ch == '\n' {
			errMsg = "string literal not terminated"
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

	return string(s.src[startOffset:s.chOffset]), errMsg
}
