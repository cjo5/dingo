package semantics

import "github.com/jhnl/dingo/token"
import "github.com/jhnl/dingo/ir"

type typeVisitor struct {
	ir.BaseVisitor
	signature bool
	exprMode  int
	c         *checker
}

func typeWalk(c *checker) {
	v := &typeVisitor{c: c}
	c.resetWalkState()
	ir.VisitModuleSet(v, c.set)
}

func (v *typeVisitor) Module(mod *ir.Module) {
	v.c.mod = mod

	v.c.scope = mod.Scope
	v.signature = true
	for _, decl := range mod.Decls {
		if decl.Symbol() == nil {
			// Nil symbol means a redeclaration
			continue
		}
		v.c.setTopDecl(decl)
		ir.VisitDecl(v, decl)
	}

	v.c.scope = mod.Scope
	v.signature = false
	for _, decl := range mod.Decls {
		if decl.Symbol() == nil {
			// Nil symbol means a redeclaration
			continue
		}
		v.c.setTopDecl(decl)
		ir.VisitDecl(v, decl)
	}
}

func (v *typeVisitor) VisitValTopDecl(decl *ir.ValTopDecl) {
	if v.signature {
		return
	}
	v.visitValDeclSpec(decl.Sym, &decl.ValDeclSpec, true)

	v.checkCompileTimeContant(decl.Initializer)
}

func (v *typeVisitor) checkCompileTimeContant(expr ir.Expr) bool {
	switch init := expr.(type) {
	case *ir.BasicLit:
	case *ir.StructLit:
		for _, field := range init.Initializers {
			if !v.checkCompileTimeContant(field.Value) {
				return false
			}
		}
	case *ir.ArrayLit:
		for _, elem := range init.Initializers {
			if !v.checkCompileTimeContant(elem) {
				return false
			}
		}
	default:
		v.c.error(expr.FirstPos(), "'%s' is not a compile-time constant", PrintExpr(init))
		return false
	}
	return true
}

func (v *typeVisitor) VisitValDecl(decl *ir.ValDecl) {
	if decl.Sym != nil {
		v.visitValDeclSpec(decl.Sym, &decl.ValDeclSpec, decl.Init())
	}
}

func (v *typeVisitor) visitValDeclSpec(sym *ir.Symbol, decl *ir.ValDeclSpec, defaultInit bool) {
	if decl.Decl.Is(token.Val) {
		sym.Flags |= ir.SymFlagReadOnly
	}

	if sym.DepCycle() {
		return
	}

	if decl.Type != nil {
		v.exprMode = exprModeType
		ir.VisitExpr(v, decl.Type)
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
			v.c.error(decl.Type.FirstPos(), "%s cannot be used as a type specifier", sym.T)
			sym.T = ir.TBuiltinUntyped
			return
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
			if !v.c.checkTypes(sym.T, decl.Initializer.Type()) {
				v.c.error(decl.Initializer.FirstPos(), "type mismatch: '%s' has type %s and is not compatible with %s",
					decl.Name.Literal, sym.T, decl.Initializer.Type())
			}
		}
	} else if decl.Type == nil {
		v.c.error(decl.Name.Pos, "missing type specifier or initializer")
	} else if defaultInit {
		decl.Initializer = createDefaultLit(sym.T)
	}
}

