package frontend

import (
	"github.com/cjo5/dingo/internal/common"
	"github.com/cjo5/dingo/internal/token"
)

type lexer struct {
	src        []byte
	filename   string
	errors     *common.ErrorList
	ch         rune
	chOffset   int
	lineOffset int
	lineCount  int
	readOffset int
	prev       token.Token
}

func (l *lexer) init(src []byte, filename string, errors *common.ErrorList) {
	l.src = src
	l.filename = filename
	l.errors = errors
	l.ch = ' '
	l.chOffset = 0
	l.readOffset = 0
	l.lineOffset = -1 // -1 so column positions for line 1 are calculated correctly
	l.lineCount = 1
	l.prev = token.Invalid
	l.next()
}

func (l *lexer) lex() (token.Token, token.Position, string) {
	l.skipWhitespace(false)

	startOffset := 0
	literal := ""
	tok := token.Invalid

	// Insert semicolon if needed

	if l.ch == '\n' {
		if isLineTerminator(l.prev) {
			tok = token.Semicolon
			pos := l.newPos()
			startOffset = l.chOffset
			l.next()
			literal = string(l.src[startOffset:l.chOffset])
			l.prev = tok
			return tok, pos, literal
		}

		l.skipWhitespace(true)
	}

	pos := l.newPos()
	startOffset = l.chOffset

	switch ch1 := l.ch; {
	case isLetter(ch1):
		tok, literal = l.lexIdent()
	case isDigit(ch1, 10):
		tok = l.lexNumber(false)
	case ch1 == '\'':
		l.lexChar()
		tok = token.Char
	case ch1 == '"':
		l.lexString()
		tok = token.String
	case ch1 == '.':
		l.next()
		if isDigit(l.ch, 10) {
			tok = l.lexNumber(true)
		} else {
			tok = token.Dot
		}
	default:
		l.next()

		switch ch1 {
		case -1:
			tok = token.EOF
		case '(':
			tok = token.Lparen
		case ')':
			tok = token.Rparen
		case '{':
			tok = token.Lbrace
		case '}':
			tok = token.Rbrace
		case '[':
			tok = token.Lbrack
		case ']':
			tok = token.Rbrack
		case ',':
			tok = token.Comma
		case ';':
			tok = token.Semicolon
		case ':':
			tok = token.Colon
		case '@':
			tok = token.Directive
		case '+':
			tok = l.lexAlt3('=', token.AddAssign, '+', token.Inc, token.Add)
		case '-':
			tok = l.lexAlt3('=', token.SubAssign, '-', token.Dec, token.Sub)
		case '*':
			tok = l.lexAltEqual(token.MulAssign, token.Mul)
		case '/':
			if l.ch == '/' {
				// Single-line comment
				l.next()
				for l.ch != '\n' && l.ch != -1 {
					l.next()
				}
				tok = token.Comment
			} else if l.ch == '*' {
				// Multi-line comment
				l.next()
				nested := 1
				for l.ch != -1 && nested > 0 {
					ch := l.ch
					l.next()
					if ch == '*' && l.ch == '/' {
						nested--
						l.next()
					} else if ch == '/' && l.ch == '*' {
						nested++
						l.next()
					}
				}
				if nested > 0 {
					l.error(pos, "multi-line comment not closed")
				}
				tok = token.MultiComment
			} else {
				tok = l.lexAltEqual(token.DivAssign, token.Div)
			}
		case '%':
			tok = l.lexAltEqual(token.ModAssign, token.Mod)
		case '=':
			tok = l.lexAltEqual(token.Eq, token.Assign)
		case '&':
			tok = l.lexAlt2('&', token.Land, token.And)
		case '|':
			tok = l.lexAlt2('|', token.Lor, token.Invalid)
		case '!':
			tok = l.lexAltEqual(token.Neq, token.Lnot)
		case '>':
			tok = l.lexAltEqual(token.GtEq, token.Gt)
		case '<':
			tok = l.lexAltEqual(token.LtEq, token.Lt)
		default:
			tok = token.Invalid
		}
	}

	if len(literal) == 0 {
		literal = string(l.src[startOffset:l.chOffset])
	}

	if !tok.OneOf(token.Comment, token.MultiComment) {
		l.prev = tok
	}

	return tok, pos, literal
}

func isLineTerminator(id token.Token) bool {
	switch id {
	case token.Module, token.Ident, token.ParentMod, token.SelfMod,
		token.Integer, token.Float, token.Char, token.String, token.True, token.False, token.Null,
		token.Rparen, token.Rbrace, token.Rbrack,
		token.Continue, token.Break, token.Return,
		token.Inc, token.Dec:
		return true
	default:
		return false
	}
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch == '_')
}

func isDigit(ch rune, base int) bool {
	base10 := '0' <= ch && ch <= '9'
	switch base {
	case 8:
		return '0' <= ch && ch <= '7'
	case 10:
		return base10
	case 16:
		switch {
		case base10:
			return true
		case 'a' <= ch && ch <= 'f':
			return true
		case 'A' <= ch && ch <= 'F':
			return true
		}
	}

	return false
}

