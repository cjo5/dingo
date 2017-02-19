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

func (s *Scanner) Scan() token.Lexeme {
	s.skipWhitespace()

	lex := token.Lexeme{
		Offset:  s.chOffset,
		Line:    s.lineCount,
		Column:  (s.chOffset - s.lineOffset) + 1,
		Literal: "",
	}

	startOffset := s.chOffset

	switch ch1 := s.ch; {
	case isLetter(ch1):
		lex.Tok, lex.Literal = s.scanIdent()
	case isDigit(ch1):
		lex.Tok, lex.Literal = s.scanNumber()
	default:
		s.next()
		ch2 := s.ch

		switch ch1 {
		case -1:
			lex.Tok = token.EOF
		case '(':
			lex.Tok = token.LPAREN
		case ')':
			lex.Tok = token.RPAREN
		case '{':
			lex.Tok = token.LBRACE
		case '}':
			lex.Tok = token.RBRACE
		case '.':
			lex.Tok = token.DOT
		case ',':
			lex.Tok = token.COMMA
		case ';':
			lex.Tok = token.SEMICOLON
		case ':':
			lex.Tok = token.COMMA
		case '+':
			lex.Tok = token.ADD
			if ch2 == '=' {
				lex.Tok = token.ADD_ASSIGN
			}
		case '-':
			lex.Tok = token.SUB
			if ch2 == '=' {
				lex.Tok = token.SUB_ASSIGN
			}
		case '*':
			lex.Tok = token.MUL
			if ch2 == '=' {
				lex.Tok = token.MUL_ASSIGN
			}
		case '/':
			lex.Tok = token.DIV
			if ch2 == '=' {
				lex.Tok = token.DIV_ASSIGN
			}
		case '%':
			lex.Tok = token.MOD
			if ch2 == '=' {
				lex.Tok = token.MOD_ASSIGN
			}
		case '=':
			lex.Tok = token.ASSIGN
			if ch2 == '=' {
				lex.Tok = token.EQ
			}
		case '&':
			lex.Tok = token.AND
			if ch2 == '&' {
				lex.Tok = token.LAND
			}
		case '|':
			lex.Tok = token.OR
			if ch2 == '|' {
				lex.Tok = token.LOR
			}
		case '!':
			lex.Tok = token.LNOT
			if ch2 == '=' {
				lex.Tok = token.NEQ
			}
		case '>':
			lex.Tok = token.GT
			if ch2 == '=' {
				lex.Tok = token.GTEQ
			}
		case '<':
			lex.Tok = token.LT
			if ch2 == '=' {
				lex.Tok = token.LTEQ
			}
		default:
			lex.Tok = token.ILLEGAL
			lex.Literal = string(s.src[startOffset:s.chOffset])
		}
	}

	return lex
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
			s.lineOffset = s.chOffset + 1
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

func (s *Scanner) scanIdent() (token.Token, string) {
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
func (s *Scanner) scanNumber() (token.Token, string) {
	startOffset := s.chOffset

	for isDigit(s.ch) {
		s.next()
	}

	return token.INT, string(s.src[startOffset:s.chOffset])
}
