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

// Print walks the ast in pre-order and generates a text representation of it.
func Print(n Node) string {
	p := &printer{}

	p.walk(n)
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
	p.printf("[%s]", tok)
}

func (p *printer) walk(n Node) {
	switch t := n.(type) {
	case *Module:
		p.printModule(t)
	case *BlockStmt:
		p.printBlockStmt(t)
	case *VarDecl:
		p.printVarDecl(t)
	case *FuncDecl:
		p.printFuncDecl(t)
	case *PrintStmt:
		p.printPrintStmt(t)
	case *IfStmt:
		p.printIfStmt(t)
	case *WhileStmt:
		p.printWhileStmt(t)
	case *ReturnStmt:
		p.printReturnStmt(t)
	case *BranchStmt:
		p.printBranchStmt(t)
	case *AssignStmt:
		p.printAssignStmt(t)
	case *ExprStmt:
		p.printExprStmt(t)
	case *BinaryExpr:
		p.printBinary(t)
	case *UnaryExpr:
		p.printUnary(t)
	case *Literal:
		p.printLiteral(t)
	case *Ident:
		p.printIdent(t)
	case *CallExpr:
		p.printCallExpr(t)
	}
}

func (p *printer) printModule(mod *Module) {
	if mod.Name != nil {
		p.printf("[module %v]", mod.Name.Name)
	}
	for _, s := range mod.Stmts {
		p.walk(s)
	}
}

func (p *printer) printBlockStmt(stmt *BlockStmt) {
	defer dec(inc(p))
	p.print("BLOCK")
	for _, s := range stmt.Stmts {
		p.walk(s)
	}
}

func (p *printer) printVarDecl(stmt *VarDecl) {
	defer dec(inc(p))
	p.printToken(stmt.Decl)
	p.walk(stmt.Name)
	p.walk(stmt.X)
}

func (p *printer) printFuncDecl(stmt *FuncDecl) {
	defer dec(inc(p))
	p.printToken(stmt.Decl)
	p.walk(stmt.Name)
	p.level++
	p.print("FIELDS")
	for _, field := range stmt.Fields {
		p.walk(field)
	}
	p.level--
	p.walk(stmt.Body)
}

func (p *printer) printPrintStmt(stmt *PrintStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Print)
	p.walk(stmt.X)
}

func (p *printer) printIfStmt(stmt *IfStmt) {
	defer dec(inc(p))
	p.printToken(stmt.If)
	p.level++
	p.print("COND")
	p.walk(stmt.Cond)
	p.level--
	p.walk(stmt.Body)
	if stmt.Else != nil {
		p.print("ELIF/ELSE")
		p.walk(stmt.Else)
	}
}

func (p *printer) printWhileStmt(stmt *WhileStmt) {
	defer dec(inc(p))
	p.printToken(stmt.While)
	p.level++
	p.print("COND")
	p.walk(stmt.Cond)
	p.level--
	p.walk(stmt.Body)
}

func (p *printer) printReturnStmt(stmt *ReturnStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Return)
	if stmt.X != nil {
		p.walk(stmt.X)
	}
}

func (p *printer) printBranchStmt(stmt *BranchStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Tok)
}

func (p *printer) printAssignStmt(stmt *AssignStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Assign)
	p.walk(stmt.Name)
	p.walk(stmt.Right)
}

func (p *printer) printExprStmt(stmt *ExprStmt) {
	defer dec(inc(p))
	p.walk(stmt.X)
}

func (p *printer) printBinary(expr *BinaryExpr) {
	defer dec(inc(p))
	p.printToken(expr.Op)
	p.walk(expr.Left)
	p.walk(expr.Right)
}

func (p *printer) printUnary(expr *UnaryExpr) {
	defer dec(inc(p))
	p.printToken(expr.Op)
	p.walk(expr.X)
}

func (p *printer) printLiteral(expr *Literal) {
	defer dec(inc(p))
	p.printToken(expr.Value)
}

func (p *printer) printIdent(expr *Ident) {
	defer dec(inc(p))
	p.printToken(expr.Name)
}

func (p *printer) printCallExpr(expr *CallExpr) {
	defer dec(inc(p))
	p.print("CALL")
	p.walk(expr.Name)
	for _, arg := range expr.Args {
		p.walk(arg)
	}
}
