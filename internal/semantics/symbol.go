package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

func (c *checker) buildSymbolTable() {
	c.resetWalkState()
	c.prepareModuleScopes()
	for _, mod := range c.set.Modules {
		c.scope = mod.Scope
		c.fqn = mod.FQN
		for _, decl := range mod.Decls {
			c.pushTopDecl(decl)
			c.symswitchDecl(decl)
			c.popTopDecl()
		}
	}
}

func (c *checker) prepareModuleScopes() {
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

func (c *checker) isTypeName(name *ir.Ident) bool {
	if sym := c.lookup(name.Literal); sym != nil {
		if sym.IsType() {
			c.error(name.Pos(), "%s is a type and cannot be used as an identifier", name.Literal)
			return true
		}
	}
	return false
}

func (c *checker) checkConsistentDecl(public bool, sym *ir.Symbol, name *ir.Ident) {
	if public != sym.Public {
		vis := "private"
		if sym.Public {
			vis = "public"
		}
		c.error(name.Pos(), "redeclaration of '%s' (previously declared as %s at %s)", name.Literal, vis, sym.DeclPos)
	}
}

func (c *checker) checkABI(abi *ir.Ident, decl ir.TopDecl) {
	if abi != nil {
		if ir.IsValidABI(abi.Literal) {
			visibility := decl.Visibility()
			if visibility == token.Invalid && abi.Literal == ir.CABI {
				decl.SetVisibility(token.Public)
			}
		} else {
			c.error(abi.Pos(), "unknown abi '%s'", abi.Literal)
		}
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

func (c *checker) symswitchDecl(decl ir.Decl) {
	switch decl := decl.(type) {
	case *ir.TypeTopDecl:
		decl.Deps = make(ir.DeclDependencyGraph)
		setDefaultVisibility(decl)
		decl.Sym = c.addTopDeclSym(decl, decl.Name, ir.TypeSymbol, nil)
	case *ir.TypeDecl:
		decl.Sym = c.insert(c.scope, ir.TypeSymbol, false, ir.DGABI, decl.Name.Literal, decl.Name.Pos())
	case *ir.ValTopDecl:
		decl.Deps = make(ir.DeclDependencyGraph)
		c.checkABI(decl.ABI, decl)
		setDefaultVisibility(decl)
		if !c.isTypeName(decl.Name) {
			decl.Sym = c.addTopDeclSym(decl, decl.Name, valOrConstID(decl.Decl), decl.ABI)
		}
	case *ir.ValDecl:
		if decl.Name.Tok == token.Underscore {
			decl.Sym = ir.NewSymbol(valOrConstID(decl.Decl), nil, false, ir.DGABI, decl.Name.Literal, decl.Name.Pos())
		} else if !c.isTypeName(decl.Name) {
			decl.Sym = c.insert(c.scope, valOrConstID(decl.Decl), false, ir.DGABI, decl.Name.Literal, decl.Name.Pos())
		}
	case *ir.FuncDecl:
		c.addFuncSymbol(decl)
	case *ir.StructDecl:
		c.addStructSymbol(decl)
	default:
		panic(fmt.Sprintf("Unhandled decl %T", decl))
	}
}

func (c *checker) addFuncSymbol(decl *ir.FuncDecl) {
	decl.Deps = make(ir.DeclDependencyGraph)
	c.checkABI(decl.ABI, decl)
	setDefaultVisibility(decl)

	if sym := c.lookup(decl.Name.Literal); sym != nil {
		if sym.ID == ir.FuncSymbol && (!sym.IsDefined() || decl.SignatureOnly()) {
			decl.Sym = sym
		}
	}

	if decl.Sym == nil {
		decl.Sym = c.addTopDeclSym(decl, decl.Name, ir.FuncSymbol, decl.ABI)
	}

	c.openScope(ir.LocalScope, c.fqn)
	defer c.closeScope()
	decl.Scope = c.scope

	for _, param := range decl.Params {
		c.symswitchDecl(param)
	}

	c.symswitchDecl(decl.Return)

	if decl.Body != nil {
		decl.Body.Scope = decl.Scope
		stmtList(decl.Body.Stmts, c.symswitchStmt)
	}

	if decl.Sym == nil {
		return
	}

	public := decl.Visibility().Is(token.Public)

	if decl.SignatureOnly() {
		if !public {
			c.error(decl.Name.Pos(), "'%s' is not declared as public", decl.Name.Literal)
			return
		}
	} else {
		decl.Sym.Flags |= ir.SymFlagDefined
		decl.Sym.DefPos = decl.Name.Pos()
	}

	c.checkConsistentDecl(public, decl.Sym, decl.Name)
}

func (c *checker) addStructSymbol(decl *ir.StructDecl) {
	if !decl.Opaque && len(decl.Fields) == 0 {
		c.error(decl.Pos(), "struct must have at least 1 field")
		return
	}

	decl.Deps = make(ir.DeclDependencyGraph)
	decl.Scope = ir.NewScope(ir.FieldScope, c.fqn, nil)
	setDefaultVisibility(decl)

	if sym := c.lookup(decl.Name.Literal); sym != nil {
		if sym.ID == ir.StructSymbol && (!sym.IsDefined() || decl.Opaque) {
			decl.Sym = sym
		}
	}

	public := decl.Visibility().Is(token.Public)

	if decl.Sym == nil {
		decl.Sym = c.addTopDeclSym(decl, decl.Name, ir.StructSymbol, nil)
		if decl.Sym != nil && !decl.Opaque {
			decl.Sym.Flags |= ir.SymFlagDefined
		}
	} else if !decl.Sym.IsDefined() && !decl.Opaque {
		decl.Sym.Flags |= ir.SymFlagDefined
		decl.Sym.DefPos = decl.Name.Pos()
		c.decls[decl.Sym] = decl
	}

	defer setScope(setScope(c, decl.Scope))
	for _, field := range decl.Fields {
		c.symswitchDecl(field)
	}

	if decl.Sym != nil {
		c.checkConsistentDecl(public, decl.Sym, decl.Name)
	}
}

func (c *checker) symswitchStmt(stmt ir.Stmt) {
	switch stmt := stmt.(type) {
	case *ir.BlockStmt:
		c.openScope(ir.LocalScope, c.fqn)
		stmt.Scope = c.scope
		stmtList(stmt.Stmts, c.symswitchStmt)
		c.closeScope()
	case *ir.DeclStmt:
		c.symswitchDecl(stmt.D)
	case *ir.IfStmt:
		c.symswitchStmt(stmt.Body)
		if stmt.Else != nil {
			c.symswitchStmt(stmt.Else)
		}
	case *ir.ForStmt:
		c.openScope(ir.LocalScope, c.fqn)
		stmt.Body.Scope = c.scope

		if stmt.Init != nil {
			c.symswitchDecl(stmt.Init)
		}

		stmtList(stmt.Body.Stmts, c.symswitchStmt)
		c.closeScope()
	}
}
