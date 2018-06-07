package semantics

import (
	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

func (v *typeChecker) VisitBlockStmt(stmt *ir.BlockStmt) {
	defer setScope(setScope(v.c, stmt.Scope))
	ir.VisitStmtList(v, stmt.Stmts)
}

func (v *typeChecker) VisitDeclStmt(stmt *ir.DeclStmt) {
	ir.VisitDecl(v, stmt.D)
}

func (v *typeChecker) VisitIfStmt(stmt *ir.IfStmt) {
	cond := ir.VisitExpr(v, stmt.Cond)
	if !checkTypes(v.c, cond.Type(), ir.TBuiltinBool) {
		v.c.error(stmt.Cond.Pos(), "condition has type %s (expected %s)", cond.Type(), ir.TBool)
	}

	stmt.Cond = cond

	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		ir.VisitStmt(v, stmt.Else)
	}
}
func (v *typeChecker) VisitForStmt(stmt *ir.ForStmt) {
	defer setScope(setScope(v.c, stmt.Body.Scope))

	if stmt.Init != nil {
		v.VisitValDecl(stmt.Init)
	}

	if stmt.Cond != nil {
		cond := ir.VisitExpr(v, stmt.Cond)
		if !checkTypes(v.c, cond.Type(), ir.TBuiltinBool) {
			v.c.error(stmt.Cond.Pos(), "condition has type %s (expected %s)", cond.Type(), ir.TBool)
		}
		stmt.Cond = cond
	}

	if stmt.Inc != nil {
		ir.VisitStmt(v, stmt.Inc)
	}

	ir.VisitStmtList(v, stmt.Body.Stmts)
}

func (v *typeChecker) VisitReturnStmt(stmt *ir.ReturnStmt) {
	mismatch := false

	funDecl, _ := v.c.topDecl().(*ir.FuncDecl)
	retType := funDecl.Return.Type.Type()
	if ir.IsUntyped(retType) {
		return
	}

	exprType := ir.TBuiltinVoid
	x := stmt.X

	if x == nil {
		if retType.ID() != ir.TVoid {
			mismatch = true
		}
	} else {
		x = v.makeTypedExpr(x, retType)

		if !checkTypes(v.c, x.Type(), retType) {
			exprType = x.Type()
			mismatch = true
		}
	}

	if mismatch {
		v.c.errorExpr(stmt.X, "function has return type %s (got type %s)", retType, exprType)
	}

	stmt.X = x
}

func (v *typeChecker) VisitAssignStmt(stmt *ir.AssignStmt) {
	left := ir.VisitExpr(v, stmt.Left)

	if ir.IsUntyped(left.Type()) {
		// Do nothing
	} else if !left.Lvalue() {
		v.c.error(stmt.Left.Pos(), "expression is not an lvalue")
	} else if stmt.Left.ReadOnly() {
		v.c.error(stmt.Left.Pos(), "expression is read-only")
	}

	right := v.makeTypedExpr(stmt.Right, left.Type())

	if !checkTypes(v.c, left.Type(), right.Type()) {
		v.c.error(stmt.Assign.Pos, "type mismatch %s and %s", left.Type(), stmt.Right.Type())
	}

	if stmt.Assign.ID != token.Assign {
		if !ir.IsNumericType(left.Type()) {
			v.c.errorExpr(stmt.Left, "type %s is not numeric", left.Type())
		}
	}

	stmt.Left = left
	stmt.Right = right
}

func (v *typeChecker) VisitExprStmt(stmt *ir.ExprStmt) {
	stmt.X = v.makeTypedExpr(stmt.X, nil)
	if stmt.X.Type().ID() == ir.TUntyped {
		common.Assert(v.c.errors.IsError(), "expr is untyped and no error was reported")
	}
}
