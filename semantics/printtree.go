package semantics

import (
	"bytes"

	"fmt"

	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

type treePrinter struct {
	ir.BaseVisitor
	buffer bytes.Buffer
	level  int
}

func inc(p *treePrinter) *treePrinter {
	p.level++
	return p
}

func dec(p *treePrinter) {
	p.level--
}

func (p *treePrinter) indent() {
	for i := 0; i < p.level; i++ {
		p.buffer.WriteByte(' ')
	}
}

func (p *treePrinter) newline() {
	p.buffer.WriteByte('\n')
}

func (p *treePrinter) printf(msg string, args ...interface{}) {
	p.indent()
	p.buffer.WriteString(fmt.Sprintf(msg, args...))
	p.newline()
}

func (p *treePrinter) print(msg string) {
	p.printf("[%s]", msg)
}

func (p *treePrinter) printToken(tok token.Token) {
	p.printf("[%s]", tok)
}

func (p *treePrinter) Module(mod *ir.Module) {
	p.printf("[module %s]", mod.Name.Literal)
	defer dec(inc(p))
	for _, file := range mod.Files {
		inc(p)
		p.printf("[file %s]", file.Ctx.Path)
		ir.VisitIncludeList(p, file.Includes)
		dec(p)
	}
	for _, decl := range mod.Decls {
		ir.VisitDecl(p, decl)
	}
}

func (p *treePrinter) VisitInclude(decl *ir.Include) {
	defer dec(inc(p))
	p.printToken(decl.Include)
	defer dec(inc(p))
	p.printToken(decl.Literal)
}

func (p *treePrinter) VisitBlockStmt(stmt *ir.BlockStmt) {
	defer dec(inc(p))
	p.print("BLOCK")
	ir.VisitStmtList(p, stmt.Stmts)
}

func (p *treePrinter) VisitDeclStmt(stmt *ir.DeclStmt) {
	ir.VisitDecl(p, stmt.D)
}

func (p *treePrinter) VisitValTopDecl(decl *ir.ValTopDecl) {
	defer dec(inc(p))
	p.printToken(decl.Visibility)
	defer dec(inc(p))
	p.visitValDeclSpec(&decl.ValDeclSpec)
}

func (p *treePrinter) VisitValDecl(decl *ir.ValDecl) {
	defer dec(inc(p))
	p.visitValDeclSpec(&decl.ValDeclSpec)
}

func (p *treePrinter) visitValDeclSpec(decl *ir.ValDeclSpec) {
	p.printToken(decl.Decl)
	p.printToken(decl.Name)
	if decl.Type != nil {
		ir.VisitExpr(p, decl.Type)
	}
	if decl.Initializer != nil {
		ir.VisitExpr(p, decl.Initializer)
	}
}

func (p *treePrinter) VisitFuncDecl(decl *ir.FuncDecl) {
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
		ir.VisitExpr(p, decl.TReturn)
	}
	p.level--
	p.VisitBlockStmt(decl.Body)
}

func (p *treePrinter) VisitStructDecl(decl *ir.StructDecl) {
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

func (p *treePrinter) VisitPrintStmt(stmt *ir.PrintStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Print)
	for _, x := range stmt.Xs {
		ir.VisitExpr(p, x)
	}
}

func (p *treePrinter) VisitIfStmt(stmt *ir.IfStmt) {
	defer dec(inc(p))
	p.printToken(stmt.If)
	p.level++
	p.print("COND")
	ir.VisitExpr(p, stmt.Cond)
	p.level--
	p.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		p.print("ELIF/ELSE")
		ir.VisitStmt(p, stmt.Else)
	}
}

func (p *treePrinter) VisitWhileStmt(stmt *ir.WhileStmt) {
	defer dec(inc(p))
	p.printToken(stmt.While)
	p.level++
	p.print("COND")
	ir.VisitExpr(p, stmt.Cond)
	p.level--
	p.VisitBlockStmt(stmt.Body)
}

func (p *treePrinter) VisitReturnStmt(stmt *ir.ReturnStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Return)
	if stmt.X != nil {
		ir.VisitExpr(p, stmt.X)
	}
}

func (p *treePrinter) VisitBranchStmt(stmt *ir.BranchStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Tok)
}

func (p *treePrinter) VisitAssignStmt(stmt *ir.AssignStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Assign)
	ir.VisitExpr(p, stmt.Left)
	ir.VisitExpr(p, stmt.Right)
}

func (p *treePrinter) VisitExprStmt(stmt *ir.ExprStmt) {
	defer dec(inc(p))
	ir.VisitExpr(p, stmt.X)
}

func (p *treePrinter) VisitBinaryExpr(expr *ir.BinaryExpr) ir.Expr {
	defer dec(inc(p))
	p.printToken(expr.Op)
	ir.VisitExpr(p, expr.Left)
	ir.VisitExpr(p, expr.Right)
	return expr
}

func (p *treePrinter) VisitUnaryExpr(expr *ir.UnaryExpr) ir.Expr {
	defer dec(inc(p))
	p.printToken(expr.Op)
	ir.VisitExpr(p, expr.X)
	return expr
}

func (p *treePrinter) VisitBasicLit(expr *ir.BasicLit) ir.Expr {
	defer dec(inc(p))
	p.printToken(expr.Value)
	return expr
}

func (p *treePrinter) VisitStructLit(expr *ir.StructLit) ir.Expr {
	defer dec(inc(p))
	p.printf("[struct literal]")
	defer dec(inc(p))
	ir.VisitExpr(p, expr.Name)
	for _, kv := range expr.Initializers {
		p.level++
		p.printToken(kv.Key)
		ir.VisitExpr(p, kv.Value)
		p.level--
	}
	return expr
}

func (p *treePrinter) VisitIdent(expr *ir.Ident) ir.Expr {
	defer dec(inc(p))
	p.printToken(expr.Name)
	return expr
}

func (p *treePrinter) VisitDotExpr(expr *ir.DotExpr) ir.Expr {
	defer dec(inc(p))
	p.print("DOT")
	ir.VisitExpr(p, expr.X)
	p.VisitIdent(expr.Name)
	return expr
}

func (p *treePrinter) VisitCast(expr *ir.Cast) ir.Expr {
	defer dec(inc(p))
	p.printToken(expr.Cast)
	ir.VisitExpr(p, expr.ToTyp)
	ir.VisitExpr(p, expr.X)
	return expr
}

func (p *treePrinter) VisitFuncCall(expr *ir.FuncCall) ir.Expr {
	defer dec(inc(p))
	p.print("FUNCCALL")
	ir.VisitExpr(p, expr.X)
	defer dec(inc(p))
	p.print("ARGS")
	for _, arg := range expr.Args {
		ir.VisitExpr(p, arg)
	}
	return expr
}
