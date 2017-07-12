package semantics

import (
	"fmt"

	"github.com/jhnl/interpreter/token"
)

type typeVisitor struct {
	BaseVisitor
	c *checker
}

func typeWalk(c *checker) {
	v := &typeVisitor{c: c}
	c.resetWalkState()
	StartProgramWalk(v, c.prog)
}

func (v *typeVisitor) Module(mod *Module) {
	v.c.mod = mod
	for _, decl := range mod.Decls {
		v.c.setTopDecl(decl)
		VisitDecl(v, decl)
	}
}

func (v *typeVisitor) VisitValTopDecl(decl *ValTopDecl) {
	if decl.Sym == nil {
		return
	}

	t := v.c.typeOf(decl.Type)
	decl.Sym.T = t
	if t == TBuiltinUntyped {
		return
	}

	if decl.Decl.Is(token.Val) {
		decl.Sym.Flags |= SymFlagConstant
	}

	if (decl.Sym.Flags & SymFlagDepCycle) != 0 {
		return
	}

	if decl.Initializer != nil {
		decl.Initializer = VisitExpr(v, decl.Initializer)

		if !v.c.tryCastLiteral(decl.Initializer, t) {
			return
		}

		if t.ID != decl.Initializer.Type().ID {
			v.c.error(decl.Initializer.FirstPos(), "type mismatch: '%s' has type %s and is not compatible with %s",
				decl.Name.Literal, t, decl.Initializer.Type())
		}
	} else {
		decl.Initializer = createDefaultValue(t)
	}
}

func (v *typeVisitor) VisitValDecl(decl *ValDecl) {
	if decl.Sym == nil {
		return
	}

	t := v.c.typeOf(decl.Type)
	decl.Sym.T = t
	if t == TBuiltinUntyped {
		return
	}

	if decl.Decl.Is(token.Val) {
		decl.Sym.Flags |= SymFlagConstant
	}

	if (decl.Sym.Flags & SymFlagDepCycle) != 0 {
		return
	}

	if decl.Initializer != nil {
		decl.Initializer = VisitExpr(v, decl.Initializer)

		if !v.c.tryCastLiteral(decl.Initializer, t) {
			return
		}

		if t.ID != decl.Initializer.Type().ID {
			v.c.error(decl.Initializer.FirstPos(), "type mismatch: '%s' has type %s and is not compatible with %s",
				decl.Name.Literal, t, decl.Initializer.Type())
		}
	} else {
		decl.Initializer = createDefaultValue(t)
	}
}

func createDefaultValue(t *TType) Expr {
	var lit *Literal
	if t.OneOf(TBool) {
		lit = &Literal{Value: token.Synthetic(token.False, token.False.String())}
		lit.T = NewType(TBool)
	} else if t.OneOf(TString) {
		lit = &Literal{Value: token.Synthetic(token.String, "")}
		lit.T = NewType(TString)
	} else if t.OneOf(TUInt64, TInt64, TUInt32, TInt32, TUInt16, TInt16, TUInt8, TInt8) {
		lit = &Literal{Value: token.Synthetic(token.Integer, "0")}
		lit.T = NewType(t.ID)
	} else if t.OneOf(TFloat64, TFloat32) {
		lit = &Literal{Value: token.Synthetic(token.Float, "0")}
		lit.T = NewType(t.ID)
	} else {
		panic(fmt.Sprintf("Unhandled init value for type %s", t.ID))
	}
	return lit
}

func (v *typeVisitor) VisitFuncDecl(decl *FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))

	for _, param := range decl.Params {
		if param.Sym != nil {
			param.Sym.T = v.c.typeOf(param.Type)
		}
	}

	decl.TReturn.T = v.c.typeOf(decl.TReturn.Name)

	v.VisitBlockStmt(decl.Body)

	// TODO: See if this can be improved

	endsWithReturn := false
	for i, stmt := range decl.Body.Stmts {
		if _, ok := stmt.(*ReturnStmt); ok {
			if (i + 1) == len(decl.Body.Stmts) {
				endsWithReturn = true
			}
		}
	}

	if !endsWithReturn {
		tok := token.Synthetic(token.Return, "return")
		returnStmt := &ReturnStmt{Return: tok}
		decl.Body.Stmts = append(decl.Body.Stmts, returnStmt)
	}
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
	if stmt.Cond.Type().ID != TBool {
		v.c.error(stmt.Cond.FirstPos(), "if condition is not of type %s (has type %s)", TBool, stmt.Cond.Type())
	}

	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		VisitStmt(v, stmt.Else)
	}
}
func (v *typeVisitor) VisitWhileStmt(stmt *WhileStmt) {
	stmt.Cond = VisitExpr(v, stmt.Cond)
	if stmt.Cond.Type().ID != TBool {
		v.c.error(stmt.Cond.FirstPos(), "while condition is not of type %s (has type %s)", TBool, stmt.Cond.Type())
	}
	v.VisitBlockStmt(stmt.Body)
}

func (v *typeVisitor) VisitReturnStmt(stmt *ReturnStmt) {
	mismatch := false

	exprType := TVoid
	funDecl, _ := v.c.topDecl.(*FuncDecl)
	retType := funDecl.TReturn.Type()
	if stmt.X == nil {
		if retType.ID != TVoid {
			mismatch = true
		}
	} else {
		stmt.X = VisitExpr(v, stmt.X)
		if !v.c.tryCastLiteral(stmt.X, retType) {
			exprType = stmt.X.Type().ID
			mismatch = true
		} else if stmt.X.Type().ID != retType.ID {
			exprType = stmt.X.Type().ID
			mismatch = true
		}
	}

	if mismatch {
		v.c.error(stmt.Return.Pos, "type mismatch: return type %s does not match function '%s' return type %s",
			exprType, funDecl.Name.Literal, retType.ID)
	}
}

func (v *typeVisitor) VisitAssignStmt(stmt *AssignStmt) {
	v.VisitIdent(stmt.Name)
	sym := v.c.lookup(stmt.Name.Name)
	if sym == nil {
		return
	}

	stmt.Right = VisitExpr(v, stmt.Right)

	if sym.Constant() {
		v.c.error(stmt.Name.Pos(), "'%s' was declared with %s and cannot be modified (constant)",
			stmt.Name.Literal(), token.Val)
	}

	if !v.c.tryCastLiteral(stmt.Right, stmt.Name.Type()) {
		return
	}

	if stmt.Name.Type().ID != stmt.Right.Type().ID {
		v.c.error(stmt.Name.Pos(), "type mismatch: '%s' is of type %s and it not compatible with %s",
			stmt.Name.Literal(), stmt.Name.Type(), stmt.Right.Type())
	}

	if stmt.Assign.ID != token.Assign {
		if !stmt.Name.Type().IsNumericType() {
			v.c.error(stmt.Name.Pos(), "type mismatch: %s is not numeric (has type %s)",
				stmt.Assign, stmt.Name.Literal(), stmt.Name.Type().ID)
		}
	}
}

func (v *typeVisitor) VisitExprStmt(stmt *ExprStmt) {
	stmt.X = VisitExpr(v, stmt.X)
}
