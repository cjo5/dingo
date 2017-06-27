package semantics

import (
	"fmt"

	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/token"
)

var builtinScope *Scope

func addBuiltinType(scope *Scope, t TypeID) {
	sym := &Symbol{}
	sym.ID = TypeSymbol
	sym.T = &TType{ID: t}
	sym.Name = token.Synthetic(token.Ident, t.String())
	scope.Insert(sym)
}

func init() {
	builtinScope = NewScope(nil)
	addBuiltinType(builtinScope, TVoid)
	addBuiltinType(builtinScope, TBool)
	addBuiltinType(builtinScope, TString)
	addBuiltinType(builtinScope, TUInt32)
	addBuiltinType(builtinScope, TInt32)
}

// Check will resolve identifiers and do type checking.
func Check(mod *Module) error {
	var c checker

	c.checkModule(mod)
	if len(c.errors) > 0 {
		return c.errors
	}

	return nil
}

type checker struct {
	scope  *Scope
	errors common.ErrorList
}

func (c *checker) error(tok token.Token, format string, args ...interface{}) {
	c.errors.Add(tok.Pos, format, args...)
}

func (c *checker) errorPos(pos token.Position, format string, args ...interface{}) {
	c.errors.Add(pos, format, args...)
}

func (c *checker) openScope() {
	c.scope = NewScope(c.scope)
}

func (c *checker) closeScope() {
	c.scope = c.scope.Outer
}

func (c *checker) declare(id SymbolID, name token.Token, node Node) *Symbol {
	sym := NewSymbol(id, name, node, c.isGlobalScope())
	if existing := c.scope.Insert(sym); existing != nil {
		msg := fmt.Sprintf("redeclaration of '%s', previously declared at %s", name.Literal, existing.Pos())
		c.error(name, msg)
	}
	return sym
}

func (c *checker) resolve(name token.Token) *Symbol {
	if existing := c.scope.Lookup(name.Literal); existing == nil {
		c.error(name, "'%s' undefined", name.Literal)
	} else {
		return existing
	}
	return nil
}

func (c *checker) isGlobalScope() bool {
	return c.scope.Outer == builtinScope
}

func (c *checker) checkModule(mod *Module) {
	c.scope = builtinScope
	c.openScope()

	var funcs []*FuncDecl
	var vars []*VarDecl

	for _, decl := range mod.Decls {
		switch t := decl.(type) {
		case *VarDecl:
			vars = append(vars, t)
		case *FuncDecl:
			c.checkFuncDecl(t, true, false)
			funcs = append(funcs, t)
		}
	}

	var decls []Decl

	for _, decl := range vars {
		c.checkVarDecl(decl)
		decls = append(decls, decl)
	}

	for _, decl := range funcs {
		c.checkFuncDecl(decl, false, true)
		decls = append(decls, decl)
	}

	if len(c.errors) > 0 {
		return
	}

	mod.Decls = decls
	mod.Scope = c.scope
	c.closeScope()
}

func (c *checker) checkDecl(decl Decl) {
	switch t := decl.(type) {
	case *VarDecl:
		c.checkVarDecl(t)
	default:
		panic(fmt.Sprintf("Unhandled decl %T", t))
	}
}

func (c *checker) checkVarDecl(decl *VarDecl) {
	c.checkTypeSpec(decl.Type)

	if decl.X != nil {
		c.checkExpr(decl.X)

		if decl.Type.Type().ID != decl.X.Type().ID {
			c.errorPos(decl.X.FirstPos(), "type mismatch: '%s' has type %s and is not compatible with %s",
				decl.Name.Literal(), decl.Type.Type(), decl.X.Type())
		}
	}

	sym := c.declare(VarSymbol, decl.Name.Name, decl.Name)
	sym.T = decl.Type.Type()
}

