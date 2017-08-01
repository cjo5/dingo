package semantics

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
	for _, decl := range mod.Decls {
		v.c.setTopDecl(decl)
		VisitDecl(v, decl)
	}
}

func (v *dependencyVisitor) VisitValTopDecl(decl *ValTopDecl) {
	if decl.Type != nil {
		VisitExpr(v, decl.Type)
	}
	if decl.Initializer != nil {
		VisitExpr(v, decl.Initializer)
	}
}

func (v *dependencyVisitor) VisitValDecl(decl *ValDecl) {
	if decl.Type != nil {
		VisitExpr(v, decl.Type)
	}
	if decl.Initializer != nil {
		VisitExpr(v, decl.Initializer)
	}
}

func (v *dependencyVisitor) VisitFuncDecl(decl *FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))
	for _, param := range decl.Params {
		VisitExpr(v, param.Type)
	}
	VisitExpr(v, decl.TReturn)
	VisitStmtList(v, decl.Body.Stmts)
}

func (v *dependencyVisitor) VisitStructDecl(decl *StructDecl) {
	for _, f := range decl.Fields {
		v.VisitValDecl(f)
	}
}

func (v *dependencyVisitor) VisitBlockStmt(stmt *BlockStmt) {
	defer setScope(setScope(v.c, stmt.Scope))
	VisitStmtList(v, stmt.Stmts)
}

func (v *dependencyVisitor) VisitDeclStmt(stmt *DeclStmt) {
	VisitDecl(v, stmt.D)
}

func (v *dependencyVisitor) VisitPrintStmt(stmt *PrintStmt) {
	for _, x := range stmt.Xs {
		VisitExpr(v, x)
	}
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
	VisitExpr(v, stmt.Left)
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
func (v *dependencyVisitor) VisitStructLit(expr *StructLit) Expr {
	VisitExpr(v, expr.Name)
	for _, kv := range expr.Initializers {
		VisitExpr(v, kv.Value)
	}
	return expr
}

func (v *dependencyVisitor) VisitIdent(expr *Ident) Expr {
	sym := v.c.lookup(expr.Literal())
	if sym != nil {
		if decl, ok := sym.Src.(TopDecl); ok {
			_, isFunc1 := v.c.topDecl.(*FuncDecl)
			_, isFunc2 := decl.(*FuncDecl)

			// Cycle between functions is ok
			if !isFunc1 || !isFunc2 {
				v.c.topDecl.addDependency(decl)
			}
		}
	}
	return expr
}

func (v *dependencyVisitor) VisitDotIdent(expr *DotIdent) Expr {
	VisitExpr(v, expr.X)
	v.VisitIdent(expr.Name)
	return expr
}

func (v *dependencyVisitor) VisitFuncCall(expr *FuncCall) Expr {
	VisitExpr(v, expr.X)
	VisitExprList(v, expr.Args)
	return expr
}
