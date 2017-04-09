package parser

import (
	"fmt"

	"github.com/jhnl/interpreter/ast"
	"github.com/jhnl/interpreter/scanner"
	"github.com/jhnl/interpreter/token"
)

func Parse(src []byte) (ast.Expr, error) {
	var p parser
	p.init(src)
	p.next()

	return p.parseExpr(), nil
}

type parser struct {
	trace   bool
	scanner scanner.Scanner
	token   token.Token
}

func (p *parser) init(src []byte) {
	p.trace = false
	p.scanner.Init(src)
}

func (p *parser) next() {
	if p.trace && p.token.IsValid() {
		fmt.Println(p.token)
	}

	p.token = p.scanner.Scan()
}

func (p *parser) match(id token.TokenID) bool {
	if p.token.ID != id {
		return false
	}
	p.next()

	return true
}

func (p *parser) parseExpr() ast.Expr {
	return p.parseEquality()
}

func (p *parser) parseEquality() ast.Expr {
	expr := p.parseComparison()

	for p.token.ID == token.EQ || p.token.ID == token.NEQ {
		op := p.token
		p.next()

		right := p.parseComparison()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}

	return expr
}

func (p *parser) parseComparison() ast.Expr {
	expr := p.parseTerm()

	for p.token.ID == token.GT || p.token.ID == token.GTEQ ||
		p.token.ID == token.LT || p.token.ID == token.LTEQ {
		op := p.token
		p.next()

		right := p.parseTerm()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}

	return expr
}

func (p *parser) parseTerm() ast.Expr {
	expr := p.parseFactor()

	for p.token.ID == token.ADD || p.token.ID == token.SUB {
		op := p.token
		p.next()
		right := p.parseFactor()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}

	return expr
}

func (p *parser) parseFactor() ast.Expr {
	expr := p.parseUnary()

	for p.token.ID == token.MUL || p.token.ID == token.DIV {
		op := p.token
		p.next()
		right := p.parseUnary()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}

	return expr
}

func (p *parser) parseUnary() ast.Expr {
	if p.token.ID == token.SUB || p.token.ID == token.LNOT {
		op := p.token
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Op: op, X: x}
	}

	return p.parsePrimary()
}

func (p *parser) parsePrimary() ast.Expr {
	switch p.token.ID {
	case token.INT, token.STRING, token.TRUE, token.FALSE:
		tok := p.token
		p.next()
		return &ast.Literal{Value: tok}
	case token.LBRACE:
		p.next()
		x := p.parseExpr()

		if p.token.ID != token.RBRACE {
			// TODO: Error handling
		}

		p.next()
		return x
	default:
		tok := p.token
		p.next()
		return &ast.BadExpr{From: tok, To: tok}
	}
}
