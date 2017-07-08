package semantics

import (
	"fmt"

	"github.com/jhnl/interpreter/token"
)

type typeVisitor struct {
	BaseVisitor
	c    *typeChecker
	decl Decl
}

func typeCheck(c *typeChecker) {
	v := &typeVisitor{c: c}
	v.VisitProgram(c.prog)
}

func setScope(c *typeChecker, scope *Scope) *Scope {
	curr := c.scope
	c.scope = curr
	return curr
}

func (v *typeVisitor) VisitModule(mod *Module) {
	defer setScope(v.c, setScope(v.c, mod.Internal))
	v.c.scope = mod.Internal
	VisitFileList(v, mod.Files)
	v.c.mod = nil
}

func (v *typeVisitor) VisitFile(file *File) {
	defer setScope(v.c, setScope(v.c, file.Scope))
	v.c.file = file
	VisitDeclList(v, file.Decls)
	v.c.file = nil
}

func (v *typeVisitor) VisitVarDecl(decl *VarDecl) {
	t := v.c.typeOf(decl.Type.Name)
	decl.Name.Sym.T = t
	if t == TBuiltinUntyped {
		return
	}

	if decl.Decl.Is(token.Val) {
		decl.Name.Sym.Constant = true
	}

	if decl.X != nil {
		v.decl = decl
		decl.X = v.VisitExpr(decl.X)
		v.decl = nil
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
	defer setScope(v.c, setScope(v.c, decl.Scope))
	v.c.fun = decl

	for _, param := range decl.Params {
		param.Name.Sym.T = v.c.typeOf(param.Type.Name)
	}

	decl.Return.Sym.T = v.c.typeOf(decl.Return.Name)

	v.decl = decl
	v.VisitBlockStmt(decl.Body)
	v.decl = nil

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

func (v *typeVisitor) VisitPrintStmt(stmt *PrintStmt) {
	stmt.X = v.VisitExpr(stmt.X)
}

func (v *typeVisitor) VisitIfStmt(stmt *IfStmt) {
	stmt.Cond = v.VisitExpr(stmt.Cond)
	if stmt.Cond.Type().ID != TBool {
		v.c.errorPos(stmt.Cond.FirstPos(), "if condition is not of type %s (has type %s)", TBool, stmt.Cond.Type())
	}

	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		v.VisitStmt(stmt.Else)
	}
}
func (v *typeVisitor) VisitWhileStmt(stmt *WhileStmt) {
	stmt.Cond = v.VisitExpr(stmt.Cond)
	if stmt.Cond.Type().ID != TBool {
		v.c.errorPos(stmt.Cond.FirstPos(), "while condition is not of type %s (has type %s)", TBool, stmt.Cond.Type())
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
		stmt.X = v.VisitExpr(stmt.X)
		if !v.c.tryCastLiteral(stmt.X, retType) {
			exprType = stmt.X.Type().ID
			mismatch = true
		} else if stmt.X.Type().ID != retType.ID {
			exprType = stmt.X.Type().ID
			mismatch = true
		}
	}

	if mismatch {
		v.c.error(stmt.Return, "type mismatch: return type %s does not match function '%s' return type %s",
			exprType, v.c.fun.Name.Literal(), retType.ID)
	}

}

func (v *typeVisitor) VisitAssignStmt(stmt *AssignStmt) {
	v.VisitIdent(stmt.Name)
	stmt.Right = v.VisitExpr(stmt.Right)

	if stmt.Name.Sym.Constant {
		v.c.error(stmt.Name.Name, "'%s' was declared with %s and cannot be modified (constant)",
			stmt.Name.Literal(), token.Val)
	}

	if !v.c.tryCastLiteral(stmt.Right, stmt.Name.Type()) {
		return
	}

	if stmt.Name.Type().ID != stmt.Right.Type().ID {
		v.c.error(stmt.Name.Name, "type mismatch: '%s' is of type %s and it not compatible with %s",
			stmt.Name.Literal(), stmt.Name.Type(), stmt.Right.Type())
	}

	if stmt.Assign.ID != token.Assign {
		if !stmt.Name.Type().IsNumericType() {
			v.c.error(stmt.Name.Name, "type mismatch: %s is not numeric (has type %s)",
				stmt.Assign, stmt.Name.Literal(), stmt.Name.Type().ID)
		}
	}
}

func (v *typeVisitor) VisitExprStmt(stmt *ExprStmt) {
	stmt.X = v.VisitExpr(stmt.X)
}
