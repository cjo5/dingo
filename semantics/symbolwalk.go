package semantics

import (
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

type symbolVisitor struct {
	ir.BaseVisitor
	c *checker
}

func symbolWalk(c *checker) {
	v := &symbolVisitor{c: c}
	ir.VisitModuleSet(v, c.set)
	c.resetWalkState()
}

func (v *symbolVisitor) Module(mod *ir.Module) {
	v.c.openScope(ir.TopScope)
	mod.Scope = v.c.scope
	v.c.mod = mod
	for _, decl := range mod.Decls {
		v.c.setTopDecl(decl)
		ir.VisitDecl(v, decl)

	}
	v.c.closeScope()
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
	scope := v.c.visibilityScope(decl.Visibility)

	sym := v.c.lookup(decl.Name.Literal)
	if sym != nil && sym.ID == ir.FuncSymbol {
		decl.Sym = sym
	} else {
		decl.Sym = v.c.insert(scope, ir.FuncSymbol, decl.Name.Literal, decl.Name.Pos, decl)
	}

	v.c.openScope(ir.LocalScope)
	decl.Scope = v.c.scope

	for _, param := range decl.Params {
		v.VisitValDecl(param)
	}

	if !decl.SignatureOnly() {
		if decl.Sym.Defined() {
			v.c.error(decl.Name.Pos, "redefinition of '%s', previously defined at %s", decl.Name.Literal, decl.Sym.Src.FirstPos())
		} else {
			decl.Sym.Flags |= ir.SymFlagDefined
		}

		decl.Body.Scope = decl.Scope
		ir.VisitStmtList(v, decl.Body.Stmts)
	}

	v.c.closeScope()
}

func (v *symbolVisitor) VisitStructDecl(decl *ir.StructDecl) {
	scope := v.c.visibilityScope(decl.Visibility)
	decl.Sym = v.c.insert(scope, ir.TypeSymbol, decl.Name.Literal, decl.Name.Pos, decl)
	decl.Scope = ir.NewScope(ir.FieldScope, nil)

	defer setScope(setScope(v.c, decl.Scope))
	for _, field := range decl.Fields {
		v.VisitValDecl(field)
	}
}

func (v *symbolVisitor) VisitBlockStmt(stmt *ir.BlockStmt) {
	v.c.openScope(ir.LocalScope)
	stmt.Scope = v.c.scope
	ir.VisitStmtList(v, stmt.Stmts)
	v.c.closeScope()
}

func (v *symbolVisitor) VisitDeclStmt(stmt *ir.DeclStmt) {
	ir.VisitDecl(v, stmt.D)
}

func (v *symbolVisitor) VisitIfStmt(stmt *ir.IfStmt) {
	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		ir.VisitStmt(v, stmt.Else)
	}
}

func (v *symbolVisitor) VisitForStmt(stmt *ir.ForStmt) {
	v.c.openScope(ir.LocalScope)
	stmt.Scope = v.c.scope

	if stmt.Init != nil {
		ir.VisitDecl(v, stmt.Init)
	}

	v.VisitBlockStmt(stmt.Body)
	v.c.closeScope()
}
