package semantics

import "github.com/jhnl/dingo/ir"

type dependencyVisitor struct {
	ir.BaseVisitor
	exprMode int
	c        *checker
}

func dependencyWalk(c *checker) {
	v := &dependencyVisitor{c: c}
	c.resetWalkState()
	ir.VisitModuleSet(v, c.set)
}

func (v *dependencyVisitor) Module(mod *ir.Module) {
	v.c.mod = mod
	v.c.scope = mod.Scope
	for _, decl := range mod.Decls {
		v.c.setTopDecl(decl)
		ir.VisitDecl(v, decl)
	}
}

func (v *dependencyVisitor) VisitValTopDecl(decl *ir.ValTopDecl) {
	if decl.Type != nil {
		ir.VisitExpr(v, decl.Type)
	}
	if decl.Initializer != nil {
		ir.VisitExpr(v, decl.Initializer)
	}
}

func (v *dependencyVisitor) VisitValDecl(decl *ir.ValDecl) {
	if decl.Type != nil {
		v.exprMode = exprModeType
		ir.VisitExpr(v, decl.Type)
		v.exprMode = exprModeNone
	}
	if decl.Initializer != nil {
		ir.VisitExpr(v, decl.Initializer)
	}
}

func (v *dependencyVisitor) VisitFuncDecl(decl *ir.FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))
	for _, param := range decl.Params {
		ir.VisitExpr(v, param.Type)
	}
	ir.VisitExpr(v, decl.TReturn)
	if decl.Body != nil {
		ir.VisitStmtList(v, decl.Body.Stmts)
	}
}

func (v *dependencyVisitor) VisitStructDecl(decl *ir.StructDecl) {
	for _, f := range decl.Fields {
		v.VisitValDecl(f)
	}
}

func (v *dependencyVisitor) VisitBlockStmt(stmt *ir.BlockStmt) {
	defer setScope(setScope(v.c, stmt.Scope))
	ir.VisitStmtList(v, stmt.Stmts)
}

func (v *dependencyVisitor) VisitDeclStmt(stmt *ir.DeclStmt) {
	ir.VisitDecl(v, stmt.D)
}

func (v *dependencyVisitor) VisitIfStmt(stmt *ir.IfStmt) {
	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		ir.VisitStmt(v, stmt.Else)
	}
}

func (v *dependencyVisitor) VisitWhileStmt(stmt *ir.WhileStmt) {
	v.VisitBlockStmt(stmt.Body)
}

func (v *dependencyVisitor) VisitReturnStmt(stmt *ir.ReturnStmt) {
	if stmt.X != nil {
		ir.VisitExpr(v, stmt.X)
	}
}

func (v *dependencyVisitor) VisitAssignStmt(stmt *ir.AssignStmt) {
	ir.VisitExpr(v, stmt.Left)
	ir.VisitExpr(v, stmt.Right)
}

func (v *dependencyVisitor) VisitExprStmt(stmt *ir.ExprStmt) {
	ir.VisitExpr(v, stmt.X)
}

func (v *dependencyVisitor) VisitArrayTypeExpr(expr *ir.ArrayTypeExpr) ir.Expr {
	ir.VisitExpr(v, expr.Size)
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *dependencyVisitor) VisitBinaryExpr(expr *ir.BinaryExpr) ir.Expr {
	ir.VisitExpr(v, expr.Left)
	ir.VisitExpr(v, expr.Right)
	return expr
}

func (v *dependencyVisitor) VisitUnaryExpr(expr *ir.UnaryExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *dependencyVisitor) VisitStarExpr(expr *ir.StarExpr) ir.Expr {
	if v.exprMode == exprModeType {
		// Don't check dependencies for pointers
		return expr
	}

	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *dependencyVisitor) VisitStructLit(expr *ir.StructLit) ir.Expr {
	ir.VisitExpr(v, expr.Name)
	for _, kv := range expr.Initializers {
		ir.VisitExpr(v, kv.Value)
	}
	return expr
}

func (v *dependencyVisitor) VisitArrayLit(expr *ir.ArrayLit) ir.Expr {
	for _, init := range expr.Initializers {
		ir.VisitExpr(v, init)
	}
	return expr
}

func (v *dependencyVisitor) VisitIdent(expr *ir.Ident) ir.Expr {
	sym := v.c.lookup(expr.Literal())
	if sym != nil {
		if decl, ok := sym.Src.(ir.TopDecl); ok {
			_, isFunc1 := v.c.topDecl.(*ir.FuncDecl)
			_, isFunc2 := decl.(*ir.FuncDecl)

			// Cycle between functions is ok
			if !isFunc1 || !isFunc2 {
				v.c.topDecl.AddDependency(decl)
			}
		}
	}
	return expr
}

func (v *dependencyVisitor) VisitDotExpr(expr *ir.DotExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	v.VisitIdent(expr.Name)
	return expr
}

func (v *dependencyVisitor) VisitCastExpr(expr *ir.CastExpr) ir.Expr {
	ir.VisitExpr(v, expr.ToTyp)
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *dependencyVisitor) VisitFuncCall(expr *ir.FuncCall) ir.Expr {
	ir.VisitExpr(v, expr.X)
	ir.VisitExprList(v, expr.Args)
	return expr
}

func (v *dependencyVisitor) VisitIndexExpr(expr *ir.IndexExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	ir.VisitExpr(v, expr.Index)
	return expr
}
