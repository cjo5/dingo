package semantics

import "github.com/jhnl/interpreter/token"

type dependencyVisitor struct {
	BaseVisitor
	c *checker
}

func dependencyWalk(c *checker) {
	v := &dependencyVisitor{c: c}
	c.resetWalkState()
	StartProgramWalk(v, c.prog)
}

func (v *dependencyVisitor) Module(mod *Module) {
	v.c.mod = mod
	for _, file := range mod.Files {
		v.c.file = file.Info
		v.c.scope = file.Info.Scope
		VisitDeclList(v, file.Decls)
	}
}

func (v *dependencyVisitor) VisitVarDecl(decl *VarDecl) {
	// Only check top level var decls
	if decl.X != nil && decl.Visibility.OneOf(token.External, token.Internal, token.Restricted) {
		v.c.declSym = decl.Symbol()
		VisitExpr(v, decl.X)
		v.c.declSym = nil
	}
}

func (v *dependencyVisitor) VisitFuncDecl(decl *FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))
	v.c.fun = decl
	v.c.declSym = decl.Symbol()
	VisitStmtList(v, decl.Body.Stmts)
	v.c.declSym = nil
	v.c.fun = nil
}

func (v *dependencyVisitor) VisitBlockStmt(stmt *BlockStmt) {
	defer setScope(setScope(v.c, stmt.Scope))
	VisitStmtList(v, stmt.Stmts)
}

func (v *dependencyVisitor) VisitDeclStmt(stmt *DeclStmt) {
	VisitDecl(v, stmt.D)
}

func (v *dependencyVisitor) VisitPrintStmt(stmt *PrintStmt) {
	VisitExpr(v, stmt.X)
}

func (v *dependencyVisitor) VisitIfStmt(stmt *IfStmt) {
	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		VisitStmt(v, stmt.Else)
	}
}

func (v *dependencyVisitor) VisitWhileStmt(stmt *WhileStmt) {
	v.VisitBlockStmt(stmt.Body)
}

func (v *dependencyVisitor) VisitReturnStmt(stmt *ReturnStmt) {
	if stmt.X != nil {
		VisitExpr(v, stmt.X)
	}
}

func (v *dependencyVisitor) VisitAssignStmt(stmt *AssignStmt) {
	v.VisitIdent(stmt.Name)
	VisitExpr(v, stmt.Right)
}

func (v *dependencyVisitor) VisitExprStmt(stmt *ExprStmt) {
	VisitExpr(v, stmt.X)
}

func (v *dependencyVisitor) VisitBinaryExpr(expr *BinaryExpr) Expr {
	VisitExpr(v, expr.Left)
	VisitExpr(v, expr.Right)
	return expr
}

func (v *dependencyVisitor) VisitUnaryExpr(expr *UnaryExpr) Expr {
	VisitExpr(v, expr.X)
	return expr
}

func (v *dependencyVisitor) VisitIdent(expr *Ident) Expr {
	if v.c.declSym == nil {
		return expr
	}

	sym := v.c.scope.Lookup(expr.Name.Literal)
	if sym != nil && sym.Toplevel() && sym.ID != ModuleSymbol {
		v.c.declSym.dependencies = append(v.c.declSym.dependencies, sym)
	}
	return expr
}

func (v *dependencyVisitor) VisitFuncCall(expr *FuncCall) Expr {
	VisitExprList(v, expr.Args)
	return expr
}

func (v *dependencyVisitor) VisitDotExpr(expr *DotExpr) Expr {
	sym := v.c.scope.Lookup(expr.Name.Literal())
	if sym != nil && sym.ID != ModuleSymbol {
		v.VisitIdent(expr.Name)
		VisitExpr(v, expr.X)
	}
	return expr
}
