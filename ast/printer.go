package ast

import (
	"bytes"

	"fmt"

	"github.com/jhnl/interpreter/token"
)

type printer struct {
	buffer bytes.Buffer
	level  int
}

func Print(n Node) string {
	p := &printer{}

	n.Accept(p)
	return p.buffer.String()
}

func inc(p *printer) *printer {
	p.level++
	return p
}

func dec(p *printer) {
	p.level--
}

func (p *printer) indent() {
	for i := 0; i < p.level; i++ {
		p.buffer.WriteByte(' ')
	}
}

func (p *printer) newline() {
	p.buffer.WriteByte('\n')
}

func (p *printer) printf(msg string, args ...interface{}) {
	p.indent()
	p.buffer.WriteString(fmt.Sprintf(msg, args...))
	p.newline()
}

func (p *printer) print(msg string) {
	p.printf("[%s]", msg)
}

func (p *printer) printToken(tok token.Token) {
	p.printf("[%v]", tok)
}

func (p *printer) visitModule(mod *Module) {
	if mod.Name != nil {
		p.printf("[module %v]", mod.Name.Name)
	}
	for _, s := range mod.Stmts {
		s.Accept(p)
	}
}

func (p *printer) visitBlockStmt(stmt *BlockStmt) {
	for _, s := range stmt.Stmts {
		s.Accept(p)
	}
}

func (p *printer) visitPrintStmt(stmt *PrintStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Print)
	stmt.X.Accept(p)
}

func (p *printer) visitIfStmt(stmt *IfStmt) {
	defer dec(inc(p))

	p.printToken(stmt.If)
	p.print("COND")
	stmt.Cond.Accept(p)
	p.print("BODY")
	stmt.Body.Accept(p)

	if stmt.Else != nil {
		p.print("ELSE/ELIF")
		stmt.Else.Accept(p)
	}
}

func (p *printer) visitWhileStmt(stmt *WhileStmt) {
	defer dec(inc(p))

	p.printToken(stmt.While)
	p.print("COND")
	stmt.Cond.Accept(p)
	p.print("BODY")
	stmt.Body.Accept(p)
}

func (p *printer) visitBranchStmt(stmt *BranchStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Tok)
}

func (p *printer) visitExprStmt(stmt *ExprStmt) {
	defer dec(inc(p))
	stmt.X.Accept(p)
}

func (p *printer) visitAssignStmt(stmt *AssignStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Assign)
	stmt.Left.Accept(p)
	stmt.Right.Accept(p)
}

func (p *printer) visitBinary(expr *BinaryExpr) {
	defer dec(inc(p))
	p.printToken(expr.Op)
	expr.Left.Accept(p)
	expr.Right.Accept(p)
}

func (p *printer) visitUnary(expr *UnaryExpr) {
	defer dec(inc(p))
	p.printToken(expr.Op)
	expr.X.Accept(p)
}

func (p *printer) visitLiteral(expr *Literal) {
	defer dec(inc(p))
	p.printToken(expr.Value)
}

func (p *printer) visitIdent(expr *Ident) {
	defer dec(inc(p))
	p.printToken(expr.Name)
}
