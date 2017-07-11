package semantics

import (
	"fmt"

	"github.com/jhnl/interpreter/token"
)

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
	v.c.openScope()
	mod.External = v.c.scope
	v.c.openScope()
	mod.Internal = v.c.scope
	v.c.mod = mod
	for _, file := range mod.Files {
		v.c.openScope()
		file.Info.Scope = v.c.scope
		v.c.file = file.Info

		VisitImportList(v, file.Info.Imports)
		VisitDeclList(v, file.Decls)

		v.c.closeScope() // File
	}
	v.c.closeScope() // Internal
	v.c.closeScope() // External
}

func (v *symbolVisitor) VisitImport(decl *Import) {
	sym := v.c.insert(v.c.file.Scope, ModuleSymbol, decl.Mod.Name.Literal, decl.Literal.Pos, decl.Mod)
	if sym != nil {
		sym.T = TBuiltinModule
	}
}

func (v *symbolVisitor) VisitVarDecl(decl *VarDecl) {
	var scope *Scope
	if decl.Visibility.Is(token.External) {
		scope = v.c.mod.External
	} else if decl.Visibility.Is(token.Internal) {
		scope = v.c.mod.Internal
	} else if decl.Visibility.Is(token.Restricted) {
		scope = v.c.file.Scope
	} else {
		scope = v.c.scope
	}
	decl.Sym = v.c.insert(scope, VarSymbol, decl.Name.Literal(), decl.Name.Name.Pos, decl)
}

func (v *symbolVisitor) VisitFuncDecl(decl *FuncDecl) {
	v.c.fun = decl

	var scope *Scope
	if decl.Visibility.Is(token.External) {
		scope = v.c.mod.External
	} else if decl.Visibility.Is(token.Internal) {
		scope = v.c.mod.Internal
	} else if decl.Visibility.Is(token.Restricted) {
		scope = v.c.file.Scope
	} else {
		panic(fmt.Sprintf("Unhandled visibility %s", decl.Visibility))
	}

	decl.Sym = v.c.insert(scope, FuncSymbol, decl.Name.Literal(), decl.Name.Pos(), decl)
	v.c.openScope()
	decl.Scope = v.c.scope

	for _, param := range decl.Params {
		param.Sym = v.c.insert(v.c.scope, VarSymbol, param.Name.Literal(), param.Name.Pos(), param)
	}

	decl.Body.Scope = decl.Scope
	VisitStmtList(v, decl.Body.Stmts)
	v.c.closeScope()

	v.c.fun = nil
}

func (v *symbolVisitor) VisitBlockStmt(stmt *BlockStmt) {
	v.c.openScope()
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
