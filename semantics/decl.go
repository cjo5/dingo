package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

func (v *typeChecker) visitModuleSet(set *ir.ModuleSet, signature bool) {
	set.ResetDeclColors()
	v.signature = signature
	for _, mod := range set.Modules {
		v.c.mod = mod
		v.c.scope = mod.Scope
		for _, decl := range mod.Decls {
			// Nil symbol means a redeclaration
			if decl.Symbol() == nil {
				continue
			}
			v.visitTopDecl(decl)
		}
	}
}

func (v *typeChecker) visitDependencies(decl ir.TopDecl) {
	graph := *decl.DependencyGraph()
	for dep := range graph {
		v.visitTopDecl(dep)
	}
}

func (v *typeChecker) removeFalseDependencies(decl ir.TopDecl) {
	graph := *decl.DependencyGraph()

	switch decl.(type) {
	case *ir.FuncDecl:
		for dep := range graph {
			sym := dep.Symbol()
			if sym.ID == ir.FuncSymbol {
				delete(graph, dep)
			}
		}
	case *ir.ValTopDecl:
		for dep := range graph {
			sym := dep.Symbol()
			if sym.ID == ir.ValSymbol || sym.ID == ir.FuncSymbol {
				delete(graph, dep)
			}
		}
	case *ir.StructDecl:
		for dep, edges := range graph {
			for i := 0; i < len(edges); i++ {
				edge := edges[i]
				if edge.IsType && edge.Sym != nil && edge.Sym.T != nil {
					t := edge.Sym.T
					if t.ID() == ir.TPointer || t.ID() == ir.TSlice || t.ID() == ir.TFunc {
						edges = append(edges[:i], edges[i+1:]...)
						i--
					}
				}
			}
			if len(edges) > 0 {
				graph[dep] = edges
			} else {
				delete(graph, dep)
			}
		}
	default:
		panic(fmt.Sprintf("Unhandled top decl %T", decl))
	}
}

func (v *typeChecker) visitTopDecl(decl ir.TopDecl) {
	if decl.Color() != ir.WhiteColor {
		return
	}

	v.c.pushTopDecl(decl)
	decl.SetColor(ir.GrayColor)
	ir.VisitDecl(v, decl)
	decl.SetColor(ir.BlackColor)
	v.c.popTopDecl()
}

