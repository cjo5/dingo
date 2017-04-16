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

func (p *printer) visitModule(mod *Module) {

}

func (p *printer) visitBlockStmt(stmt *BlockStmt) {

}

func (p *printer) visitPrintStmt(stmt *PrintStmt) {

}

func (p *printer) visitIfStmt(stmt *IfStmt) {

}

func (p *printer) visitWhileStmt(stmt *WhileStmt) {

}

func (p *printer) visitBranchStmt(stmt *BranchStmt) {

}

func (p *printer) visitExprStmt(stmt *ExprStmt) {

}

func (p *printer) visitAssignStmt(stmt *AssignStmt) {

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

func (p *printer) visitIdent(expr *Ident) {
	p.buffer.WriteString(expr.Name.Literal)
}
