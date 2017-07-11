package semantics

import (
	"bytes"

	"fmt"

	"github.com/jhnl/interpreter/token"
)

type printVisitor struct {
	BaseVisitor
	buffer bytes.Buffer
	level  int
}

// Print walks the ast in pre-order and generates a text representation of it.
func Print(n Node) string {
	p := &printVisitor{}
	StartWalk(p, n)
	return p.buffer.String()
}

func inc(p *printVisitor) *printVisitor {
	p.level++
	return p
}

func dec(p *printVisitor) {
	p.level--
}

func (p *printVisitor) indent() {
	for i := 0; i < p.level; i++ {
		p.buffer.WriteByte(' ')
	}
}

func (p *printVisitor) newline() {
	p.buffer.WriteByte('\n')
}

func (p *printVisitor) printf(msg string, args ...interface{}) {
	p.indent()
	p.buffer.WriteString(fmt.Sprintf(msg, args...))
	p.newline()
}

func (p *printVisitor) print(msg string) {
	p.printf("[%s]", msg)
}

func (p *printVisitor) printToken(tok token.Token) {
	p.printf("[%s]", tok)
}

func (p *printVisitor) Module(mod *Module) {
	defer dec(inc(p))
	p.printf("[module %s]", mod.Name.Literal)
	for _, file := range mod.Files {
		defer dec(inc(p))
		p.printf("[file]")
		VisitImportList(p, file.Info.Imports)
		VisitDeclList(p, file.Decls)
	}
}

func (p *printVisitor) VisitImport(decl *Import) {
	defer dec(inc(p))
	p.printToken(decl.Import)
	p.printToken(decl.Literal)
}

func (p *printVisitor) VisitBlockStmt(stmt *BlockStmt) {
	defer dec(inc(p))
	p.print("BLOCK")
	VisitStmtList(p, stmt.Stmts)
}

func (p *printVisitor) VisitDeclStmt(stmt *DeclStmt) {
	VisitDecl(p, stmt.D)
}

func (p *printVisitor) VisitVarDecl(decl *VarDecl) {
	defer dec(inc(p))
	p.printToken(decl.Decl)
	p.VisitIdent(decl.Name)
	p.VisitIdent(decl.Type)
	VisitExpr(p, decl.X)
}

func (p *printVisitor) printField(field *Field) {
	defer dec(inc(p))
	p.print("FIELD")
	p.VisitIdent(field.Name)
	p.VisitIdent(field.Type)
}

func (p *printVisitor) VisitFuncDecl(decl *FuncDecl) {
	defer dec(inc(p))
	p.printToken(decl.Decl)
	p.VisitIdent(decl.Name)
	p.level++
	p.print("PARAMS")
	for _, param := range decl.Params {
		p.printField(param)
	}
	p.print("RETURN")
	if decl.Return != nil {
		p.VisitIdent(decl.Return)
	}
	p.level--
	p.VisitBlockStmt(decl.Body)
}

func (p *printVisitor) printStructField(field *StructField) {
	defer dec(inc(p))
	p.printToken(field.Qualifier)
	p.VisitIdent(field.Name)
	VisitExpr(p, field.Type)
}

func (p *printVisitor) VisitStructDecl(decl *StructDecl) {
	defer dec(inc(p))
	p.printToken(decl.Decl)
	p.VisitIdent(decl.Name)
	p.level++
	p.print("FIELDS")
	for _, field := range decl.Fields {
		p.printStructField(field)
	}
	p.level--
}

func (p *printVisitor) VisitPrintStmt(stmt *PrintStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Print)
	VisitExpr(p, stmt.X)
}

func (p *printVisitor) VisitIfStmt(stmt *IfStmt) {
	defer dec(inc(p))
	p.printToken(stmt.If)
	p.level++
	p.print("COND")
	VisitExpr(p, stmt.Cond)
	p.level--
	p.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		p.print("ELIF/ELSE")
		VisitStmt(p, stmt.Else)
	}
}

func (p *printVisitor) VisitWhileStmt(stmt *WhileStmt) {
	defer dec(inc(p))
	p.printToken(stmt.While)
	p.level++
	p.print("COND")
	VisitExpr(p, stmt.Cond)
	p.level--
	p.VisitBlockStmt(stmt.Body)
}

func (p *printVisitor) VisitReturnStmt(stmt *ReturnStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Return)
	if stmt.X != nil {
		VisitExpr(p, stmt.X)
	}
}

func (p *printVisitor) VisitBranchStmt(stmt *BranchStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Tok)
}

func (p *printVisitor) VisitAssignStmt(stmt *AssignStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Assign)
	p.VisitIdent(stmt.Name)
	VisitExpr(p, stmt.Right)
}

func (p *printVisitor) VisitExprStmt(stmt *ExprStmt) {
	defer dec(inc(p))
	VisitExpr(p, stmt.X)
}

func (p *printVisitor) VisitBinaryExpr(expr *BinaryExpr) Expr {
	defer dec(inc(p))
	p.printToken(expr.Op)
	VisitExpr(p, expr.Left)
	VisitExpr(p, expr.Right)
	return expr
}

func (p *printVisitor) VisitUnaryExpr(expr *UnaryExpr) Expr {
	defer dec(inc(p))
	p.printToken(expr.Op)
	VisitExpr(p, expr.X)
	return expr
}

func (p *printVisitor) VisitLiteral(expr *Literal) Expr {
	defer dec(inc(p))
	p.printToken(expr.Value)
	return expr
}

func (p *printVisitor) VisitStructLiteral(expr *StructLiteral) Expr {
	panic("VisitStructLiteral not implemented")
}

func (p *printVisitor) VisitIdent(expr *Ident) Expr {
	defer dec(inc(p))
	p.printToken(expr.Name)
	return expr
}

func (p *printVisitor) VisitFuncCall(expr *FuncCall) Expr {
	defer dec(inc(p))
	p.print("FUNCCALL")
	p.VisitIdent(expr.Name)
	for _, arg := range expr.Args {
		VisitExpr(p, arg)
	}
	return expr
}

func (p *printVisitor) VisitDotExpr(expr *DotExpr) Expr {
	defer dec(inc(p))
	p.print("DOT")
	p.VisitIdent(expr.Name)
	VisitExpr(p, expr.X)
	return expr
}