func (v *typeVisitor) VisitFuncDecl(decl *ir.FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))

	if v.signature {
		for _, param := range decl.Params {
			v.VisitValDecl(param)
		}
		v.exprMode = exprModeType
		decl.TReturn = ir.VisitExpr(v, decl.TReturn)
		v.exprMode = exprModeNone

		typ := ir.NewFuncType(decl)

		if decl.Sym.T != nil && !v.c.checkTypes(decl.Sym.T, typ) {
			v.c.error(decl.Name.Pos, "'%s' was previously declared at %s with a different type signature", decl.Name.Literal, decl.Sym.Src.FirstPos())
		} else if decl.Visibility.ID == token.Private && !decl.Sym.Defined() {
			v.c.error(decl.Name.Pos, "'%s' is declared as private and there's no definition in this module", decl.Name.Literal)
		}

		if decl.Sym.T == nil {
			decl.Sym.T = typ
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

func (v *typeVisitor) VisitStructDecl(decl *ir.StructDecl) {
	if v.signature {
		decl.Sym.T = ir.NewIncompleteStructType(decl)
		return
	}

	for _, field := range decl.Fields {
		v.VisitValDecl(field)
	}
	structt := decl.Sym.T.(*ir.StructType)
	structt.SetBody(decl)
}

func (v *typeVisitor) VisitBlockStmt(stmt *ir.BlockStmt) {
	defer setScope(setScope(v.c, stmt.Scope))
	ir.VisitStmtList(v, stmt.Stmts)
}

func (v *typeVisitor) VisitDeclStmt(stmt *ir.DeclStmt) {
	ir.VisitDecl(v, stmt.D)
}

func (v *typeVisitor) VisitIfStmt(stmt *ir.IfStmt) {
	stmt.Cond = ir.VisitExpr(v, stmt.Cond)
	if !v.c.checkTypes(stmt.Cond.Type(), ir.TBuiltinBool) {
		v.c.error(stmt.Cond.FirstPos(), "condition has type %s (expected %s)", stmt.Cond.Type(), ir.TBool)
	}

	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		ir.VisitStmt(v, stmt.Else)
	}
}
func (v *typeVisitor) VisitForStmt(stmt *ir.ForStmt) {
	defer setScope(setScope(v.c, stmt.Scope))

	if stmt.Init != nil {
		v.VisitValDecl(stmt.Init)
	}

	if stmt.Cond != nil {
		stmt.Cond = ir.VisitExpr(v, stmt.Cond)
		if !v.c.checkTypes(stmt.Cond.Type(), ir.TBuiltinBool) {
			v.c.error(stmt.Cond.FirstPos(), "condition has type %s (expected %s)", stmt.Cond.Type(), ir.TBool)
		}
	}

	if stmt.Inc != nil {
		ir.VisitStmt(v, stmt.Inc)
	}

	v.VisitBlockStmt(stmt.Body)
}

func (v *typeVisitor) VisitReturnStmt(stmt *ir.ReturnStmt) {
	mismatch := false

	funDecl, _ := v.c.topDecl.(*ir.FuncDecl)
	retType := funDecl.TReturn.Type()
	if retType.ID() == ir.TUntyped {
		return
	}

	exprType := ir.TVoid

	if stmt.X == nil {
		if retType.ID() != ir.TVoid {
			mismatch = true
		}
	} else {
		stmt.X = v.makeTypedExpr(stmt.X, retType)

		if !v.c.checkTypes(stmt.X.Type(), retType) {
			exprType = stmt.X.Type().ID()
			mismatch = true
		}
	}

	if mismatch {
		v.c.error(stmt.Return.Pos, "type mismatch: return type %s does not match function '%s' return type %s",
			exprType, funDecl.Name.Literal, retType)
	}
}

func (v *typeVisitor) VisitAssignStmt(stmt *ir.AssignStmt) {
	stmt.Left = ir.VisitExpr(v, stmt.Left)
	if stmt.Left.Type().ID() == ir.TUntyped {
		return
	}

	left := stmt.Left
	if !left.Lvalue() {
		v.c.error(stmt.Left.FirstPos(), "cannot assign to '%s' (not an lvalue)", PrintExpr(left))
		return
	}

	stmt.Right = v.makeTypedExpr(stmt.Right, left.Type())

	if stmt.Left.ReadOnly() {
		v.c.error(stmt.Left.FirstPos(), "'%s' is read-only", PrintExpr(stmt.Left))
	}

	if !v.c.checkTypes(left.Type(), stmt.Right.Type()) {
		v.c.error(left.FirstPos(), "type mismatch: '%s' is of type %s and is not compatible with %s",
			PrintExpr(left), left.Type(), stmt.Right.Type())
	}

	if stmt.Assign.ID != token.Assign {
		if !ir.IsNumericType(left.Type()) {
			v.c.error(left.FirstPos(), "type mismatch: %s is not numeric (has type %s)",
				stmt.Assign, PrintExpr(left), left.Type())
		}
	}
}

func (v *typeVisitor) VisitExprStmt(stmt *ir.ExprStmt) {
	stmt.X = v.makeTypedExpr(stmt.X, nil)
}
