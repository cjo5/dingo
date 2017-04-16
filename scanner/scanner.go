package scanner

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
		tok.ID = token.STRING
	default:
		s.next()
		ch2 := s.ch

		switch ch1 {
		case -1:
			tok.ID = token.EOF
		case '(':
			tok.ID = token.LPAREN
		case ')':
			tok.ID = token.RPAREN
		case '{':
			tok.ID = token.LBRACE
		case '}':
			tok.ID = token.RBRACE
		case '.':
			tok.ID = token.DOT
		case ',':
			tok.ID = token.COMMA
		case ';':
			tok.ID = token.SEMICOLON
		case ':':
			tok.ID = token.COMMA
		case '+':
			tok.ID = token.ADD
			if ch2 == '=' {
				s.next()
				tok.ID = token.ADD_ASSIGN
			}
		case '-':
			tok.ID = token.SUB
			if ch2 == '=' {
				s.next()
				tok.ID = token.SUB_ASSIGN
			}
		case '*':
			tok.ID = token.MUL
			if ch2 == '=' {
				s.next()
				tok.ID = token.MUL_ASSIGN
			}
		case '/':
			tok.ID = token.DIV
			if ch2 == '=' {
				s.next()
				tok.ID = token.DIV_ASSIGN
			}
		case '%':
			tok.ID = token.MOD
			if ch2 == '=' {
				s.next()
				tok.ID = token.MOD_ASSIGN
			}
		case '=':
			tok.ID = token.ASSIGN
			if ch2 == '=' {
				s.next()
				tok.ID = token.EQ
			}
		case '&':
			tok.ID = token.AND
			if ch2 == '&' {
				s.next()
				tok.ID = token.LAND
			}
		case '|':
			tok.ID = token.OR
			if ch2 == '|' {
				s.next()
				tok.ID = token.LOR
			}
		case '!':
			tok.ID = token.LNOT
			if ch2 == '=' {
				s.next()
				tok.ID = token.NEQ
			}
		case '>':
			tok.ID = token.GT
			if ch2 == '=' {
				s.next()
				tok.ID = token.GTEQ
			}
		case '<':
			tok.ID = token.LT
			if ch2 == '=' {
				s.next()
				tok.ID = token.LTEQ
			}
		default:
			tok.ID = token.ILLEGAL
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
	tok := token.IDENT

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

	return token.INT, string(s.src[startOffset:s.chOffset])
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
