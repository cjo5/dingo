package semantics

import (
	"bytes"

	"fmt"

	"github.com/jhnl/interpreter/token"
)

type astPrinter struct {
	BaseVisitor
	buffer bytes.Buffer
	level  int
}

// PrintAst pre-order.
func PrintAst(n Node) string {
	p := &astPrinter{}
	StartWalk(p, n)
	return p.buffer.String()
}

func inc(p *astPrinter) *astPrinter {
	p.level++
	return p
}

func dec(p *astPrinter) {
	p.level--
}

func (p *astPrinter) indent() {
	for i := 0; i < p.level; i++ {
		p.buffer.WriteByte(' ')
	}
}

func (p *astPrinter) newline() {
	p.buffer.WriteByte('\n')
}

func (p *astPrinter) printf(msg string, args ...interface{}) {
	p.indent()
	p.buffer.WriteString(fmt.Sprintf(msg, args...))
	p.newline()
}

func (p *astPrinter) print(msg string) {
	p.printf("[%s]", msg)
}

func (p *astPrinter) printToken(tok token.Token) {
	p.printf("[%s]", tok)
}

func (p *astPrinter) Module(mod *Module) {
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

func (p *astPrinter) VisitImport(decl *Import) {
	defer dec(inc(p))
	p.printToken(decl.Import)
	defer dec(inc(p))
	p.printToken(decl.Literal)
}

func (p *astPrinter) VisitBlockStmt(stmt *BlockStmt) {
	defer dec(inc(p))
	p.print("BLOCK")
	VisitStmtList(p, stmt.Stmts)
}

func (p *astPrinter) VisitDeclStmt(stmt *DeclStmt) {
	VisitDecl(p, stmt.D)
}

func (p *astPrinter) VisitValTopDecl(decl *ValTopDecl) {
	defer dec(inc(p))
	p.printToken(decl.Visibility)
	defer dec(inc(p))
	p.visitValDeclSpec(&decl.ValDeclSpec)
}

func (p *astPrinter) VisitValDecl(decl *ValDecl) {
	defer dec(inc(p))
	p.visitValDeclSpec(&decl.ValDeclSpec)
}

func (p *astPrinter) visitValDeclSpec(decl *ValDeclSpec) {
	p.printToken(decl.Decl)
	p.printToken(decl.Name)
	if decl.Type != nil {
		VisitExpr(p, decl.Type)
	}
	if decl.Initializer != nil {
		VisitExpr(p, decl.Initializer)
	}
}

func (p *astPrinter) VisitFuncDecl(decl *FuncDecl) {
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

func (p *astPrinter) VisitStructDecl(decl *StructDecl) {
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

func (p *astPrinter) VisitPrintStmt(stmt *PrintStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Print)
	for _, x := range stmt.Xs {
		VisitExpr(p, x)
	}
}

func (p *astPrinter) VisitIfStmt(stmt *IfStmt) {
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

func (p *astPrinter) VisitWhileStmt(stmt *WhileStmt) {
	defer dec(inc(p))
	p.printToken(stmt.While)
	p.level++
	p.print("COND")
	VisitExpr(p, stmt.Cond)
	p.level--
	p.VisitBlockStmt(stmt.Body)
}

func (p *astPrinter) VisitReturnStmt(stmt *ReturnStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Return)
	if stmt.X != nil {
		VisitExpr(p, stmt.X)
	}
}

func (p *astPrinter) VisitBranchStmt(stmt *BranchStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Tok)
}

func (p *astPrinter) VisitAssignStmt(stmt *AssignStmt) {
	defer dec(inc(p))
	p.printToken(stmt.Assign)
	VisitExpr(p, stmt.Left)
	VisitExpr(p, stmt.Right)
}

func (p *astPrinter) VisitExprStmt(stmt *ExprStmt) {
	defer dec(inc(p))
	VisitExpr(p, stmt.X)
}

func (p *astPrinter) VisitBinaryExpr(expr *BinaryExpr) Expr {
	defer dec(inc(p))
	p.printToken(expr.Op)
	VisitExpr(p, expr.Left)
	VisitExpr(p, expr.Right)
	return expr
}

func (p *astPrinter) VisitUnaryExpr(expr *UnaryExpr) Expr {
	defer dec(inc(p))
	p.printToken(expr.Op)
	VisitExpr(p, expr.X)
	return expr
}

func (p *astPrinter) VisitBasicLit(expr *BasicLit) Expr {
	defer dec(inc(p))
	p.printToken(expr.Value)
	return expr
}

func (p *astPrinter) VisitStructLit(expr *StructLit) Expr {
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

func (p *astPrinter) VisitIdent(expr *Ident) Expr {
	defer dec(inc(p))
	p.printToken(expr.Name)
	return expr
}

func (p *astPrinter) VisitDotIdent(expr *DotIdent) Expr {
	defer dec(inc(p))
	p.print("DOT")
	VisitExpr(p, expr.X)
	p.VisitIdent(expr.Name)
	return expr
}

func (p *astPrinter) VisitFuncCall(expr *FuncCall) Expr {
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
