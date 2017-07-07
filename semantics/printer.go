package semantics

import (
	"bytes"

	"fmt"

	"github.com/jhnl/interpreter/token"
)

type printer struct {
	BaseVisitor
	buffer bytes.Buffer
	level  int
}

// Print walks the ast in pre-order and generates a text representation of it.
func Print(n Node) string {
	p := &printer{}

	VisitNode(p, n)
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

func (p *printer) visitModule(mod *File) {
	p.printf("[module]")
	for _, d := range mod.Decls {
		VisitDecl(p, d)
	}
}

func (p *printer) visitImport(decl *Import) {
	defer dec(inc(p))
	p.printToken(decl.Import)
	p.printToken(decl.Path)
}

func (p *printer) visitBlockStmt(stmt *BlockStmt) {
	defer dec(inc(p))
	p.print("BLOCK")
	VisitStmtList(p, stmt.Stmts)
}

func (p *printer) visitDeclStmt(stmt *DeclStmt) {
	VisitDecl(p, stmt.D)
}

func (p *printer) visitVarDecl(decl *VarDecl) {
	defer dec(inc(p))
	p.printToken(decl.Decl)
	p.visitIdent(decl.Name)
	VisitExpr(p, decl.X)
}

func (p *printer) printField(field *Field) {
	defer dec(inc(p))
	p.print("FIELD")
	p.visitIdent(field.Name)
	p.visitIdent(field.Type)
}

func (p *printer) visitFuncDecl(decl *FuncDecl) {
	defer dec(inc(p))
	p.printToken(decl.Decl)
	p.visitIdent(decl.Name)
	p.level++
	p.print("PARAMS")
	for _, param := range decl.Params {
		p.printField(param)
	}
	p.print("RETURN")
	if decl.Return != nil {
		p.visitIdent(decl.Return)
	}
	p.level--
	p.visitBlockStmt(decl.Body)
}

func (p *printer) printStructField(field *StructField) {
	defer dec(inc(p))
	p.printToken(field.Qualifier)
	p.visitIdent(field.Name)
	VisitExpr(p, field.Type)
}

func (p *printer) visitStructDecl(decl *StructDecl) {
	defer dec(inc(p))
	p.printToken(decl.Decl)
	p.visitIdent(decl.Name)
	p.level++
	p.print("FIELDS")
	for _, field := range decl.Fields {
		p.printStructField(field)
	}
	p.level--
}

func (p *printer) visitPrintStmt(stmt *PrintStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Print)
	VisitExpr(p, stmt.X)
}

func (p *printer) visitIfStmt(stmt *IfStmt) {
	defer dec(inc(p))
	p.printToken(stmt.If)
	p.level++
	p.print("COND")
	VisitExpr(p, stmt.Cond)
	p.level--
	p.visitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		p.print("ELIF/ELSE")
		VisitStmt(p, stmt.Else)
	}
}

func (p *printer) visitWhileStmt(stmt *WhileStmt) {
	defer dec(inc(p))
	p.printToken(stmt.While)
	p.level++
	p.print("COND")
	VisitExpr(p, stmt.Cond)
	p.level--
	p.visitBlockStmt(stmt.Body)
}

func (p *printer) visitReturnStmt(stmt *ReturnStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Return)
	if stmt.X != nil {
		VisitExpr(p, stmt.X)
	}
}

func (p *printer) visitBranchStmt(stmt *BranchStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Tok)
}

func (p *printer) visitAssignStmt(stmt *AssignStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Assign)
	p.visitIdent(stmt.Name)
	VisitExpr(p, stmt.Right)
}

func (p *printer) visitExprStmt(stmt *ExprStmt) {
	defer dec(inc(p))
	VisitExpr(p, stmt.X)
}

func (p *printer) visitBinaryExpr(expr *BinaryExpr) Expr {
	defer dec(inc(p))
	p.printToken(expr.Op)
	VisitExpr(p, expr.Left)
	VisitExpr(p, expr.Right)
	return expr
}

func (p *printer) visitUnaryExpr(expr *UnaryExpr) Expr {
	defer dec(inc(p))
	p.printToken(expr.Op)
	VisitExpr(p, expr.X)
	return expr
}

func (p *printer) visitLiteral(expr *Literal) Expr {
	defer dec(inc(p))
	p.printToken(expr.Value)
	return expr
}

func (p *printer) visitStructLiteral(expr *StructLiteral) Expr {
	panic("visitStructLiteral not implemented")
}

func (p *printer) visitIdent(expr *Ident) Expr {
	defer dec(inc(p))
	p.printToken(expr.Name)
	return expr
}

func (p *printer) visitFuncCall(expr *FuncCall) Expr {
	defer dec(inc(p))
	p.print("FUNCCALL")
	p.visitIdent(expr.Name)
	for _, arg := range expr.Args {
		VisitExpr(p, arg)
	}
	return expr
}

func (p *printer) visitDotExpr(expr *DotExpr) Expr {
	defer dec(inc(p))
	p.print("DOT")
	p.visitIdent(expr.Name)
	VisitExpr(p, expr.X)
	return expr
}
