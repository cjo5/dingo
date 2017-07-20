package semantics

import "github.com/jhnl/interpreter/token"

type typeVisitor struct {
	BaseVisitor
	signature bool
	c         *checker
}

func typeWalk(c *checker) {
	v := &typeVisitor{c: c}
	c.resetWalkState()
	StartProgramWalk(v, c.prog)
}

func (v *typeVisitor) Module(mod *Module) {
	v.c.mod = mod

	v.signature = true
	for _, decl := range mod.Decls {
		if decl.Symbol() == nil {
			// Nil symbol means a redeclaration
			continue
		}

		if fun, ok := decl.(*FuncDecl); ok {
			v.c.setTopDecl(decl)
			v.VisitFuncDecl(fun)
		}
	}

	v.signature = false
	for _, decl := range mod.Decls {
		if decl.Symbol() == nil {
			// Nil symbol means a redeclaration
			continue
		}
		v.c.setTopDecl(decl)
		VisitDecl(v, decl)
	}
}

func (v *typeVisitor) VisitValTopDecl(decl *ValTopDecl) {
	v.visitValDeclSpec(decl.Sym, &decl.ValDeclSpec, true)
}

func (v *typeVisitor) VisitValDecl(decl *ValDecl) {
	if decl.Sym != nil {
		v.visitValDeclSpec(decl.Sym, &decl.ValDeclSpec, decl.Init())
	}
}

func (v *typeVisitor) visitValDeclSpec(sym *Symbol, decl *ValDeclSpec, defaultInit bool) {
	VisitExpr(v, decl.Type)
	sym.T = decl.Type.Type()

	if decl.Decl.Is(token.Val) {
		sym.Flags |= SymFlagConstant
	}

	if sym.DepCycle() || IsUntyped(sym.T) {
		return
	}

	if decl.Initializer != nil {
		decl.Initializer = VisitExpr(v, decl.Initializer)

		if !v.c.tryCastLiteral(decl.Initializer, sym.T) {
			return
		}

		if !sym.T.IsEqual(decl.Initializer.Type()) {
			v.c.error(decl.Initializer.FirstPos(), "type mismatch: '%s' has type %s and is not compatible with %s",
				decl.Name.Literal, sym.T, decl.Initializer.Type())
		}
	} else if defaultInit {
		decl.Initializer = createDefaultLiteral(sym.T)
	}
}

func (v *typeVisitor) VisitFuncDecl(decl *FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))

	if v.signature {
		for _, param := range decl.Params {
			v.VisitValDecl(param)
		}
		decl.TReturn = VisitExpr(v, decl.TReturn)

		decl.Sym.T = NewFuncType(decl)
		return
	}

	if decl.Sym.DepCycle() {
		return
	}

	v.VisitBlockStmt(decl.Body)

	endsWithReturn := false
	for i, stmt := range decl.Body.Stmts {
		if _, ok := stmt.(*ReturnStmt); ok {
			if (i + 1) == len(decl.Body.Stmts) {
				endsWithReturn = true
			}
		}
	}

	if !endsWithReturn {
		if decl.TReturn.Type().ID() != TVoid {
			v.c.error(decl.Body.Rbrace.Pos, "missing return")
		} else {
			tok := token.Synthetic(token.Return, "return")
			returnStmt := &ReturnStmt{Return: tok}
			decl.Body.Stmts = append(decl.Body.Stmts, returnStmt)

		}
	}
}

func (v *typeVisitor) VisitStructDecl(decl *StructDecl) {
	for _, field := range decl.Fields {
		v.VisitValDecl(field)
	}
	decl.Sym.T = NewStructType(decl)
}

func (v *typeVisitor) VisitBlockStmt(stmt *BlockStmt) {
	defer setScope(setScope(v.c, stmt.Scope))
	VisitStmtList(v, stmt.Stmts)
}

func (v *typeVisitor) VisitDeclStmt(stmt *DeclStmt) {
	VisitDecl(v, stmt.D)
}

func (v *typeVisitor) VisitPrintStmt(stmt *PrintStmt) {
	stmt.X = VisitExpr(v, stmt.X)
}

func (v *typeVisitor) VisitIfStmt(stmt *IfStmt) {
	stmt.Cond = VisitExpr(v, stmt.Cond)
	if stmt.Cond.Type().ID() != TBool {
		v.c.error(stmt.Cond.FirstPos(), "if condition has type %s (expected %s)", stmt.Cond.Type(), TBool)
	}

	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		VisitStmt(v, stmt.Else)
	}
}
func (v *typeVisitor) VisitWhileStmt(stmt *WhileStmt) {
	stmt.Cond = VisitExpr(v, stmt.Cond)
	if stmt.Cond.Type().ID() != TBool {
		v.c.error(stmt.Cond.FirstPos(), "while condition has type %s (expected %s)", stmt.Cond.Type(), TBool)
	}
	v.VisitBlockStmt(stmt.Body)
}

func (v *typeVisitor) VisitReturnStmt(stmt *ReturnStmt) {
	mismatch := false

	funDecl, _ := v.c.topDecl.(*FuncDecl)
	retType := funDecl.TReturn.Type()
	if retType.ID() == TUntyped {
		return
	}

	exprType := TVoid

	if stmt.X == nil {
		if retType.ID() != TVoid {
			mismatch = true
		}
	} else {
		stmt.X = VisitExpr(v, stmt.X)
		if !v.c.tryCastLiteral(stmt.X, retType) {
			exprType = stmt.X.Type().ID()
			mismatch = true
		} else if !stmt.X.Type().IsEqual(retType) {
			exprType = stmt.X.Type().ID()
			mismatch = true
		}
	}

	if mismatch {
		v.c.error(stmt.Return.Pos, "type mismatch: return type %s does not match function '%s' return type %s",
			exprType, funDecl.Name.Literal, retType)
	}
}

func (v *typeVisitor) VisitAssignStmt(stmt *AssignStmt) {
	stmt.Left = VisitExpr(v, stmt.Left)
	if stmt.Left.Type().ID() == TUntyped {
		return
	}

	var name *Ident

	switch id := stmt.Left.(type) {
	case *Ident:
		name = id
	case *DotIdent:
		name = id.Name
	default:
		v.c.error(stmt.Left.FirstPos(), "invalid assignment")
		return
	}

	sym := name.Sym
	if sym == nil {
		return
	} else if sym.ID != ValSymbol {
		v.c.error(name.Pos(), "invalid assignment: '%s' is not a variable", name.Literal())
		return
	}

	stmt.Right = VisitExpr(v, stmt.Right)

	if constID := v.c.checkConstant(stmt.Left); constID != nil {
		v.c.error(constID.Pos(), "'%s' was declared with %s and cannot be modified (constant)",
			constID.Literal(), token.Val)
	}

	if !v.c.tryCastLiteral(stmt.Right, name.Type()) {
		return
	}

	if !name.Type().IsEqual(stmt.Right.Type()) {
		v.c.error(name.Pos(), "type mismatch: '%s' is of type %s and is not compatible with %s",
			name.Literal(), name.Type(), stmt.Right.Type())
	}

	if stmt.Assign.ID != token.Assign {
		if !IsNumericType(name.Type()) {
			v.c.error(name.Pos(), "type mismatch: %s is not numeric (has type %s)",
				stmt.Assign, name.Literal(), name.Type())
		}
	}
}

func (v *typeVisitor) VisitExprStmt(stmt *ExprStmt) {
	stmt.X = VisitExpr(v, stmt.X)
}
