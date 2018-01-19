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
	stmt.Cond = ir.VisitExpr(v, stmt.Cond)
	if !checkTypes(v.c, stmt.Cond.Type(), ir.TBuiltinBool) {
		v.c.error(stmt.Cond.FirstPos(), "condition has type %s (expected %s)", stmt.Cond.Type(), ir.TBool)
	}

	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		ir.VisitStmt(v, stmt.Else)
	}
}
func (v *typeChecker) VisitForStmt(stmt *ir.ForStmt) {
	defer setScope(setScope(v.c, stmt.Scope))

	if stmt.Init != nil {
		v.VisitValDecl(stmt.Init)
	}

	if stmt.Cond != nil {
		stmt.Cond = ir.VisitExpr(v, stmt.Cond)
		if !checkTypes(v.c, stmt.Cond.Type(), ir.TBuiltinBool) {
			v.c.error(stmt.Cond.FirstPos(), "condition has type %s (expected %s)", stmt.Cond.Type(), ir.TBool)
		}
	}

	if stmt.Inc != nil {
		ir.VisitStmt(v, stmt.Inc)
	}

	v.VisitBlockStmt(stmt.Body)
}

func (v *typeChecker) VisitReturnStmt(stmt *ir.ReturnStmt) {
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

		if !checkTypes(v.c, stmt.X.Type(), retType) {
			exprType = stmt.X.Type().ID()
			mismatch = true
		}
	}

	if mismatch {
		v.c.error(stmt.Return.Pos, "type mismatch: return type %s does not match function '%s' return type %s",
			exprType, funDecl.Name.Literal, retType)
	}
}

func (v *typeChecker) VisitAssignStmt(stmt *ir.AssignStmt) {
	stmt.Left = ir.VisitExpr(v, stmt.Left)
	if stmt.Left.Type().ID() == ir.TUntyped {
		return
	}

	left := stmt.Left
	if !left.Lvalue() {
		v.c.error(stmt.Left.FirstPos(), "expression is not an lvalue")
		return
	}

	stmt.Right = v.makeTypedExpr(stmt.Right, left.Type())

	if stmt.Left.ReadOnly() {
		v.c.error(stmt.Left.FirstPos(), "expression is read-only")
	}

	if !checkTypes(v.c, left.Type(), stmt.Right.Type()) {
		v.c.error(left.FirstPos(), "type mismatch %s and %s", left.Type(), stmt.Right.Type())
	}

	if stmt.Assign.ID != token.Assign {
		if !ir.IsNumericType(left.Type()) {
			v.c.error(left.FirstPos(), "type mismatch: type %s is not numeric", left.Type())
		}
	}
}

func (v *typeChecker) VisitExprStmt(stmt *ir.ExprStmt) {
	stmt.X = v.makeTypedExpr(stmt.X, nil)
	texpr := stmt.X.Type()
	if texpr.ID() == ir.TUntyped {
		common.Assert(v.c.errors.IsError(), "expr is untyped and no error was reported")
	}
}
