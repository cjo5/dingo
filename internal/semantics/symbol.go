package semantics

import (
	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

type symChecker struct {
	ir.BaseVisitor
	c   *context
	fqn string
}

func symCheck(c *context) {
	v := &symChecker{c: c}
	registerModules(c)
	ir.VisitModuleSet(v, c.set)
	c.resetWalkState()
}

func registerModules(c *context) {
	for _, mod := range c.set.Modules {
		c.openScope(ir.TopScope, mod.FQN)
		mod.Scope = c.scope
		c.closeScope()
	}

	for _, mod := range c.set.Modules {
		for _, file := range mod.Files {
			for _, dep := range file.ModDeps {
				fqn := ir.ExprNameToText(dep.ModName)
				if fqn == mod.FQN {
					c.error(dep.ModName.Pos(), "module '%s' cannot import itself", fqn)
				} else if moddep, ok := c.set.Modules[fqn]; ok {
					sym := c.insert(mod.Scope, ir.ModuleSymbol, isPublic(dep.Visibility), dep.Alias.Literal, dep.ModName.Pos())
					if sym != nil {
						sym.T = ir.NewModuleType(fqn, moddep.Scope)
					}
				} else {
					c.error(dep.ModName.Pos(), "module '%s' not found", fqn)
				}
			}
		}
	}
}

func (v *symChecker) Module(mod *ir.Module) {
	v.fqn = mod.FQN
	v.c.scope = mod.Scope
	for _, decl := range mod.Decls {
		v.c.pushTopDecl(decl)
		ir.VisitDecl(v, decl)
		v.c.popTopDecl()
	}
	v.c.scope = nil
}

func (v *symChecker) isTypeName(name *ir.Ident) bool {
	if sym := v.c.lookup(name.Literal); sym != nil {
		if sym.ID == ir.TypeSymbol {
			v.c.error(name.Pos(), "%s is a type and cannot be used as an identifier", name.Literal)
			return true
		}
	}
	return false
}

func isPublic(tok token.Token) bool {
	return tok.Is(token.Public)
}

func valOrConstID(tok token.Token) ir.SymbolID {
	if tok.Is(token.Const) {
		return ir.ConstSymbol
	}
	return ir.ValSymbol
}

func (v *symChecker) VisitValTopDecl(decl *ir.ValTopDecl) {
	decl.Deps = make(ir.DeclDependencyGraph)
	if !v.isTypeName(decl.Name) {
		decl.Sym = v.c.insert(v.c.scope, valOrConstID(decl.Decl), isPublic(decl.Visibility), decl.Name.Literal, decl.Name.Pos())
		v.c.mapTopDecl(decl.Sym, decl)
	}
}

func (v *symChecker) VisitValDecl(decl *ir.ValDecl) {
	if decl.Name.Tok == token.Underscore {
		decl.Sym = ir.NewSymbol(valOrConstID(decl.Decl), nil, false, decl.Name.Literal, decl.Name.Pos())
	} else if !v.isTypeName(decl.Name) {
		decl.Sym = v.c.insert(v.c.scope, valOrConstID(decl.Decl), false, decl.Name.Literal, decl.Name.Pos())
	}
}

func (v *symChecker) VisitFuncDecl(decl *ir.FuncDecl) {
	decl.Deps = make(ir.DeclDependencyGraph)

	if sym := v.c.lookup(decl.Name.Literal); sym != nil {
		if sym.ID == ir.FuncSymbol && (!sym.Defined() || decl.SignatureOnly()) {
			decl.Sym = sym
		}
	}

	if decl.Sym == nil {
		decl.Sym = v.c.insert(v.c.scope, ir.FuncSymbol, isPublic(decl.Visibility), decl.Name.Literal, decl.Name.Pos())
		v.c.mapTopDecl(decl.Sym, decl)
	}

	v.c.openScope(ir.LocalScope, v.fqn)
	defer v.c.closeScope()
	decl.Scope = v.c.scope

	for _, param := range decl.Params {
		v.VisitValDecl(param)
	}

	if decl.Body != nil {
		decl.Body.Scope = decl.Scope
		ir.VisitStmtList(v, decl.Body.Stmts)
	}

	if decl.Sym == nil {
		return
	}

	public := decl.Visibility.Is(token.Public)

	if decl.SignatureOnly() {
		if !public {
			v.c.error(decl.Name.Pos(), "'%s' is not declared as public", decl.Name.Literal)
			return
		}
	} else {
		decl.Sym.Flags |= ir.SymFlagDefined
		decl.Sym.DefPos = decl.Name.Pos()
	}

	if public != decl.Sym.Public {
		vis := "private"
		if decl.Sym.Public {
			vis = "public"
		}
		v.c.error(decl.Name.Pos(), "redeclaration of '%s' (previously declared as %s at %s)",
			decl.Name.Literal, vis, decl.Sym.DeclPos)
	}
}

func (v *symChecker) VisitStructDecl(decl *ir.StructDecl) {
	decl.Deps = make(ir.DeclDependencyGraph)

	decl.Sym = v.c.insert(v.c.scope, ir.TypeSymbol, isPublic(decl.Visibility), decl.Name.Literal, decl.Name.Pos())
	decl.Scope = ir.NewScope(ir.FieldScope, v.fqn, nil)

	if decl.Sym != nil {
		decl.Sym.T = ir.NewIncompleteStructType(decl)
	}

	v.c.mapTopDecl(decl.Sym, decl)

	defer setScope(setScope(v.c, decl.Scope))
	for _, field := range decl.Fields {
		v.VisitValDecl(field)
	}
}

func (v *symChecker) VisitBlockStmt(stmt *ir.BlockStmt) {
	v.c.openScope(ir.LocalScope, v.fqn)
	stmt.Scope = v.c.scope
	ir.VisitStmtList(v, stmt.Stmts)
	v.c.closeScope()
}

func (v *symChecker) VisitDeclStmt(stmt *ir.DeclStmt) {
	ir.VisitDecl(v, stmt.D)
}

func (v *symChecker) VisitIfStmt(stmt *ir.IfStmt) {
	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		ir.VisitStmt(v, stmt.Else)
	}
}

func (v *symChecker) VisitForStmt(stmt *ir.ForStmt) {
	v.c.openScope(ir.LocalScope, v.fqn)
	stmt.Body.Scope = v.c.scope

	if stmt.Init != nil {
		v.VisitValDecl(stmt.Init)
	}

	ir.VisitStmtList(v, stmt.Body.Stmts)
	v.c.closeScope()
}
