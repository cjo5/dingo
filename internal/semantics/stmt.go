package semantics

import (
	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

func (v *typeChecker) VisitBlockStmt(stmt *ir.BlockStmt) {
	defer setScope(setScope(v.c, stmt.Scope))
	ir.VisitStmtList(v, stmt.Stmts)
}

func (v *typeChecker) VisitDeclStmt(stmt *ir.DeclStmt) {
	ir.VisitDecl(v, stmt.D)
}

func (v *typeChecker) VisitIfStmt(stmt *ir.IfStmt) {
	stmt.Cond = ir.VisitExpr(v, stmt.Cond)
	if !checkTypes(v.c, stmt.Cond.Type(), ir.TBuiltinBool) {
		v.c.error(stmt.Cond.Pos(), "condition has type %s (expected %s)", stmt.Cond.Type(), ir.TBool)
	}

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
		stmt.Cond = ir.VisitExpr(v, stmt.Cond)
		if !checkTypes(v.c, stmt.Cond.Type(), ir.TBuiltinBool) {
			v.c.error(stmt.Cond.Pos(), "condition has type %s (expected %s)", stmt.Cond.Type(), ir.TBool)
		}
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

	if stmt.X == nil {
		if retType.ID() != ir.TVoid {
			mismatch = true
		}
	} else {
		stmt.X = v.makeTypedExpr(stmt.X, retType)

		if !checkTypes(v.c, stmt.X.Type(), retType) {
			exprType = stmt.X.Type()
			mismatch = true
		}
	}

	if mismatch {
		v.c.errorNode(stmt.X, "function has return type %s (got type %s)", retType, exprType)
	}
}

func (v *typeChecker) VisitAssignStmt(stmt *ir.AssignStmt) {
	stmt.Left = ir.VisitExpr(v, stmt.Left)
	left := stmt.Left

	if ir.IsUntyped(left.Type()) {
		// Do nothing
	} else if !left.Lvalue() {
		v.c.error(left.Pos(), "expression is not an lvalue")
	} else if left.ReadOnly() {
		v.c.error(left.Pos(), "expression is read-only")
	}

	stmt.Right = v.makeTypedExpr(stmt.Right, left.Type())
	right := stmt.Right

	if !checkTypes(v.c, left.Type(), right.Type()) {
		v.c.errorNode(stmt, "type mismatch %s and %s", left.Type(), right.Type())
	}

	if stmt.Assign != token.Assign {
		if !ir.IsNumericType(left.Type()) {
			v.c.errorNode(left, "type %s is not numeric", left.Type())
		}
	}
}

func (v *typeChecker) VisitExprStmt(stmt *ir.ExprStmt) {
	stmt.X = v.makeTypedExpr(stmt.X, nil)
	if stmt.X.Type().ID() == ir.TUntyped {
		common.Assert(v.c.errors.IsError(), "expr is untyped and no error was reported")
	}
}
