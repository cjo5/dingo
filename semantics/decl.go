package semantics

import (
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

func (v *typeChecker) Module(mod *ir.Module) {
	v.c.mod = mod

	v.c.scope = mod.Scope
	v.signature = true
	for _, decl := range mod.Decls {
		if decl.Symbol() == nil {
			// Nil symbol means a redeclaration
			continue
		}
		v.c.setCurrentTopDecl(decl)
		ir.VisitDecl(v, decl)
	}

	v.c.scope = mod.Scope
	v.signature = false
	for _, decl := range mod.Decls {
		if decl.Symbol() == nil {
			// Nil symbol means a redeclaration
			continue
		}
		v.c.setCurrentTopDecl(decl)
		ir.VisitDecl(v, decl)
	}
}

func (v *typeChecker) VisitValTopDecl(decl *ir.ValTopDecl) {
	if v.signature {
		return
	}

	v.warnUnusedDirectives(decl.Directives)
	v.visitValDeclSpec(decl.Sym, &decl.ValDeclSpec, true)

	if !ir.IsUntyped(decl.Sym.T) {
		init := decl.Initializer
		if !v.checkCompileTimeConstant(init) {
			v.c.error(init.FirstPos(), "'%s' is not a compile-time constant", PrintExpr(init))
		}
	}
}

func (v *typeChecker) VisitValDecl(decl *ir.ValDecl) {
	if decl.Sym != nil {
		v.visitValDeclSpec(decl.Sym, &decl.ValDeclSpec, decl.Init())
	}
}

func (v *typeChecker) warnUnusedDirectives(directives []ir.Directive) {
	for _, dir := range directives {
		v.c.warning(dir.Directive.Pos, "unused directive %s%s", dir.Directive.Literal, dir.Name.Literal)
	}
}

func (v *typeChecker) checkCABI(abi *ir.Ident) bool {
	if abi == nil {
		return false
	}
	if abi.Literal() != "c" {
		v.c.error(abi.Pos(), "unknown abi '%s'", abi.Literal())
		return false
	}
	return true
}

func (v *typeChecker) visitValDeclSpec(sym *ir.Symbol, decl *ir.ValDeclSpec, defaultInit bool) {
	if decl.Decl.Is(token.Val) {
		sym.Flags |= ir.SymFlagReadOnly
	}

	if sym.DepCycle() {
		sym.T = ir.TBuiltinUntyped
		return
	}

	if decl.Type != nil {
		v.exprMode = exprModeType
		decl.Type = ir.VisitExpr(v, decl.Type)
		v.exprMode = exprModeNone
		sym.T = decl.Type.Type()

		if typeSym := ir.ExprSymbol(decl.Type); typeSym != nil {
			if typeSym.ID != ir.TypeSymbol {
				v.c.error(decl.Type.FirstPos(), "'%s' is not a type", PrintExpr(decl.Type))
				sym.T = ir.TBuiltinUntyped
				return
			}
		}

		if sym.T.ID() == ir.TVoid {
			v.c.error(decl.Type.FirstPos(), "%s cannot be used as a type", sym.T)
			sym.T = ir.TBuiltinUntyped
		} else if !checkCompleteType(sym.T) {
			v.c.error(decl.Type.FirstPos(), "incomplete type %s", sym.T)
			sym.T = ir.TBuiltinUntyped
		}

		if ir.IsUntyped(sym.T) {
			return
		}
	}

	if decl.Initializer != nil {
		decl.Initializer = v.makeTypedExpr(decl.Initializer, sym.T)

		if decl.Type == nil {
			sym.T = decl.Initializer.Type()
			if ptr, ok := sym.T.(*ir.PointerType); ok {
				if ir.IsTypeID(ptr.Underlying, ir.TUntyped) {
					v.c.error(decl.Initializer.FirstPos(), "type specifier required; impossible to infer type from '%s'", PrintExpr(decl.Initializer))
				}
			}
		} else {
			if !checkTypes(v.c, sym.T, decl.Initializer.Type()) {
				v.c.error(decl.Initializer.FirstPos(), "type mismatch: '%s' with type %s is different from '%s' with type %s",
					decl.Name.Literal, sym.T, PrintExpr(decl.Initializer), decl.Initializer.Type())
			}
		}
	} else if decl.Type == nil {
		v.c.error(decl.Name.Pos, "missing type specifier or initializer")
	} else if defaultInit {
		decl.Initializer = createDefaultLit(sym.T)
	}
}

func (v *typeChecker) VisitFuncDecl(decl *ir.FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))

	if v.signature {
		var tparams []ir.Type

		for _, param := range decl.Params {
			v.VisitValDecl(param)
			if param.Sym.T == nil || ir.IsUntyped(param.Sym.T) {
				tparams = append(tparams, ir.TBuiltinUntyped)
			} else {
				tparams = append(tparams, param.Sym.T)
			}
		}

		v.exprMode = exprModeType
		decl.TReturn = ir.VisitExpr(v, decl.TReturn)
		v.exprMode = exprModeNone

		v.warnUnusedDirectives(decl.Directives)

		c := v.checkCABI(decl.ABI)
		tfun := ir.NewFuncType(tparams, decl.TReturn.Type(), c)

		if decl.Sym.T != nil && !checkTypes(v.c, decl.Sym.T, tfun) {
			v.c.error(decl.Name.Pos, "'%s' was previously declared at %s with a different type signature", decl.Name.Literal, decl.Sym.Pos)
		} else if !decl.Sym.Defined() && !c {
			v.c.error(decl.Name.Pos, "'%s' is not defined or declared as a C function", decl.Name.Literal)
		}

		if decl.Sym.T == nil {
			decl.Sym.T = tfun
		}

		return
	} else if decl.SignatureOnly() {
		return
	}

	if decl.Sym.DepCycle() {
		return
	}

	v.VisitBlockStmt(decl.Body)

	endsWithReturn := false
	for i, stmt := range decl.Body.Stmts {
		if _, ok := stmt.(*ir.ReturnStmt); ok {
			if (i + 1) == len(decl.Body.Stmts) {
				endsWithReturn = true
			}
		}
	}

	if !endsWithReturn {
		if decl.TReturn.Type().ID() != ir.TVoid {
			v.c.error(decl.Body.Rbrace.Pos, "missing return")
		} else {
			tok := token.Synthetic(token.Return, "return")
			returnStmt := &ir.ReturnStmt{Return: tok}
			decl.Body.Stmts = append(decl.Body.Stmts, returnStmt)

		}
	}
}

func (v *typeChecker) VisitStructDecl(decl *ir.StructDecl) {
	if v.signature {
		v.warnUnusedDirectives(decl.Directives)
		decl.Sym.T = ir.NewIncompleteStructType(decl)
		return
	}

	for _, field := range decl.Fields {
		v.VisitValDecl(field)
	}
	structt := decl.Sym.T.(*ir.StructType)
	structt.SetBody(decl)
}
