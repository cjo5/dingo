package semantics

type symbolVisitor struct {
	BaseVisitor
	c *checker
}

func symbolWalk(c *checker) {
	v := &symbolVisitor{c: c}
	StartProgramWalk(v, c.prog)
	c.resetWalkState()
}

func (v *symbolVisitor) Module(mod *Module) {
	v.c.openScope(TopScope)
	mod.Public = v.c.scope
	v.c.openScope(TopScope)
	mod.Internal = v.c.scope
	v.c.mod = mod
	for _, file := range mod.Files {
		v.c.openScope(TopScope)
		file.Ctx.Scope = v.c.scope
		v.c.fileCtx = file.Ctx
		VisitImportList(v, file.Imports)
		v.c.closeScope()
	}
	for _, decl := range mod.Decls {
		v.c.setTopDecl(decl)
		VisitDecl(v, decl)

	}
	v.c.closeScope() // Internal
	v.c.closeScope() // External
}

func (v *symbolVisitor) VisitImport(decl *Import) {
	sym := v.c.insert(v.c.fileScope(), ModuleSymbol, decl.Mod.Name.Literal, decl.Literal.Pos, decl)
	if sym != nil {
		sym.T = NewModuleType(decl.Mod.ID, decl.Mod.Public)
	}
}

func (v *symbolVisitor) VisitValTopDecl(decl *ValTopDecl) {
	scope := v.c.visibilityScope(decl.Visibility)
	decl.Sym = v.c.insert(scope, ValSymbol, decl.Name.Literal, decl.Name.Pos, decl)
}

func (v *symbolVisitor) VisitValDecl(decl *ValDecl) {
	decl.Sym = v.c.insert(v.c.scope, ValSymbol, decl.Name.Literal, decl.Name.Pos, decl)
}

func (v *symbolVisitor) VisitFuncDecl(decl *FuncDecl) {
	scope := v.c.visibilityScope(decl.Visibility)
	decl.Sym = v.c.insert(scope, FuncSymbol, decl.Name.Literal, decl.Name.Pos, decl)
	v.c.openScope(LocalScope)
	decl.Scope = v.c.scope

	for _, param := range decl.Params {
		v.VisitValDecl(param)
	}

	decl.Body.Scope = decl.Scope
	VisitStmtList(v, decl.Body.Stmts)
	v.c.closeScope()
}

func (v *symbolVisitor) VisitStructDecl(decl *StructDecl) {
	scope := v.c.visibilityScope(decl.Visibility)
	decl.Sym = v.c.insert(scope, TypeSymbol, decl.Name.Literal, decl.Name.Pos, decl)
	v.c.openScope(FieldScope)
	decl.Scope = v.c.scope

	for _, field := range decl.Fields {
		v.VisitValDecl(field)
	}

	v.c.closeScope()
}

func (v *symbolVisitor) VisitBlockStmt(stmt *BlockStmt) {
	v.c.openScope(LocalScope)
	stmt.Scope = v.c.scope
	VisitStmtList(v, stmt.Stmts)
	v.c.closeScope()
}

func (v *symbolVisitor) VisitDeclStmt(stmt *DeclStmt) {
	VisitDecl(v, stmt.D)
}

func (v *symbolVisitor) VisitIfStmt(stmt *IfStmt) {
	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		VisitStmt(v, stmt.Else)
	}
}

func (v *symbolVisitor) VisitWhileStmt(stmt *WhileStmt) {
	v.VisitBlockStmt(stmt.Body)
}
