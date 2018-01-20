package semantics

import (
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

type symChecker struct {
	ir.BaseVisitor
	c *context
}

func symCheck(c *context) {
	v := &symChecker{c: c}
	ir.VisitModuleSet(v, c.set)
	c.resetWalkState()
}

func (v *symChecker) Module(mod *ir.Module) {
	v.c.openScope(ir.TopScope)
	mod.Scope = v.c.scope
	v.c.mod = mod
	for _, decl := range mod.Decls {
		v.c.setCurrentTopDecl(decl)
		ir.VisitDecl(v, decl)
	}
	v.c.closeScope()
}

func (v *symChecker) isTypeName(name token.Token) bool {
	if sym := v.c.lookup(name.Literal); sym != nil {
		if sym.ID == ir.TypeSymbol {
			v.c.error(name.Pos, "%s is a type and cannot be used as an identifier", name.Literal)
			return true
		}
	}
	return false
}

func isPublic(tok token.Token) bool {
	return tok.Is(token.Public)
}

func (v *symChecker) VisitValTopDecl(decl *ir.ValTopDecl) {
	if !v.isTypeName(decl.Name) {
		scope := v.c.visibilityScope(decl.Visibility)
		decl.Sym = v.c.insert(scope, ir.ValSymbol, isPublic(decl.Visibility), decl.Name.Literal, decl.Name.Pos)
		v.c.mapTopDecl(decl.Sym, decl)
	}
}

func (v *symChecker) VisitValDecl(decl *ir.ValDecl) {
	if decl.Name.ID == token.Underscore {
		decl.Sym = ir.NewSymbol(ir.ValSymbol, v.c.scope.ID, false, decl.Name.Literal, v.c.newSymPos(decl.Name.Pos))
	} else if !v.isTypeName(decl.Name) {
		decl.Sym = v.c.insert(v.c.scope, ir.ValSymbol, false, decl.Name.Literal, decl.Name.Pos)
	}
}

func (v *symChecker) VisitFuncDecl(decl *ir.FuncDecl) {
	scope := v.c.visibilityScope(decl.Visibility)

	if sym := v.c.lookup(decl.Name.Literal); sym != nil {
		if sym.ID == ir.FuncSymbol && (!sym.Defined() || decl.SignatureOnly()) {
			decl.Sym = sym
		}
	}

	if decl.Sym == nil {
		decl.Sym = v.c.insert(scope, ir.FuncSymbol, isPublic(decl.Visibility), decl.Name.Literal, decl.Name.Pos)
		v.c.mapTopDecl(decl.Sym, decl)
	}

	v.c.openScope(ir.LocalScope)
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
			v.c.error(decl.Name.Pos, "'%s' is not declared as public", decl.Name.Literal)
			return
		}
	} else {
		decl.Sym.Flags |= ir.SymFlagDefined
		decl.Sym.DefPos = v.c.newSymPos(decl.Name.Pos)
	}

	if public != decl.Sym.Public {
		vis := "private"
		if decl.Sym.Public {
			vis = "public"
		}
		v.c.error(decl.Name.Pos, "redeclaration of '%s' (previously declared as %s at %s)",
			decl.Name.Literal, vis, v.c.fmtSymPos(decl.Sym.DeclPos))
	}
}

func (v *symChecker) VisitStructDecl(decl *ir.StructDecl) {
	scope := v.c.visibilityScope(decl.Visibility)
	decl.Sym = v.c.insert(scope, ir.TypeSymbol, isPublic(decl.Visibility), decl.Name.Literal, decl.Name.Pos)
	decl.Scope = ir.NewScope(ir.FieldScope, nil)

	v.c.mapTopDecl(decl.Sym, decl)

	defer setScope(setScope(v.c, decl.Scope))
	for _, field := range decl.Fields {
		v.VisitValDecl(field)
	}
}

func (v *symChecker) VisitBlockStmt(stmt *ir.BlockStmt) {
	v.c.openScope(ir.LocalScope)
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
	v.c.openScope(ir.LocalScope)
	stmt.Scope = v.c.scope

	if stmt.Init != nil {
		ir.VisitDecl(v, stmt.Init)
	}

	v.VisitBlockStmt(stmt.Body)
	v.c.closeScope()
}
