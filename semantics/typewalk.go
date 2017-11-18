package semantics

import "github.com/jhnl/dingo/token"
import "github.com/jhnl/dingo/ir"

const (
	identModeNone = 0
	identModeType = 1
	identModeFunc = 2
)

type typeVisitor struct {
	ir.BaseVisitor
	signature bool
	identMode int
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
}

func (v *typeVisitor) VisitValDecl(decl *ir.ValDecl) {
	if decl.Sym != nil {
		v.visitValDeclSpec(decl.Sym, &decl.ValDeclSpec, decl.Init())
	}
}

func (v *typeVisitor) visitValDeclSpec(sym *ir.Symbol, decl *ir.ValDeclSpec, defaultInit bool) {
	if decl.Decl.Is(token.Val) {
		sym.Flags |= ir.SymFlagConstant
	}

	if sym.DepCycle() {
		return
	}

	if decl.Type != nil {
		v.identMode = identModeType
		ir.VisitExpr(v, decl.Type)
		v.identMode = identModeNone
		sym.T = decl.Type.Type()

		if typeSym := ir.ExprSymbol(decl.Type); typeSym != nil {
			if typeSym.ID != ir.TypeSymbol {
				v.c.error(decl.Type.FirstPos(), "'%s' is not a type", PrintExpr(decl.Type))
				sym.T = ir.TBuiltinUntyped
				return
			}
		}

		if sym.T.ID() == ir.TVoid {
			declType := "variable"
			if sym.Constant() {
				declType = "value"
			}
			v.c.error(decl.Type.FirstPos(), "cannot declare %s with type %s", declType, ir.TVoid)
			sym.T = ir.TBuiltinUntyped
			return
		}

		if ir.IsUntyped(sym.T) {
			return
		}
	}

	if decl.Initializer != nil {
		decl.Initializer = ir.VisitExpr(v, decl.Initializer)

		if decl.Type == nil {
			if !v.c.tryCoerceBigNumber(decl.Initializer) {
				return
			}
			sym.T = decl.Initializer.Type()
		} else {
			if !v.c.tryCastLiteral(decl.Initializer, sym.T) {
				return
			}
			if !sym.T.IsEqual(decl.Initializer.Type()) {
				v.c.error(decl.Initializer.FirstPos(), "type mismatch: '%s' has type %s and is not compatible with %s",
					decl.Name.Literal, sym.T, decl.Initializer.Type())
			}
		}
	} else if decl.Type == nil {
		v.c.error(decl.Name.Pos, "missing type specifier or initializer")
	} else if defaultInit {
		decl.Initializer = createDefaultLiteral(sym.T, decl.Type)
	}
}

func (v *typeVisitor) VisitFuncDecl(decl *ir.FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))

	if v.signature {
		for _, param := range decl.Params {
			v.VisitValDecl(param)
		}
		v.identMode = identModeType
		decl.TReturn = ir.VisitExpr(v, decl.TReturn)
		v.identMode = identModeNone

		typ := ir.NewFuncType(decl)

		if decl.Sym.T != nil && !decl.Sym.T.IsEqual(typ) {
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
	if !v.signature {
		return
	}

	for _, field := range decl.Fields {
		v.VisitValDecl(field)
	}
	decl.Sym.T = ir.NewStructType(decl)
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
	if stmt.Cond.Type().ID() != ir.TBool {
		v.c.error(stmt.Cond.FirstPos(), "if condition has type %s (expected %s)", stmt.Cond.Type(), ir.TBool)
	}

	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		ir.VisitStmt(v, stmt.Else)
	}
}
func (v *typeVisitor) VisitWhileStmt(stmt *ir.WhileStmt) {
	stmt.Cond = ir.VisitExpr(v, stmt.Cond)
	if stmt.Cond.Type().ID() != ir.TBool {
		v.c.error(stmt.Cond.FirstPos(), "while condition has type %s (expected %s)", stmt.Cond.Type(), ir.TBool)
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
		stmt.X = ir.VisitExpr(v, stmt.X)
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

func (v *typeVisitor) VisitAssignStmt(stmt *ir.AssignStmt) {
	stmt.Left = ir.VisitExpr(v, stmt.Left)
	if stmt.Left.Type().ID() == ir.TUntyped {
		return
	}

	var name *ir.Ident

	switch id := stmt.Left.(type) {
	case *ir.Ident:
		name = id
	case *ir.DotExpr:
		name = id.Name
	default:
		v.c.error(stmt.Left.FirstPos(), "invalid assignment")
		return
	}

	sym := name.Sym
	if sym == nil {
		return
	} else if sym.ID != ir.ValSymbol {
		v.c.error(name.Pos(), "invalid assignment: '%s' is not a variable", name.Literal())
		return
	}

	stmt.Right = ir.VisitExpr(v, stmt.Right)

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
		if !ir.IsNumericType(name.Type()) {
			v.c.error(name.Pos(), "type mismatch: %s is not numeric (has type %s)",
				stmt.Assign, name.Literal(), name.Type())
		}
	}
}

func (v *typeVisitor) VisitExprStmt(stmt *ir.ExprStmt) {
	stmt.X = ir.VisitExpr(v, stmt.X)
	v.c.tryCoerceBigNumber(stmt.X)
}
