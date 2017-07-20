package semantics

import (
	"bytes"

	"fmt"

	"github.com/jhnl/interpreter/token"
)

type exprPrinter struct {
	BaseVisitor
	buffer bytes.Buffer
}

// PrintExpr in-order.
func PrintExpr(expr Expr) string {
	p := &exprPrinter{}
	VisitExpr(p, expr)
	return p.buffer.String()
}

func (p *exprPrinter) VisitBinaryExpr(expr *BinaryExpr) Expr {
	leftPrec := prec(expr.Left)
	rightPrec := prec(expr.Right)
	opPrec := prec(expr)

	if opPrec < leftPrec {
		p.buffer.WriteString("(")
	}
	VisitExpr(p, expr.Left)
	if opPrec < leftPrec {
		p.buffer.WriteString(")")
	}

	p.buffer.WriteString(fmt.Sprintf(" %s ", expr.Op.Literal))

	if opPrec < rightPrec {
		p.buffer.WriteString("(")
	}
	VisitExpr(p, expr.Right)
	if opPrec < rightPrec {
		p.buffer.WriteString(")")
	}

	return expr
}

func (p *exprPrinter) VisitUnaryExpr(expr *UnaryExpr) Expr {
	xPrec := prec(expr.X)
	opPrec := prec(expr)

	p.buffer.WriteString(expr.Op.Literal)
	if opPrec < xPrec {
		p.buffer.WriteString("(")
	}
	VisitExpr(p, expr.X)
	if opPrec < xPrec {
		p.buffer.WriteString("(")
	}

	return expr
}

func (p *exprPrinter) VisitBasicLit(expr *BasicLit) Expr {
	p.buffer.WriteString(expr.Value.Literal)
	return expr
}

func (p *exprPrinter) VisitStructLit(expr *StructLit) Expr {
	p.buffer.WriteString("struct ")
	VisitExpr(p, expr.Name)
	return expr
}

func (p *exprPrinter) VisitIdent(expr *Ident) Expr {
	p.buffer.WriteString(expr.Name.Literal)
	return expr
}

func (p *exprPrinter) VisitDotIdent(expr *DotIdent) Expr {
	VisitExpr(p, expr.X)
	p.buffer.WriteString(".")
	p.VisitIdent(expr.Name)
	return expr
}

func (p *exprPrinter) VisitFuncCall(expr *FuncCall) Expr {
	VisitExpr(p, expr)

	p.buffer.WriteString("(")
	for i, arg := range expr.Args {
		VisitExpr(p, arg)
		if (i + 1) < len(expr.Args) {
			p.buffer.WriteString(", ")
		}
	}
	p.buffer.WriteString(")")

	return expr
}

// Lower number means higher precedence
func prec(expr Expr) int {
	switch t := expr.(type) {
	case *BinaryExpr:
		switch t.Op.ID {
		case token.Mul, token.Div, token.Mod:
			return 3
		case token.Add, token.Sub:
			return 4
		case token.Gt, token.GtEq, token.Lt, token.LtEq:
			return 5
		case token.Eq, token.Neq:
			return 6
		case token.And:
			return 7
		case token.Or:
			return 8
		default:
			panic(fmt.Sprintf("Unhandled binary op %s", t.Op.ID))
		}
	case *UnaryExpr:
		return 2
	case *BasicLit, *StructLit, *Ident:
		return 0
	case *DotIdent:
		return 1
	case *FuncCall:
		return 1
	default:
		return -1
	}
}