// Only supports ASCII characters for now
//
func (l *lexer) next() {
	if l.ch == '\n' {
		l.lineOffset = l.chOffset
		l.lineCount++
	}
	if l.readOffset < len(l.src) {
		l.chOffset = l.readOffset
		l.readOffset++
		l.ch = rune(l.src[l.chOffset])
	} else {
		l.chOffset = l.readOffset
		l.ch = -1
	}
}

func (l *lexer) newPos() token.Position {
	col := l.chOffset - l.lineOffset
	if col <= 0 {
		col = 1
	}
	return token.Position{Filename: l.filename, Offset: l.chOffset, Line: l.lineCount, Column: col}
}

func (l *lexer) error(pos token.Position, msg string) {
	l.errors.Add(pos, msg)
}

func (l *lexer) skipWhitespace(newline bool) {
	for l.ch == ' ' || l.ch == '\t' || (newline && l.ch == '\n') {
		l.next()
	}
}

func (l *lexer) lexAlt3(ch0 rune, tok0 token.Token, ch1 rune, tok1 token.Token, tok2 token.Token) token.Token {
	if l.ch == ch0 {
		l.next()
		return tok0
	} else if l.ch == ch1 {
		l.next()
		return tok1
	}
	return tok2
}

func (l *lexer) lexAlt2(ch0 rune, tok0 token.Token, tok1 token.Token) token.Token {
	if l.ch == ch0 {
		l.next()
		return tok0
	}
	return tok1
}

func (l *lexer) lexAltEqual(tok0 token.Token, tok1 token.Token) token.Token {
	return l.lexAlt2('=', tok0, tok1)
}

func (l *lexer) lexIdent() (token.Token, string) {
	startOffset := l.chOffset

	for isLetter(l.ch) || isDigit(l.ch, 10) {
		l.next()
	}

	tok := token.Ident
	lit := string(l.src[startOffset:l.chOffset])

	if len(lit) > 1 {
		tok = token.Lookup(lit)
	} else if lit == "_" {
		tok = token.Underscore
	}

	return tok, lit
}

func (l *lexer) lexDigits(base int) int {
	count := 0
	for {
		if isDigit(l.ch, base) {
			count++
		} else if l.ch != '_' {
			break
		}
		l.next()
	}
	return count
}

func isFractionOrExponent(ch rune) bool {
	return ch == '.' || ch == 'e' || ch == 'E'
}

func (l *lexer) lexNumber(float bool) token.Token {
	// Floating numbers can only be interpreted in base 10.

	id := token.Integer
	if float {
		id = token.Float
		l.lexDigits(10)
	} else {
		if l.ch == '0' {
			l.next()

			if l.ch == 'x' || l.ch == 'X' {
				// Hex
				l.next()
				if l.lexDigits(16) == 0 {
					l.error(l.newPos(), "invalid hexadecimal literal")
				} else if isFractionOrExponent(l.ch) {
					l.error(l.newPos(), "hexadecimal float literal is not supported")
				}
			} else {
				// Octal
				l.lexDigits(8)
				if isDigit(l.ch, 10) {
					l.error(l.newPos(), "invalid octal literal")
				} else if isFractionOrExponent(l.ch) {
					l.error(l.newPos(), "octal float literal is not supported")
				}
			}

			return id
		}

		l.lexDigits(10)

		if l.ch == '.' {
			id = token.Float
			l.next()

			if l.ch == '_' {
				l.error(l.newPos(), "decimal point '.' in float literal can not be followed by '_'")
				return id
			}

			l.lexDigits(10)
		}
	}

	if l.ch == 'e' || l.ch == 'E' {
		id = token.Float

		l.next()
		if l.ch == '+' || l.ch == '-' {
			l.next()
		}

		if isDigit(l.ch, 10) {
			l.lexDigits(10)
		} else {
			l.error(l.newPos(), "invalid exponent in float literal")
		}
	}

	return id
}

func (l *lexer) lexChar() {
	l.next()
	start := l.chOffset

	if l.ch == '\\' {
		l.next()
	}

	if l.ch != '\'' {
		if l.ch == '\n' {
			l.error(l.newPos(), "newline in char literal")
			return
		}
		l.next()
	}

	if l.ch == '\'' {
		if (l.chOffset - start) > 0 {
			l.next()
		} else {
			l.error(l.newPos(), "empty char literal")
		}
	} else {
		l.error(l.newPos(), "char literal not terminated")
	}
}

func (l *lexer) lexString() {
	l.next()

	for {
		ch := l.ch

		if ch == '\n' {
			l.error(l.newPos(), "newline in string literal")
			break
		}

		l.next()
		if ch == '"' {
			break
		}
		if ch == '\\' {
			l.next() // TODO: handle multi-char escape
		}
	}
}
