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
					dep.Visibility = defaultVisibility(dep.Visibility)
					public := dep.Visibility.Is(token.Public)
					sym := c.insert(mod.Scope, ir.ModuleSymbol, public, ir.DGABI, dep.Alias.Literal, dep.ModName.Pos())
					if sym != nil {
						sym.T = ir.NewModuleType(sym, moddep.Scope)
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
		if sym.IsType() {
			v.c.error(name.Pos(), "%s is a type and cannot be used as an identifier", name.Literal)
			return true
		}
	}
	return false
}

func (v *symChecker) checkConsistentDecl(public bool, sym *ir.Symbol, name *ir.Ident) {
	if public != sym.Public {
		vis := "private"
		if sym.Public {
			vis = "public"
		}
		v.c.error(name.Pos(), "redeclaration of '%s' (previously declared as %s at %s)", name.Literal, vis, sym.DeclPos)
	}
}

func valOrConstID(tok token.Token) ir.SymbolID {
	if tok.Is(token.Const) {
		return ir.ConstSymbol
	}
	return ir.ValSymbol
}

func defaultVisibility(visibility token.Token) token.Token {
	if visibility == token.Invalid {
		visibility = token.Private
	}
	return visibility
}

func setDefaultVisibility(decl ir.TopDecl) {
	visibility := defaultVisibility(decl.Visibility())
	decl.SetVisibility(visibility)
}

func (v *symChecker) checkABI(abi *ir.Ident, decl ir.TopDecl) {
	if abi != nil {
		if ir.IsValidABI(abi.Literal) {
			visibility := decl.Visibility()
			if visibility == token.Invalid && abi.Literal == ir.CABI {
				decl.SetVisibility(token.Public)
			}
		} else {
			v.c.error(abi.Pos(), "unknown abi '%s'", abi.Literal)
		}
	}
}

func (v *symChecker) VisitTypeTopDecl(decl *ir.TypeTopDecl) {
	decl.Deps = make(ir.DeclDependencyGraph)
	setDefaultVisibility(decl)
	if !v.isTypeName(decl.Name) {
		decl.Sym = v.c.addTopDeclSymbol(decl, decl.Name, ir.TypeSymbol, nil)
	}
}

func (v *symChecker) VisitTypeDecl(decl *ir.TypeDecl) {
	decl.Sym = v.c.insert(v.c.scope, ir.TypeSymbol, false, ir.DGABI, decl.Name.Literal, decl.Name.Pos())
}

func (v *symChecker) VisitValTopDecl(decl *ir.ValTopDecl) {
	decl.Deps = make(ir.DeclDependencyGraph)
	v.checkABI(decl.ABI, decl)
	setDefaultVisibility(decl)
	if !v.isTypeName(decl.Name) {
		decl.Sym = v.c.addTopDeclSymbol(decl, decl.Name, valOrConstID(decl.Decl), decl.ABI)
	}
}

func (v *symChecker) VisitValDecl(decl *ir.ValDecl) {
	if decl.Name.Tok == token.Underscore {
		decl.Sym = ir.NewSymbol(valOrConstID(decl.Decl), nil, false, ir.DGABI, decl.Name.Literal, decl.Name.Pos())
	} else if !v.isTypeName(decl.Name) {
		decl.Sym = v.c.insert(v.c.scope, valOrConstID(decl.Decl), false, ir.DGABI, decl.Name.Literal, decl.Name.Pos())
	}
}

func (v *symChecker) VisitFuncDecl(decl *ir.FuncDecl) {
	decl.Deps = make(ir.DeclDependencyGraph)
	v.checkABI(decl.ABI, decl)
	setDefaultVisibility(decl)

	if sym := v.c.lookup(decl.Name.Literal); sym != nil {
		if sym.ID == ir.FuncSymbol && (!sym.IsDefined() || decl.SignatureOnly()) {
			decl.Sym = sym
		}
	}

	if decl.Sym == nil {
		decl.Sym = v.c.addTopDeclSymbol(decl, decl.Name, ir.FuncSymbol, decl.ABI)
	}

	v.c.openScope(ir.LocalScope, v.fqn)
	defer v.c.closeScope()
	decl.Scope = v.c.scope

	for _, param := range decl.Params {
		v.VisitValDecl(param)
	}

	v.VisitValDecl(decl.Return)

	if decl.Body != nil {
		decl.Body.Scope = decl.Scope
		ir.VisitStmtList(v, decl.Body.Stmts)
	}

	if decl.Sym == nil {
		return
	}

	public := decl.Visibility().Is(token.Public)

	if decl.SignatureOnly() {
		if !public {
			v.c.error(decl.Name.Pos(), "'%s' is not declared as public", decl.Name.Literal)
			return
		}
	} else {
		decl.Sym.Flags |= ir.SymFlagDefined
		decl.Sym.DefPos = decl.Name.Pos()
	}

	v.checkConsistentDecl(public, decl.Sym, decl.Name)
}

func (v *symChecker) VisitStructDecl(decl *ir.StructDecl) {
	if !decl.Opaque && len(decl.Fields) == 0 {
		v.c.error(decl.Pos(), "struct must have at least 1 field")
		return
	}

	decl.Deps = make(ir.DeclDependencyGraph)
	decl.Scope = ir.NewScope(ir.FieldScope, v.fqn, nil)
	setDefaultVisibility(decl)

	if sym := v.c.lookup(decl.Name.Literal); sym != nil {
		if sym.ID == ir.StructSymbol && (!sym.IsDefined() || decl.Opaque) {
			decl.Sym = sym
		}
	}

	public := decl.Visibility().Is(token.Public)

	if decl.Sym == nil {
		decl.Sym = v.c.addTopDeclSymbol(decl, decl.Name, ir.StructSymbol, nil)
		if decl.Sym != nil && !decl.Opaque {
			decl.Sym.Flags |= ir.SymFlagDefined
		}
	} else if !decl.Sym.IsDefined() && !decl.Opaque {
		decl.Sym.Flags |= ir.SymFlagDefined
		decl.Sym.DefPos = decl.Name.Pos()
		v.c.decls[decl.Sym] = decl
	}

	defer setScope(setScope(v.c, decl.Scope))
	for _, field := range decl.Fields {
		v.VisitValDecl(field)
	}

	if decl.Sym != nil {
		v.checkConsistentDecl(public, decl.Sym, decl.Name)
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
