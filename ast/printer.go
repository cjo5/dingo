package ast

import (
	"bytes"

	"github.com/jhnl/interpreter/token"
)

type printer struct {
	buffer bytes.Buffer
}

func Print(n Node) string {
	p := &printer{}

	n.Accept(p)
	return p.buffer.String()
}

func (p *printer) visitBinary(expr *BinaryExpr) {
	expr.Left.Accept(p)
	expr.Right.Accept(p)
	p.buffer.WriteString(expr.Op.Literal)
}

func (p *printer) visitUnary(expr *UnaryExpr) {
	closeParen := false

	switch expr.Op.ID {
	case token.SUB:
		p.buffer.WriteString("(-")
		closeParen = true
	case token.LNOT:
		p.buffer.WriteString("(!")
		closeParen = true
	}

	expr.X.Accept(p)
	if closeParen {
		p.buffer.WriteString(")")
	}
}

func (p *printer) visitLiteral(expr *Literal) {
	p.buffer.WriteString(expr.Value.Literal)
}
