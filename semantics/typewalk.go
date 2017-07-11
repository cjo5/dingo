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
	v.VisitProgram(c.prog)
}

func (v *typeVisitor) VisitProgram(prog *Program) {
	for _, mod := range prog.Modules {
		v.c.mod = mod
		for _, toplevelDecl := range mod.Decls {
			v.c.file = toplevelDecl.File
			v.c.scope = toplevelDecl.File.Scope

			for _, decl := range toplevelDecl.Decls {
				VisitDecl(v, decl)
			}

			v.c.scope = nil
			v.c.file = nil
		}
		v.c.mod = nil
	}
}

func (v *typeVisitor) VisitVarDecl(decl *VarDecl) {
	if decl.Sym == nil {
		return
	}

	t := v.c.typeOf(decl.Type.Name)
	decl.Sym.T = t
	decl.Name.T = t
	if t == TBuiltinUntyped {
		return
	}

	if decl.Decl.Is(token.Val) {
		decl.Sym.Flags |= SymFlagConstant
	}

	if (decl.Sym.Flags & SymFlagDepCycle) != 0 {
		return
	}

	if decl.X != nil {
		decl.X = VisitExpr(v, decl.X)

		if !v.c.tryCastLiteral(decl.X, t) {
			return
		}

		if t.ID != decl.X.Type().ID {
			v.c.error(decl.X.FirstPos(), "type mismatch: '%s' has type %s and is not compatible with %s",
				decl.Name.Literal(), decl.Name.Type(), decl.X.Type())
		}
	} else {
		// Default values
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
		decl.X = lit
	}
}

func (v *typeVisitor) VisitFuncDecl(decl *FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))
	v.c.fun = decl

	for _, param := range decl.Params {
		if param.Sym != nil {
			param.Sym.T = v.c.typeOf(param.Type.Name)
			param.Name.T = param.Sym.T
		}
	}

	decl.Return.T = v.c.typeOf(decl.Return.Name)

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

	v.c.fun = nil
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
	retType := v.c.fun.Return.Type()
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
			exprType, v.c.fun.Name.Literal(), retType.ID)
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