func (v *typeChecker) VisitValTopDecl(decl *ir.ValTopDecl) {
	if v.signature {
		return
	}

	v.removeFalseDependencies(decl)
	v.visitDependencies(decl)

	v.warnUnusedDirectives(decl.Directives)
	v.visitValDeclSpec(decl.Sym, &decl.ValDeclSpec, true)

	if !ir.IsUntyped(decl.Sym.T) {
		init := decl.Initializer
		if !v.checkCompileTimeConstant(init) {
			v.c.error(init.Pos(), "initializer is not a compile-time constant")
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
		v.c.warning(dir.Directive.Pos, "unused directive %s%s", dir.Directive.String(), dir.Name.Literal)
	}
}

func (v *typeChecker) checkCABI(abi *ir.Ident) bool {
	if abi == nil {
		return false
	}
	if abi.Literal != ir.CABI {
		v.c.error(abi.Pos(), "unknown abi '%s'", abi.Literal)
		return false
	}
	return true
}

func (v *typeChecker) visitValDeclSpec(sym *ir.Symbol, decl *ir.ValDeclSpec, defaultInit bool) {
	if decl.Decl.Is(token.Val) {
		sym.Flags |= ir.SymFlagReadOnly
	}

	var t ir.Type

	if decl.Type != nil {
		v.exprMode = exprModeType
		decl.Type = ir.VisitExpr(v, decl.Type)
		v.exprMode = exprModeNone
		t = decl.Type.Type()

		if t.ID() == ir.TVoid {
			v.c.error(decl.Type.Pos(), "%s cannot be used as a type", t)
			t = ir.TBuiltinUntyped
		} else if !checkCompleteType(t) {
			v.c.error(decl.Type.Pos(), "incomplete type %s", t)
			t = ir.TBuiltinUntyped
		}

		if ir.IsUntyped(t) {
			sym.T = t
			return
		}
	}

	if decl.Initializer != nil {
		decl.Initializer = v.makeTypedExpr(decl.Initializer, t)
		tinit := decl.Initializer.Type()

		if decl.Type == nil {
			if ptr, ok := tinit.(*ir.PointerType); ok {
				if ir.IsTypeID(ptr.Underlying, ir.TUntyped) {
					v.c.error(decl.Initializer.Pos(), "impossible to infer type from initializer")
					t = ir.TBuiltinUntyped
				}
			} else if tinit.ID() == ir.TVoid {
				v.c.error(decl.Initializer.Pos(), "initializer has invalid type %s", tinit)
				t = ir.TBuiltinUntyped
			}

			if t == nil {
				t = tinit
			}
		} else {
			if ir.IsUntyped(t) || ir.IsUntyped(tinit) {
				t = ir.TBuiltinUntyped
			} else if !checkTypes(v.c, t, tinit) {
				v.c.error(decl.Initializer.Pos(), "type mismatch %s and %s", t, tinit)
				t = ir.TBuiltinUntyped
			}
		}
	} else if decl.Type == nil {
		v.c.error(decl.Name.Pos(), "missing type or initializer")
		t = ir.TBuiltinUntyped
	} else if defaultInit {
		decl.Initializer = createDefaultLit(t)
	}

	// Wait to set type until the final step in order to be able to detect cycles
	sym.T = t
}

func (v *typeChecker) VisitFuncDecl(decl *ir.FuncDecl) {
	if v.signature {
		defer setScope(setScope(v.c, decl.Scope))

		c := v.checkCABI(decl.ABI)
		untyped := false

		var tparams []ir.Type

		for _, param := range decl.Params {
			v.VisitValDecl(param)
			if param.Sym == nil || ir.IsUntyped(param.Sym.T) {
				tparams = append(tparams, ir.TBuiltinUntyped)
				untyped = true
			} else {
				tparams = append(tparams, param.Sym.T)
			}
		}

		v.exprMode = exprModeType
		decl.Return.Type = ir.VisitExpr(v, decl.Return.Type)
		v.exprMode = exprModeNone

		v.warnUnusedDirectives(decl.Directives)

		tret := decl.Return.Type.Type()
		if ir.IsUntyped(tret) {
			untyped = true
		}

		tfun := ir.TBuiltinUntyped

		if !untyped {
			tfun = ir.NewFuncType(tparams, tret, c)
			if decl.Sym.T != nil && !checkTypes(v.c, decl.Sym.T, tfun) {
				v.c.error(decl.Name.Pos(), "redeclaration of '%s' (previously declared with a different signature at %s)",
					decl.Name.Literal, decl.Sym.DeclPos)
			}
		}

		if decl.Sym.T == nil {
			decl.Sym.T = tfun
		}

		return
	} else if decl.SignatureOnly() {
		return
	}

	v.visitDependencies(decl)
	v.removeFalseDependencies(decl)

	defer setScope(setScope(v.c, decl.Scope))
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
		if decl.Return.Type.Type().ID() != ir.TVoid {
			v.c.error(decl.Body.Rbrace.Pos, "missing return")
		} else {
			tok := token.Synthetic(token.Return)
			returnStmt := &ir.ReturnStmt{Return: tok}
			decl.Body.Stmts = append(decl.Body.Stmts, returnStmt)
		}
	}
}

func (v *typeChecker) VisitStructDecl(decl *ir.StructDecl) {
	if !v.signature {
		v.warnUnusedDirectives(decl.Directives)
		//decl.Sym.T = ir.NewIncompleteStructType(decl)
		return
	}

	v.visitDependencies(decl)

	untyped := false

	for _, field := range decl.Fields {
		v.VisitValDecl(field)
		if ir.IsUntyped(field.Sym.T) {
			untyped = true
		}
	}

	v.removeFalseDependencies(decl)

	if untyped {
		decl.Sym.T = ir.TBuiltinUntyped
	} else {
		if checkCycle(decl) {
			decl.Sym.T = ir.TBuiltinUntyped
		} else {
			structt := decl.Sym.T.(*ir.StructType)
			structt.SetBody(decl)
		}
	}
}
