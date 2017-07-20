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
	defer dec(inc(p))
	for _, file := range mod.Files {
		p.printf("[file %s]", file.Ctx.Path)
		VisitImportList(p, file.Imports)
	}
	for _, decl := range mod.Decls {
		VisitDecl(p, decl)
	}
}

func (p *printVisitor) VisitImport(decl *Import) {
	defer dec(inc(p))
	p.printToken(decl.Import)
	defer dec(inc(p))
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

func (p *printVisitor) VisitValTopDecl(decl *ValTopDecl) {
	defer dec(inc(p))
	p.printToken(decl.Visibility)
	defer dec(inc(p))
	p.visitValDeclSpec(&decl.ValDeclSpec)
}

func (p *printVisitor) VisitValDecl(decl *ValDecl) {
	defer dec(inc(p))
	p.visitValDeclSpec(&decl.ValDeclSpec)
}

func (p *printVisitor) visitValDeclSpec(decl *ValDeclSpec) {
	p.printToken(decl.Decl)
	p.printToken(decl.Name)
	VisitExpr(p, decl.Type)
	if decl.Initializer != nil {
		VisitExpr(p, decl.Initializer)
	}
}

func (p *printVisitor) VisitFuncDecl(decl *FuncDecl) {
	defer dec(inc(p))
	p.printToken(decl.Decl)
	defer dec(inc(p))
	p.printToken(decl.Name)
	p.level++
	p.print("PARAMS")
	for _, param := range decl.Params {
		p.VisitValDecl(param)
	}
	p.print("RETURN")
	if decl.TReturn != nil {
		VisitExpr(p, decl.TReturn)
	}
	p.level--
	p.VisitBlockStmt(decl.Body)
}

func (p *printVisitor) VisitStructDecl(decl *StructDecl) {
	defer dec(inc(p))
	p.printToken(decl.Decl)
	defer dec(inc(p))
	p.printToken(decl.Name)
	defer dec(inc(p))
	p.print("FIELDS")
	for _, field := range decl.Fields {
		p.VisitValDecl(field)
	}
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
	VisitExpr(p, stmt.Left)
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
	defer dec(inc(p))
	p.printf("[struct literal]")
	defer dec(inc(p))
	VisitExpr(p, expr.Name)
	for _, kv := range expr.Initializers {
		p.level++
		p.printToken(kv.Key)
		VisitExpr(p, kv.Value)
		p.level--
	}
	return expr
}

func (p *printVisitor) VisitIdent(expr *Ident) Expr {
	defer dec(inc(p))
	p.printToken(expr.Name)
	return expr
}

func (p *printVisitor) VisitDotIdent(expr *DotIdent) Expr {
	defer dec(inc(p))
	p.print("DOT")
	VisitExpr(p, expr.X)
	p.VisitIdent(expr.Name)
	return expr
}

func (p *printVisitor) VisitFuncCall(expr *FuncCall) Expr {
	defer dec(inc(p))
	p.print("FUNCCALL")
	VisitExpr(p, expr.X)
	defer dec(inc(p))
	p.print("ARGS")
	for _, arg := range expr.Args {
		VisitExpr(p, arg)
	}
	return expr
}
