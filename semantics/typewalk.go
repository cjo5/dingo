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
	typeSym := v.c.typeOfSym(decl.Type)
	if typeSym == nil {
		sym.T = TBuiltinUntyped
		return
	}
	sym.T = typeSym.T

	if decl.Decl.Is(token.Val) {
		sym.Flags |= SymFlagConstant
	}

	if (sym.Flags & SymFlagDepCycle) != 0 {
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
		decl.Initializer = createDefaultLiteral(typeSym)
	}
}

func (v *typeVisitor) VisitFuncDecl(decl *FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))

	for _, param := range decl.Params {
		v.VisitValDecl(param)
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
		if !decl.TReturn.T.IsEqual(TBuiltinVoid) {
			v.c.error(decl.Body.Rbrace.Pos, "missing return")
		} else {
			tok := token.Synthetic(token.Return, "return")
			returnStmt := &ReturnStmt{Return: tok}
			decl.Body.Stmts = append(decl.Body.Stmts, returnStmt)

		}
	}
}

func (v *typeVisitor) VisitStructDecl(decl *StructDecl) {
	defer setScope(setScope(v.c, decl.Scope))
	decl.Sym.T = NewType(TStruct, decl.Name.Literal)

	for _, field := range decl.Fields {
		v.VisitValDecl(field)
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
	stmt.Left = VisitExpr(v, stmt.Left)

	var sym *Symbol
	var name *Ident
	constant := false

	switch t := stmt.Left.(type) {
	case *Ident:
		sym = t.Sym
		constant = t.Sym.Constant()
		name = t
	case *DotExpr:
		switch right := t.X.(type) {
		case *Ident:
			sym = right.Sym
			constant = t.Name.Sym.Constant()
			name = right
		case *FuncCall:
			sym = right.Name.Sym
			name = right.Name
		default:
			panic(fmt.Sprintf("Unhandled DotExpr %T", right))
		}
	default:
		v.c.error(stmt.Left.FirstPos(), "invalid assignment")
		return
	}

	if sym == nil {
		return
	} else if sym.ID != ValSymbol {
		v.c.error(name.Pos(), "invalid assignment: '%s' is not a variable", name.Literal())
		return
	}

	stmt.Right = VisitExpr(v, stmt.Right)

	if constant {
		v.c.error(name.Pos(), "'%s' was declared with %s and cannot be modified (constant)",
			name.Literal(), token.Val)
	}

	if !v.c.tryCastLiteral(stmt.Right, name.Type()) {
		return
	}

	if !name.Type().IsEqual(stmt.Right.Type()) {
		v.c.error(name.Pos(), "type mismatch: '%s' is of type %s and is not compatible with %s",
			name.Literal(), name.Type(), stmt.Right.Type())
	}

	if stmt.Assign.ID != token.Assign {
		if !name.Type().IsNumericType() {
			v.c.error(name.Pos(), "type mismatch: %s is not numeric (has type %s)",
				stmt.Assign, name.Literal(), name.Type())
		}
	}
}

func (v *typeVisitor) VisitExprStmt(stmt *ExprStmt) {
	stmt.X = VisitExpr(v, stmt.X)
}
