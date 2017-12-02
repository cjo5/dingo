package semantics

import (
	"bytes"

	"fmt"

	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

type exprPrinter struct {
	ir.BaseVisitor
	buffer bytes.Buffer
}

func (p *exprPrinter) VisitBinaryExpr(expr *ir.BinaryExpr) ir.Expr {
	leftPrec := prec(expr.Left)
	rightPrec := prec(expr.Right)
	opPrec := prec(expr)

	if opPrec < leftPrec {
		p.buffer.WriteString("(")
	}
	ir.VisitExpr(p, expr.Left)
	if opPrec < leftPrec {
		p.buffer.WriteString(")")
	}

	p.buffer.WriteString(fmt.Sprintf(" %s ", expr.Op.Literal))

	if opPrec < rightPrec {
		p.buffer.WriteString("(")
	}
	ir.VisitExpr(p, expr.Right)
	if opPrec < rightPrec {
		p.buffer.WriteString(")")
	}

	return expr
}

func (p *exprPrinter) VisitUnaryExpr(expr *ir.UnaryExpr) ir.Expr {
	xPrec := prec(expr.X)
	opPrec := prec(expr)

	p.buffer.WriteString(expr.Op.Literal)
	if opPrec < xPrec {
		p.buffer.WriteString("(")
	}
	ir.VisitExpr(p, expr.X)
	if opPrec < xPrec {
		p.buffer.WriteString(")")
	}

	return expr
}

func (p *exprPrinter) VisitStarExpr(expr *ir.StarExpr) ir.Expr {
	xPrec := prec(expr.X)
	opPrec := prec(expr)

	p.buffer.WriteString(expr.Star.Literal)
	if opPrec < xPrec {
		p.buffer.WriteString("(")
	}
	ir.VisitExpr(p, expr.X)
	if opPrec < xPrec {
		p.buffer.WriteString(")")
	}

	return expr
}

func (p *exprPrinter) VisitBasicLit(expr *ir.BasicLit) ir.Expr {
	p.buffer.WriteString(expr.Value.Literal)
	return expr
}

func (p *exprPrinter) VisitStructLit(expr *ir.StructLit) ir.Expr {
	ir.VisitExpr(p, expr.Name)
	return expr
}

func (p *exprPrinter) VisitIdent(expr *ir.Ident) ir.Expr {
	p.buffer.WriteString(expr.Name.Literal)
	return expr
}

func (p *exprPrinter) VisitDotExpr(expr *ir.DotExpr) ir.Expr {
	ir.VisitExpr(p, expr.X)
	p.buffer.WriteString(".")
	p.VisitIdent(expr.Name)
	return expr
}

func (p *exprPrinter) VisitCastExpr(expr *ir.CastExpr) ir.Expr {
	p.buffer.WriteString(expr.Cast.Literal)
	p.buffer.WriteString("(")
	ir.VisitExpr(p, expr.ToTyp)
	p.buffer.WriteString(",")
	ir.VisitExpr(p, expr.X)
	p.buffer.WriteString(")")
	return expr
}

func (p *exprPrinter) VisitFuncCall(expr *ir.FuncCall) ir.Expr {
	ir.VisitExpr(p, expr.X)

	p.buffer.WriteString("(")
	for i, arg := range expr.Args {
		ir.VisitExpr(p, arg)
		if (i + 1) < len(expr.Args) {
			p.buffer.WriteString(", ")
		}
	}
	p.buffer.WriteString(")")

	return expr
}

// Lower number means higher precedence
func prec(expr ir.Expr) int {
	switch t := expr.(type) {
	case *ir.BinaryExpr:
		switch t.Op.ID {
		case token.Mul, token.Div, token.Mod:
			return 3
		case token.Add, token.Sub:
			return 4
		case token.Gt, token.GtEq, token.Lt, token.LtEq:
			return 5
		case token.Eq, token.Neq:
			return 6
		case token.Land:
			return 7
		case token.Lor:
			return 8
		default:
			panic(fmt.Sprintf("Unhandled binary op %s", t.Op.ID))
		}
	case *ir.UnaryExpr:
		return 2
	case *ir.StarExpr:
		return 1
	case *ir.BasicLit, *ir.StructLit, *ir.Ident:
		return 0
	case *ir.DotExpr:
		return 1
	case *ir.CastExpr:
		return 1
	case *ir.FuncCall:
		return 1
	default:
		return -1
	}
}
