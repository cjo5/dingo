package semantics

import (
	"fmt"

	"github.com/jhnl/interpreter/token"
)

type symbolVisitor struct {
	BaseVisitor
	c *typeChecker
}

func buildSymbolTableTree(c *typeChecker) {
	v := &symbolVisitor{c: c}
	v.VisitProgram(c.prog)
}

func (v *symbolVisitor) VisitModule(mod *Module) {
	v.c.mod = mod
	v.c.openScope()
	mod.External = v.c.scope
	v.c.openScope()
	mod.Internal = v.c.scope
	VisitFileList(v, mod.Files)
	v.c.closeScope()
	v.c.closeScope()
	v.c.mod = nil
}

func (v *symbolVisitor) VisitFile(file *File) {
	v.c.file = file
	v.c.openScope()
	file.Scope = v.c.scope
	VisitImportList(v, file.Imports)
	VisitDeclList(v, file.Decls)
	v.c.closeScope()
	v.c.file = nil
}

func (v *symbolVisitor) VisitImport(decl *Import) {
	v.c.insert(v.c.file.Scope, ModuleSymbol, decl.Mod.Name, decl.Mod)
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
	decl.Name.Sym = v.c.insert(scope, VarSymbol, decl.Name.Name, decl)
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

	decl.Name.Sym = v.c.insert(scope, FuncSymbol, decl.Name.Name, decl)
	v.c.openScope()
	decl.Scope = v.c.scope

	for _, param := range decl.Params {
		v.c.insert(v.c.scope, VarSymbol, param.Name.Name, param.Name)
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

func (v *symbolVisitor) VisitIfStmt(stmt *IfStmt) {
	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		v.VisitStmt(stmt.Else)
	}
}

func (v *symbolVisitor) VisitWhileStmt(stmt *WhileStmt) {
	v.VisitBlockStmt(stmt.Body)
}
