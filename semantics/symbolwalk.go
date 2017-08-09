package semantics

import "github.com/jhnl/interpreter/token"
import "github.com/jhnl/interpreter/ir"

type symbolVisitor struct {
	BaseVisitor
	c *checker
}

func symbolWalk(c *checker) {
	v := &symbolVisitor{c: c}
	VisitModuleSet(v, c.set)
	c.resetWalkState()
}

func (v *symbolVisitor) Module(mod *ir.Module) {
	v.c.openScope(ir.TopScope)
	mod.Public = v.c.scope
	v.c.openScope(ir.TopScope)
	mod.Internal = v.c.scope
	v.c.mod = mod
	for _, file := range mod.Files {
		v.c.openScope(ir.TopScope)
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

func (v *symbolVisitor) VisitImport(decl *ir.Import) {
	sym := v.c.insert(v.c.fileScope(), ir.ModuleSymbol, decl.Mod.Name.Literal, decl.Literal.Pos, decl)
	if sym != nil {
		sym.T = ir.NewModuleType(decl.Mod.ID, decl.Mod.Public)
	}
}

func (v *symbolVisitor) isTypeName(name token.Token) bool {
	if sym := v.c.lookup(name.Literal); sym != nil {
		if sym.ID == ir.TypeSymbol {
			v.c.error(name.Pos, "%s is a type and cannot be used as an identifier", name.Literal)
			return true
		}
	}
	return false
}

func (v *symbolVisitor) VisitValTopDecl(decl *ir.ValTopDecl) {
	if !v.isTypeName(decl.Name) {
		scope := v.c.visibilityScope(decl.Visibility)
		decl.Sym = v.c.insert(scope, ir.ValSymbol, decl.Name.Literal, decl.Name.Pos, decl)
	}
}

func (v *symbolVisitor) VisitValDecl(decl *ir.ValDecl) {
	if !v.isTypeName(decl.Name) {
		decl.Sym = v.c.insert(v.c.scope, ir.ValSymbol, decl.Name.Literal, decl.Name.Pos, decl)
	}
}

func (v *symbolVisitor) VisitFuncDecl(decl *ir.FuncDecl) {
	if !v.isTypeName(decl.Name) {
		scope := v.c.visibilityScope(decl.Visibility)
		decl.Sym = v.c.insert(scope, ir.FuncSymbol, decl.Name.Literal, decl.Name.Pos, decl)
	}
	v.c.openScope(ir.LocalScope)
	decl.Scope = v.c.scope

	for _, param := range decl.Params {
		v.VisitValDecl(param)
	}

	decl.Body.Scope = decl.Scope
	VisitStmtList(v, decl.Body.Stmts)
	v.c.closeScope()
}

func (v *symbolVisitor) VisitStructDecl(decl *ir.StructDecl) {
	scope := v.c.visibilityScope(decl.Visibility)
	decl.Sym = v.c.insert(scope, ir.TypeSymbol, decl.Name.Literal, decl.Name.Pos, decl)
	v.c.openScope(ir.FieldScope)
	decl.Scope = v.c.scope

	for _, field := range decl.Fields {
		v.VisitValDecl(field)
	}

	v.c.closeScope()
}

func (v *symbolVisitor) VisitBlockStmt(stmt *ir.BlockStmt) {
	v.c.openScope(ir.LocalScope)
	stmt.Scope = v.c.scope
	VisitStmtList(v, stmt.Stmts)
	v.c.closeScope()
}

func (v *symbolVisitor) VisitDeclStmt(stmt *ir.DeclStmt) {
	VisitDecl(v, stmt.D)
}

func (v *symbolVisitor) VisitIfStmt(stmt *ir.IfStmt) {
	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		VisitStmt(v, stmt.Else)
	}
}

func (v *symbolVisitor) VisitWhileStmt(stmt *ir.WhileStmt) {
	v.VisitBlockStmt(stmt.Body)
}
