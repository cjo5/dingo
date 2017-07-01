package semantics

import (
	"fmt"

	"math/big"

	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/token"
)

var builtinScope = NewScope(nil)

func addBuiltinType(t *TType) {
	sym := &Symbol{}
	sym.ID = TypeSymbol
	sym.T = t
	sym.Name = token.Synthetic(token.Ident, t.String())
	builtinScope.Insert(sym)
}

func init() {
	addBuiltinType(TBuiltinVoid)
	addBuiltinType(TBuiltinBool)
	addBuiltinType(TBuiltinString)
	addBuiltinType(TBuiltinUInt64)
	addBuiltinType(TBuiltinInt64)
	addBuiltinType(TBuiltinUInt32)
	addBuiltinType(TBuiltinInt32)
	addBuiltinType(TBuiltinUInt16)
	addBuiltinType(TBuiltinInt16)
	addBuiltinType(TBuiltinUInt8)
	addBuiltinType(TBuiltinInt8)
	addBuiltinType(TBuiltinFloat64)
	addBuiltinType(TBuiltinFloat32)
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
	scope    *Scope
	errors   common.ErrorList
	currFunc *FuncDecl
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

// Returns false if error
func (c *checker) tryCastLiteral(expr Expr, target *TType) bool {
	if target.IsNumericType() && expr.Type().IsNumericType() {
		lit, _ := expr.(*Literal)
		if lit != nil {
			castResult := typeCastNumericLiteral(lit, target)

			if castResult == numericCastOK {
				return true
			}

			if castResult == numericCastOverflows {
				c.error(lit.Value, "constant expression %s overflows %s", lit.Value.Literal, target.ID)
			} else if castResult == numericCastTruncated {
				c.error(lit.Value, "type mismatch: constant float expression %s not compatible with %s", lit.Value.Literal, target.ID)
			} else {
				panic(fmt.Sprintf("Unhandled numeric cast result %d", castResult))
			}

			return false
		}
	}
	return true
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
	t := decl.Type.Type()

	sym := c.declare(VarSymbol, decl.Name.Name, decl.Name)
	sym.T = t
	if decl.Decl.ID == token.Val {
		sym.Constant = true
	}

	if decl.X != nil {
		decl.X = c.checkExpr(decl.X)

		if !c.tryCastLiteral(decl.X, decl.Type.Type()) {
			return
		}

		if decl.Type.Type().ID != decl.X.Type().ID {
			c.errorPos(decl.X.FirstPos(), "type mismatch: '%s' has type %s and is not compatible with %s",
				decl.Name.Literal(), decl.Type.Type(), decl.X.Type())
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

		c.currFunc = decl
		c.checkBlockStmt(false, decl.Body)
		c.currFunc = nil

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
		t.X = c.checkExpr(t.X)
	case *AssignStmt:
		c.checkAssignStmt(t)
	case *ExprStmt:
		t.X = c.checkExpr(t.X)
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
	stmt.Right = c.checkExpr(stmt.Right)

	if stmt.Name.Sym.Constant {
		c.error(stmt.Name.Name, "'%s' was declared with %s and cannot be modified (constant)",
			stmt.Name.Literal(), token.Val)
	}

	if !c.tryCastLiteral(stmt.Right, stmt.Name.Type()) {
		return
	}

	if stmt.Name.Type().ID != stmt.Right.Type().ID {
		c.error(stmt.Name.Name, "type mismatch: '%s' is of type %s and it not compatible with %s",
			stmt.Name.Literal(), stmt.Name.Type(), stmt.Right.Type())
	}

	if stmt.Assign.ID != token.Assign {
		if !stmt.Name.Type().IsNumericType() {
			c.error(stmt.Name.Name, "type mismatch: %s is not numeric (has type %s)",
				stmt.Assign, stmt.Name.Literal(), stmt.Name.Type().ID)
		}
	}
}

func (c *checker) checkIfStmt(stmt *IfStmt) {
	stmt.Cond = c.checkExpr(stmt.Cond)
	if stmt.Cond.Type().ID != TBool {
		c.errorPos(stmt.Cond.FirstPos(), "if condition is not of type %s (has type %s)", TBool, stmt.Cond.Type())
	}

	c.checkBlockStmt(true, stmt.Body)
	if stmt.Else != nil {
		c.checkStmt(stmt.Else)
	}
}

func (c *checker) checkWhileStmt(stmt *WhileStmt) {
	stmt.Cond = c.checkExpr(stmt.Cond)
	if stmt.Cond.Type().ID != TBool {
		c.errorPos(stmt.Cond.FirstPos(), "while condition is not of type %s (has type %s)", TBool, stmt.Cond.Type())
	}
	c.checkBlockStmt(true, stmt.Body)
}

func (c *checker) checkReturnStmt(stmt *ReturnStmt) {
	mismatch := false

	exprType := TVoid
	retType := c.currFunc.Return.Type()
	if stmt.X == nil {
		if retType.ID != TVoid {
			mismatch = true
		}
	} else {
		stmt.X = c.checkExpr(stmt.X)
		if !c.tryCastLiteral(stmt.X, retType) {
			exprType = stmt.X.Type().ID
			mismatch = true
		} else if stmt.X.Type().ID != retType.ID {
			exprType = stmt.X.Type().ID
			mismatch = true
		}
	}

	if mismatch {
		c.error(stmt.Return, "type mismatch: return type %s does not match function '%s' return type %s",
			exprType, c.currFunc.Name.Literal(), retType.ID)
	}
}

func (c *checker) checkExpr(expr Expr) Expr {
	switch t := expr.(type) {
	case *BinaryExpr:
		return c.checkBinaryExpr(t)
	case *UnaryExpr:
		return c.checkUnaryExpr(t)
	case *Literal:
		return c.checkLiteral(t)
	case *Ident:
		return c.checkIdent(t)
	case *CallExpr:
		return c.checkCallExpr(t)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", t))
	}
}

// TODO: Evaluate constant boolean expressions

func (c *checker) checkBinaryExpr(expr *BinaryExpr) Expr {
	expr.Left = c.checkExpr(expr.Left)
	expr.Right = c.checkExpr(expr.Right)

	leftType := expr.Left.Type()
	rightType := expr.Right.Type()

	binType := TUntyped
	boolOp := expr.Op.OneOf(token.Eq, token.Neq, token.Gt, token.GtEq, token.Lt, token.LtEq)
	arithOp := expr.Op.OneOf(token.Add, token.Sub, token.Mul, token.Div, token.Mod)
	typeNotSupported := TUntyped

	if expr.Op.OneOf(token.And, token.Or) {
		if leftType.ID != TBool || rightType.ID != TBool {
			c.error(expr.Op, "type mismatch: arguments to operation '%s' are not of type %s (got %s and %s)",
				expr.Op.ID, TBool, leftType.ID, rightType.ID)
		} else {
			binType = TBool
		}
	} else if boolOp || arithOp {
		leftLit, _ := expr.Left.(*Literal)
		rightLit, _ := expr.Right.(*Literal)

		if leftType.IsNumericType() && rightType.IsNumericType() {
			var leftBigInt *big.Int
			var leftBigFloat *big.Float
			var rightBigInt *big.Int
			var rightBigFloat *big.Float

			if leftLit != nil {
				leftBigInt, _ = leftLit.Raw.(*big.Int)
				leftBigFloat, _ = leftLit.Raw.(*big.Float)
			}
			if rightLit != nil {
				rightBigInt, _ = rightLit.Raw.(*big.Int)
				rightBigFloat, _ = rightLit.Raw.(*big.Float)
			}

			// Check division by zero

			if expr.Op.ID == token.Div || expr.Op.ID == token.Mod {
				if (rightBigInt != nil && rightBigInt.Cmp(BigIntZero) == 0) ||
					(rightBigFloat != nil && rightBigFloat.Cmp(BigFloatZero) == 0) {
					c.error(rightLit.Value, "Division by zero")
					expr.T = NewType(TUntyped)
					return expr
				}
			}

			// Convert integer literals to floats

			if leftBigInt != nil && rightBigFloat != nil {
				leftBigFloat = big.NewFloat(0)
				leftBigFloat.SetInt(leftBigInt)
				leftLit.Raw = leftBigFloat
				leftLit.T = NewType(TBigFloat)
				leftType = leftLit.T
				leftBigInt = nil
			}

			if rightBigInt != nil && leftBigFloat != nil {
				rightBigFloat = big.NewFloat(0)
				rightBigFloat.SetInt(rightBigInt)
				rightLit.Raw = rightBigFloat
				rightLit.T = NewType(TBigFloat)
				rightType = rightLit.T
				rightBigInt = nil
			}

			bigIntOperands := (leftBigInt != nil && rightBigInt != nil)
			bigFloatOperands := (leftBigFloat != nil && rightBigFloat != nil)

			if bigIntOperands || bigFloatOperands {
				cmpRes := 0
				if bigIntOperands {
					cmpRes = leftBigInt.Cmp(rightBigInt)
				} else {
					cmpRes = leftBigFloat.Cmp(rightBigFloat)
				}

				boolRes := false
				switch expr.Op.ID {
				case token.Eq:
					boolRes = (cmpRes == 0)
				case token.Neq:
					boolRes = (cmpRes != 0)
				case token.Gt:
					boolRes = (cmpRes > 0)
				case token.GtEq:
					boolRes = (cmpRes >= 0)
				case token.Lt:
					boolRes = (cmpRes < 0)
				case token.LtEq:
					boolRes = (cmpRes <= 0)
				default:
					if bigIntOperands {
						switch expr.Op.ID {
						case token.Add:
							leftBigInt.Add(leftBigInt, rightBigInt)
						case token.Sub:
							leftBigInt.Sub(leftBigInt, rightBigInt)
						case token.Mul:
							leftBigInt.Mul(leftBigInt, rightBigInt)
						case token.Div:
							leftBigInt.Div(leftBigInt, rightBigInt)
						case token.Mod:
							leftBigInt.Mod(leftBigInt, rightBigInt)
						default:
							panic(fmt.Sprintf("Unhandled binop %s", expr.Op.ID))
						}
					} else {
						switch expr.Op.ID {
						case token.Add:
							leftBigFloat.Add(leftBigFloat, rightBigFloat)
						case token.Sub:
							leftBigFloat.Sub(leftBigFloat, rightBigFloat)
						case token.Mul:
							leftBigFloat.Mul(leftBigFloat, rightBigFloat)
						case token.Div:
							leftBigFloat.Quo(leftBigFloat, rightBigFloat)
						case token.Mod:
							typeNotSupported = leftType.ID
						default:
							panic(fmt.Sprintf("Unhandled binop %s", expr.Op.ID))
						}
					}
				}

				if typeNotSupported == TUntyped {
					if boolOp {
						if boolRes {
							leftLit.Value.ID = token.True
						} else {
							leftLit.Value.ID = token.False
						}
						leftLit.T = NewType(TBool)
						leftLit.Raw = nil
					}

					leftLit.Value.Literal = "(" + leftLit.Value.Literal + " " + expr.Op.Literal + " " + rightLit.Value.Literal + ")"
					leftLit.Rewrite++
					return leftLit
				}
			} else if leftBigInt != nil && rightBigInt == nil {
				typeCastNumericLiteral(leftLit, rightType)
				leftType = leftLit.T
			} else if leftBigInt == nil && rightBigInt != nil {
				typeCastNumericLiteral(rightLit, leftType)
				rightType = rightLit.T
			} else if leftBigFloat != nil && rightBigFloat == nil {
				typeCastNumericLiteral(leftLit, rightType)
				leftType = leftLit.T
			} else if leftBigFloat == nil && rightBigFloat != nil {
				typeCastNumericLiteral(rightLit, leftType)
				rightType = rightLit.T
			}
		} else if leftType.OneOf(TBool) && rightType.OneOf(TBool) {
			if arithOp || expr.Op.OneOf(token.Gt, token.GtEq, token.Lt, token.LtEq) {
				typeNotSupported = TBool
			}
		} else if leftType.OneOf(TString) && rightType.OneOf(TString) {
			typeNotSupported = TString
		}

		if typeNotSupported != TUntyped {
			c.error(expr.Op, "operation '%s' does not support type %s", expr.Op.ID, typeNotSupported)
		} else if leftType.ID != rightType.ID {
			c.error(expr.Op, "type mismatch: arguments to operation '%s' are not compatible (got %s and %s)",
				expr.Op.ID, leftType.ID, rightType.ID)
		} else {
			if boolOp {
				binType = TBool
			} else {
				binType = leftType.ID
			}
		}
	} else {
		panic(fmt.Sprintf("Unhandled binop %s", expr.Op.ID))
	}

	expr.T = NewType(binType)
	return expr
}

func (c *checker) checkUnaryExpr(expr *UnaryExpr) Expr {
	expr.X = c.checkExpr(expr.X)
	expr.T = expr.X.Type()
	switch expr.Op.ID {
	case token.Sub:
		if !expr.T.IsNumericType() {
			c.error(expr.Op, "type mismatch: operation '%s' expects a numeric type but got %s", token.Sub, expr.T.ID)
		} else if lit, ok := expr.X.(*Literal); ok {
			var raw interface{}

			switch n := lit.Raw.(type) {
			case *big.Int:
				raw = n.Neg(n)
			case *big.Float:
				raw = n.Neg(n)
			default:
				panic(fmt.Sprintf("Unhandled raw type %T", n))
			}

			lit.Value.Pos = expr.Op.Pos
			if lit.Rewrite > 0 {
				lit.Value.Literal = "(" + lit.Value.Literal + ")"
			}
			lit.Value.Literal = expr.Op.Literal + lit.Value.Literal
			lit.Rewrite++
			lit.Raw = raw
			return lit
		}
	case token.Lnot:
		if expr.T.ID != TBool {
			c.error(expr.Op, "type mismatch: operation '%s' expects type %s but got %s", token.Lnot, TBool, expr.T.ID)
		}
	default:
		panic(fmt.Sprintf("Unhandled unary op %s", expr.Op.ID))
	}
	return expr
}

func unescapeString(literal string) string {
	// TODO:
	// - Handle more escape sequences
	// - Improve rune handling

	escaped := []rune(literal)
	var unescaped []rune

	start := 0
	n := len(escaped)

	// Remove quotes
	if n >= 2 {
		start++
		n--
	}

	for i := start; i < n; i++ {
		ch1 := escaped[i]
		if ch1 == '\\' && (i+1) < len(escaped) {
			i++
			ch2 := escaped[i]

			if ch2 == 'a' {
				ch1 = 0x07
			} else if ch2 == 'b' {
				ch1 = 0x08
			} else if ch2 == 'f' {
				ch1 = 0x0c
			} else if ch2 == 'n' {
				ch1 = 0x0a
			} else if ch2 == 'r' {
				ch1 = 0x0d
			} else if ch2 == 't' {
				ch1 = 0x09
			} else if ch2 == 'v' {
				ch1 = 0x0b
			} else {
				ch1 = ch2
			}
		}
		unescaped = append(unescaped, ch1)
	}

	return string(unescaped)
}

func (c *checker) checkLiteral(lit *Literal) Expr {
	if lit.Value.ID == token.False || lit.Value.ID == token.True {
		lit.T = NewType(TBool)
	} else if lit.Value.ID == token.String {
		if lit.Raw == nil {
			lit.T = NewType(TString)
			lit.Raw = unescapeString(lit.Value.Literal)
		}
	} else if lit.Value.ID == token.Integer {
		if lit.Raw == nil {
			val := big.NewInt(0)
			_, ok := val.SetString(lit.Value.Literal, 10)
			if !ok {
				c.error(lit.Value, "unable to interpret integer literal %s", lit.Value.Literal)
			}
			lit.T = NewType(TBigInt)
			lit.Raw = val
		}
	} else if lit.Value.ID == token.Float {
		if lit.Raw == nil {
			val := big.NewFloat(0)
			_, ok := val.SetString(lit.Value.Literal)
			if !ok {
				c.error(lit.Value, "unable to interpret float literal %s", lit.Value.Literal)
			}
			lit.T = NewType(TBigFloat)
			lit.Raw = val
		}
	} else {
		panic(fmt.Sprintf("Unhandled literal %s", lit.Value.ID))
	}
	return lit
}

func (c *checker) checkIdent(id *Ident) Expr {
	sym := c.resolve(id.Name)
	if sym == nil {
		c.error(id.Name, "'%s' undefined", id.Name.Literal)
	}
	id.Sym = sym
	return id
}

func (c *checker) checkCallExpr(call *CallExpr) Expr {
	sym := c.scope.Lookup(call.Name.Literal())
	if sym == nil {
		c.error(call.Name.Name, "'%s' undefined", call.Name.Literal())
	} else if sym.ID != FuncSymbol && sym.ID != TypeSymbol {
		c.error(call.Name.Name, "'%s' is not a function", sym.Name.Literal)
	}

	c.checkIdent(call.Name)
	for i, arg := range call.Args {
		call.Args[i] = c.checkExpr(arg)
	}

	if sym != nil {
		if sym.ID == TypeSymbol {
			if len(call.Args) != 1 {
				c.error(call.Name.Name, "type conversion %s takes exactly 1 argument", sym.T.ID)
			} else if !castableType(call.Args[0].Type(), sym.T) {
				c.error(call.Name.Name, "type mismatch: %s cannot be converted to %s", call.Args[0].Type(), sym.T)
			} else if c.tryCastLiteral(call.Args[0], sym.T) {
				call.T = sym.T
			}
		} else if sym.ID == FuncSymbol {
			decl, _ := sym.Src.(*FuncDecl)
			if len(decl.Params) != len(call.Args) {
				c.error(call.Name.Name, "'%s' takes %d argument(s) but called with %d", sym.Name.Literal, len(decl.Params), len(call.Args))
			} else {
				for i, arg := range call.Args {
					paramType := decl.Params[i].Name.Type()

					if !c.tryCastLiteral(arg, paramType) {
						continue
					}

					argType := arg.Type()
					if argType.ID != paramType.ID {
						c.errorPos(arg.FirstPos(), "type mismatch: argument %d of function '%s' expects type %s but got %s",
							i, call.Name.Literal(), paramType.ID, argType.ID)
					}
				}
				call.T = decl.Return.Type()
			}
		}
	}

	if call.T == nil {
		call.T = TBuiltinUntyped
	}

	return call
}