func (c *checker) checkFuncDecl(decl *FuncDecl, signature bool, body bool) {
	if signature {
		c.declare(FuncSymbol, decl.Name.Name, decl)
		c.openScope()
		for _, param := range decl.Params {
			c.checkField(param)
		}

		c.checkTypeSpec(decl.Return)

		decl.Scope = c.scope
		c.closeScope()
	}
	if body {
		c.scope = decl.Scope
		c.checkBlockStmt(false, decl.Body)

		endsWithReturn := false
		retType := decl.Return.Type()
		for i, stmt := range decl.Body.Stmts {
			if ret, ok := stmt.(*ReturnStmt); ok {
				retTypeError := false
				exprType := TVoid
				if ret.X == nil {
					if retType.ID != TVoid {
						retTypeError = true
					}
				} else {
					if ret.X.Type().ID != retType.ID {
						exprType = ret.X.Type().ID
						retTypeError = true
					}
				}

				if retTypeError {
					c.error(ret.Return, "type mismatch: return type %s does not match function '%s' return type %s",
						exprType, decl.Name.Literal(), retType.ID)
				}

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

		c.closeScope()
	}
}

func (c *checker) checkField(field *Field) {
	c.checkTypeSpec(field.Type)
	sym := c.declare(VarSymbol, field.Name.Name, field.Name)
	sym.T = field.Type.Type()
	field.Name.Sym = sym
}

func (c *checker) checkTypeSpec(spec *Ident) {
	sym := c.scope.Lookup(spec.Literal())
	if sym == nil || sym.ID != TypeSymbol {
		c.error(spec.Name, "%s is not a type", spec.Literal())
	}
	spec.Sym = sym
}

func (c *checker) checkStmt(stmt Stmt) {
	switch t := stmt.(type) {
	case *BlockStmt:
		c.checkBlockStmt(true, t)
	case *DeclStmt:
		c.checkDecl(t.D)
	case *PrintStmt:
		c.checkExpr(t.X)
	case *AssignStmt:
		c.checkAssignStmt(t)
	case *ExprStmt:
		c.checkExpr(t.X)
	case *IfStmt:
		c.checkIfStmt(t)
	case *WhileStmt:
		c.checkWhileStmt(t)
	case *ReturnStmt:
		c.checkReturnStmt(t)
	}
}

func (c *checker) checkBlockStmt(newScope bool, stmt *BlockStmt) {
	if newScope {
		c.openScope()
	}
	for _, stmt := range stmt.Stmts {
		c.checkStmt(stmt)
	}
	stmt.Scope = c.scope
	if newScope {
		c.closeScope()
	}
}

func (c *checker) checkAssignStmt(stmt *AssignStmt) {
	c.checkIdent(stmt.Name)
	c.checkExpr(stmt.Right)

	if stmt.Name.Type().ID != stmt.Right.Type().ID {
		c.error(stmt.Name.Name, "type mismatch: '%s' is of type %s and it not compatible with %s",
			stmt.Name.Literal(), stmt.Name.Type(), stmt.Right.Type())
	}

	if stmt.Assign.ID != token.Assign {
		if stmt.Name.Type().ID != TInt32 || stmt.Right.Type().ID != TInt32 {
			c.error(stmt.Name.Name, "type mismatch: arguments to operation %s is not of type %s (got %s and %s)",
				stmt.Assign, TInt32, stmt.Name.Type().ID, stmt.Right.Type().ID)
		}
	}
}

func (c *checker) checkIfStmt(stmt *IfStmt) {
	c.checkExpr(stmt.Cond)
	if stmt.Cond.Type().ID != TBool {
		c.errorPos(stmt.Cond.FirstPos(), "if condition is not of type %s (has type %s)", TBool, stmt.Cond.Type())
	}

	c.checkBlockStmt(true, stmt.Body)
	if stmt.Else != nil {
		c.checkStmt(stmt.Else)
	}
}

func (c *checker) checkWhileStmt(stmt *WhileStmt) {
	c.checkExpr(stmt.Cond)
	if stmt.Cond.Type().ID != TBool {
		c.errorPos(stmt.Cond.FirstPos(), "while condition is not of type %s (has type %s)", TBool, stmt.Cond.Type())
	}
	c.checkBlockStmt(true, stmt.Body)
}

func (c *checker) checkReturnStmt(stmt *ReturnStmt) {
	if stmt.X != nil {
		c.checkExpr(stmt.X)
	}
}

func (c *checker) checkExpr(expr Expr) {
	switch t := expr.(type) {
	case *BinaryExpr:
		c.checkBinaryExpr(t)
	case *UnaryExpr:
		c.checkUnaryExpr(t)
	case *Literal:
		c.checkLiteral(t)
	case *Ident:
		c.checkIdent(t)
	case *CallExpr:
		c.checkCallExpr(t)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", t))
	}
}

func (c *checker) checkBinaryExpr(expr *BinaryExpr) {
	c.checkExpr(expr.Left)
	c.checkExpr(expr.Right)

	left := expr.Left.Type()
	right := expr.Right.Type()

	if left.ID != right.ID {
		c.errorPos(expr.Left.FirstPos(), "type mismatch: type %s and %s are not compatible", left, right)
	}

	binType := TInvalid
	switch expr.Op.ID {
	case token.And, token.Or:
		if left.ID != TBool || right.ID != TBool {
			c.errorPos(expr.Left.FirstPos(), "type mismatch: arguments to operation '%s' is not of type %s (got %s and %s)",
				expr.Op.ID, TBool, left.ID, right.ID)
		}
		binType = TBool
	case token.Eq, token.Neq, token.Gt, token.GtEq, token.Lt, token.LtEq:
		if left.ID == TString || right.ID == TString {
			c.errorPos(expr.Left.FirstPos(), "type mismatch: operation '%s' does not support type %s (got %s and %s)",
				expr.Op.ID, TString, left.ID, right.ID)

		}
		binType = TBool

	case token.Add, token.Sub, token.Mul, token.Div, token.Mod:
		if left.ID != TInt32 || right.ID != TInt32 {
			c.errorPos(expr.Left.FirstPos(), "type mismatch: arguments to operation '%s' is not of type %s (got %s and %s)",
				expr.Op.ID, TInt32, left.ID, right.ID)
		}
		binType = TInt32
	}

	expr.T = NewType(binType)
}

func (c *checker) checkUnaryExpr(expr *UnaryExpr) {
	c.checkExpr(expr.X)
	t := expr.X.Type()
	switch expr.Op.ID {
	case token.Sub:
		if t.ID != TInt32 {
			c.error(expr.Op, "type mismatch: operation '%s' expects type %s but got %s", token.Sub, TInt32, t)
		}
	case token.Lnot:
		if t.ID != TBool {
			c.error(expr.Op, "type mismatch: operation '%s' expects type %s but got %s", token.Lnot, TBool, t)
		}
	default:
		panic(fmt.Sprintf("Unhandled unary op %s", expr.Op.ID))
	}
	expr.T = t
}

func (c *checker) checkLiteral(lit *Literal) {
	if lit.Value.ID == token.False || lit.Value.ID == token.True {
		lit.T = NewType(TBool)
	} else if lit.Value.ID == token.LitString {
		lit.T = NewType(TString)
	} else if lit.Value.ID == token.LitInteger {
		// TODO: Check size
		lit.T = NewType(TInt32)
	} else {
		panic(fmt.Sprintf("Unhandled literal %s", lit.Value.ID))
	}
}

func (c *checker) checkIdent(id *Ident) {
	sym := c.resolve(id.Name)
	if sym == nil {
		c.error(id.Name, "'%s' undefined", id.Name.Literal)
	}
	id.Sym = sym
}

func (c *checker) checkCallExpr(call *CallExpr) {
	sym := c.scope.Lookup(call.Name.Literal())
	if sym == nil {
		c.error(call.Name.Name, "'%s' undefined", call.Name.Literal())
	} else if sym.ID != FuncSymbol {
		c.error(call.Name.Name, "'%s' is not a function", sym.Name.Literal)
	}

	c.checkIdent(call.Name)

	var decl *FuncDecl
	if sym != nil {
		decl, _ = sym.Src.(*FuncDecl)
		if len(decl.Params) != len(call.Args) {
			c.error(call.Name.Name, "'%s' takes %d argument(s) but called with %d", sym.Name.Literal, len(decl.Params), len(call.Args))
		}
	}

	for _, arg := range call.Args {
		c.checkExpr(arg)
	}

	if decl != nil {
		for i, arg := range call.Args {
			paramType := decl.Params[i].Name.Type()
			argType := arg.Type()

			if argType.ID != paramType.ID {
				c.errorPos(arg.FirstPos(), "type mismatch: argument %d of function '%s' expects type %s but got %s",
					i, call.Name.Literal(), paramType.ID, argType.ID)
			}
		}
		call.T = decl.Return.Type()
	} else {
		call.T = NewType(TInvalid)
	}
}
