package semantics

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/jhnl/interpreter/token"
)

// TODO: Evaluate constant boolean expressions

func (v *typeVisitor) VisitBinaryExpr(expr *BinaryExpr) Expr {
	expr.Left = VisitExpr(v, expr.Left)
	expr.Right = VisitExpr(v, expr.Right)

	leftType := expr.Left.Type()
	rightType := expr.Right.Type()

	binType := TUntyped
	boolOp := expr.Op.OneOf(token.Eq, token.Neq, token.Gt, token.GtEq, token.Lt, token.LtEq)
	arithOp := expr.Op.OneOf(token.Add, token.Sub, token.Mul, token.Div, token.Mod)
	typeNotSupported := TUntyped

	if expr.Op.OneOf(token.Land, token.Lor) {
		if leftType.ID() != TBool || rightType.ID() != TBool {
			v.c.error(expr.Op.Pos, "type mismatch: expression %s have types %s and %s (expected %s)",
				PrintExpr(expr), leftType, rightType, TBool)
		} else {
			binType = TBool
		}
	} else if boolOp || arithOp {
		leftLit, _ := expr.Left.(*BasicLit)
		rightLit, _ := expr.Right.(*BasicLit)

		if IsNumericType(leftType) && IsNumericType(rightType) {
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
					v.c.error(rightLit.Value.Pos, "Division by zero")
					expr.T = NewBasicType(TUntyped)
					return expr
				}
			}

			// Convert integer literals to floats

			if leftBigInt != nil && rightBigFloat != nil {
				leftBigFloat = big.NewFloat(0)
				leftBigFloat.SetInt(leftBigInt)
				leftLit.Raw = leftBigFloat
				leftLit.T = NewBasicType(TBigFloat)
				leftType = leftLit.T
				leftBigInt = nil
			}

			if rightBigInt != nil && leftBigFloat != nil {
				rightBigFloat = big.NewFloat(0)
				rightBigFloat.SetInt(rightBigInt)
				rightLit.Raw = rightBigFloat
				rightLit.T = NewBasicType(TBigFloat)
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
							typeNotSupported = leftType.ID()
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
						leftLit.T = NewBasicType(TBool)
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
		} else if leftType.ID() == TBool && rightType.ID() == TBool {
			if arithOp || expr.Op.OneOf(token.Gt, token.GtEq, token.Lt, token.LtEq) {
				typeNotSupported = TBool
			}
		} else if leftType.ID() == TString && rightType.ID() == TString {
			typeNotSupported = TString
		}

		if typeNotSupported != TUntyped {
			v.c.error(expr.Op.Pos, "type mismatch: expression %s with type %s is not supported", PrintExpr(expr), typeNotSupported)
		} else if !leftType.IsEqual(rightType) {
			v.c.error(expr.Op.Pos, "type mismatch: expression %s have types %s and %s",
				PrintExpr(expr), leftType, rightType)
		} else {
			if boolOp {
				binType = TBool
			} else {
				binType = leftType.ID()
			}
		}
	} else {
		panic(fmt.Sprintf("Unhandled binop %s", expr.Op.ID))
	}

	expr.T = NewBasicType(binType)
	return expr
}

func (v *typeVisitor) VisitUnaryExpr(expr *UnaryExpr) Expr {
	expr.X = VisitExpr(v, expr.X)
	expr.T = expr.X.Type()
	switch expr.Op.ID {
	case token.Sub:
		if !IsNumericType(expr.T) {
			v.c.error(expr.Op.Pos, "type mismatch: expression %s has type %s (expected integer or float)", PrintExpr(expr), expr.T)
		} else if lit, ok := expr.X.(*BasicLit); ok {
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
		if expr.T.ID() != TBool {
			v.c.error(expr.Op.Pos, "type mismatch: expression %s has type %s (expected %s)", PrintExpr(expr), expr.T, TBuiltinBool)
		}
	default:
		panic(fmt.Sprintf("Unhandled unary op %s", expr.Op.ID))
	}
	return expr
}

func unescapeStringLiteral(lit string) string {
	// TODO:
	// - Handle more escape sequences
	// - Improve rune handling

	escaped := []rune(lit)
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

func removeUnderscores(lit string) string {
	res := strings.Replace(lit, "_", "", -1)
	return res
}

func (v *typeVisitor) VisitBasicLit(expr *BasicLit) Expr {
	if expr.Value.ID == token.False || expr.Value.ID == token.True {
		expr.T = TBuiltinBool
	} else if expr.Value.ID == token.String {
		if expr.Raw == nil {
			expr.T = TBuiltinString
			expr.Raw = unescapeStringLiteral(expr.Value.Literal)
		}
	} else if expr.Value.ID == token.Integer {
		if expr.Raw == nil {
			val := big.NewInt(0)
			normalized := removeUnderscores(expr.Value.Literal)
			_, ok := val.SetString(normalized, 0)
			if !ok {
				v.c.error(expr.Value.Pos, "unable to interpret integer literal %s", normalized)
			}
			expr.T = NewBasicType(TBigInt)
			expr.Raw = val
		}
	} else if expr.Value.ID == token.Float {
		if expr.Raw == nil {
			val := big.NewFloat(0)
			normalized := removeUnderscores(expr.Value.Literal)
			_, ok := val.SetString(normalized)
			if !ok {
				v.c.error(expr.Value.Pos, "unable to interpret float literal %s", normalized)
			}
			expr.T = NewBasicType(TBigFloat)
			expr.Raw = val
		}
	} else {
		panic(fmt.Sprintf("Unhandled literal %s", expr.Value.ID))
	}
	return expr
}

func (v *typeVisitor) VisitStructLit(expr *StructLit) Expr {
	prevMode := v.identMode
	v.identMode = identModeType
	expr.Name = VisitExpr(v, expr.Name)
	v.identMode = prevMode

	t := expr.Name.Type()
	if IsUntyped(t) {
		expr.T = TBuiltinUntyped
		return expr
	} else if t.ID() != TStruct {
		v.c.error(expr.Name.FirstPos(), "'%s' is not a struct", PrintExpr(expr.Name))
		expr.T = TBuiltinUntyped
		return expr
	}

	err := false
	inits := make(map[string]Expr)
	structt, _ := t.(*StructType)

	for _, kv := range expr.Initializers {
		if existing, ok := inits[kv.Key.Literal]; ok {
			if existing != nil {
				v.c.error(kv.Key.Pos, "duplicate field key '%s'", kv.Key.Literal)
			}
			inits[kv.Key.Literal] = nil
			continue
		}

		fieldSym := structt.Scope.Lookup(kv.Key.Literal)
		if fieldSym == nil {
			v.c.error(kv.Key.Pos, "'%s' undefined struct field", kv.Key.Literal)
			inits[kv.Key.Literal] = nil
			continue
		}

		kv.Value = VisitExpr(v, kv.Value)

		if IsUntyped(fieldSym.T) {
			inits[kv.Key.Literal] = nil
			continue
		}

		if !v.c.tryCastLiteral(kv.Value, fieldSym.T) {
			inits[kv.Key.Literal] = nil
			continue
		}

		if !fieldSym.T.IsEqual(kv.Value.Type()) {
			v.c.error(kv.Key.Pos, "type mismatch: field '%s' expects type %s but got %s",
				kv.Key.Literal, fieldSym.T, kv.Value.Type())
			inits[kv.Key.Literal] = nil
			continue
		}
		inits[kv.Key.Literal] = kv.Value
	}

	if err {
		expr.T = TBuiltinUntyped
		return expr
	}

	expr.T = structt
	return createStructLit(structt, expr)
}

func createDefaultLiteral(t Type, name Expr) Expr {
	if t.ID() == TStruct {
		structt, _ := t.(*StructType)
		lit := &StructLit{}
		lit.Name = CopyExpr(name, false)
		return createStructLit(structt, lit)
	}
	return createDefaultBasicLit(t)
}

func createDefaultBasicLit(t Type) *BasicLit {
	var lit *BasicLit
	if IsTypeID(t, TBool) {
		lit = &BasicLit{Value: token.Synthetic(token.False, token.False.String())}
		lit.T = NewBasicType(TBool)
	} else if IsTypeID(t, TString) {
		lit = &BasicLit{Value: token.Synthetic(token.String, "")}
		lit.Raw = ""
		lit.T = NewBasicType(TString)
	} else if IsTypeID(t, TUInt64, TInt64, TUInt32, TInt32, TUInt16, TInt16, TUInt8, TInt8) {
		lit = &BasicLit{Value: token.Synthetic(token.Integer, "0")}
		lit.Raw = BigIntZero
		lit.T = NewBasicType(t.ID())
	} else if IsTypeID(t, TFloat64, TFloat32) {
		lit = &BasicLit{Value: token.Synthetic(token.Float, "0")}
		lit.Raw = BigFloatZero
		lit.T = NewBasicType(t.ID())
	} else {
		panic(fmt.Sprintf("Unhandled init value for type %s", t.ID()))
	}
	return lit
}

func createStructLit(structt *StructType, lit *StructLit) *StructLit {
	var initializers []*KeyValue
	for _, f := range structt.Fields {
		key := f.Name()
		found := false
		for _, init := range lit.Initializers {
			if init.Key.Literal == key {
				initializers = append(initializers, init)
				found = true
				break
			}
		}
		if found {
			continue
		}
		kv := &KeyValue{}
		kv.Key = token.Synthetic(token.Ident, key)

		decl := f.Sym.Src.(*ValDecl)
		kv.Value = createDefaultLiteral(f.T, decl.Type)
		initializers = append(initializers, kv)
	}
	lit.Initializers = initializers
	return lit
}

func (v *typeVisitor) VisitIdent(expr *Ident) Expr {
	sym := v.c.lookup(expr.Name.Literal)
	if sym == nil || sym.Untyped() {
		v.c.error(expr.Pos(), "'%s' undefined", expr.Name.Literal)
		expr.T = TBuiltinUntyped
	} else if v.identMode != identModeType && v.identMode != identModeFunc && sym.ID == TypeSymbol {
		v.c.error(expr.Pos(), "type %s cannot be used in an expression", sym.T)
		expr.T = TBuiltinUntyped
	} else if v.identMode == identModeNone && sym.ID == FuncSymbol {
		v.c.error(expr.Pos(), "invalid function call to '%s' (missing argument list)", expr.Literal())
		expr.T = TBuiltinUntyped
	} else {
		expr.setSymbol(sym)
	}
	return expr
}

func (v *typeVisitor) VisitDotIdent(expr *DotIdent) Expr {
	expr.X = VisitExpr(v, expr.X)
	t := expr.X.Type()
	if IsUntyped(t) {
		expr.T = TBuiltinUntyped
		return expr
	}

	if id := ExprToIdent(expr.X); id != nil {
		sym := id.Sym
		if sym != nil && sym.ID == TypeSymbol {
			v.c.error(id.Pos(), "static field access is not supported")
			expr.T = TBuiltinUntyped
		}
	}

	if t.ID() == TModule {
		mod, _ := t.(*ModuleType)
		defer setScope(setScope(v.c, mod.Scope))
	} else if t.ID() == TStruct {
		structt, _ := t.(*StructType)
		defer setScope(setScope(v.c, structt.Scope))
	} else {
		v.c.error(expr.X.FirstPos(), "type %s does not support field access", t)
		expr.T = TBuiltinUntyped
		return expr
	}

	v.VisitIdent(expr.Name)
	expr.T = expr.Name.Type()

	return expr
}

func (v *typeVisitor) VisitFuncCall(expr *FuncCall) Expr {
	prevMode := v.identMode
	v.identMode = identModeFunc
	expr.X = VisitExpr(v, expr.X)
	v.identMode = prevMode

	t := expr.X.Type()
	if IsUntyped(t) {
		expr.T = TBuiltinUntyped
		return expr
	}

	typeCast := false

	// Check if type cast
	switch id := expr.X.(type) {
	case *Ident:
		if id.Sym.ID == TypeSymbol && id.Sym.Castable() {
			typeCast = true
		}
	}

	if !typeCast && t.ID() != TFunc {
		v.c.error(expr.X.FirstPos(), "'%s' is not a function", PrintExpr(expr.X))
		expr.T = TBuiltinUntyped
		return expr
	}

	for i, arg := range expr.Args {
		expr.Args[i] = VisitExpr(v, arg)
	}

	if typeCast {
		if len(expr.Args) != 1 {
			v.c.error(expr.X.FirstPos(), "type conversion takes exactly 1 argument")
		} else if !compatibleTypes(expr.Args[0].Type(), t) {
			v.c.error(expr.X.FirstPos(), "type mismatch: %s cannot be converted to %s", expr.Args[0].Type(), t)
		} else if v.c.tryCastLiteral(expr.Args[0], t) {
			expr.T = t
		}
	} else {
		funcType, _ := t.(*FuncType)
		if len(funcType.Params) != len(expr.Args) {
			v.c.error(expr.X.FirstPos(), "'%s' takes %d argument(s) but called with %d", PrintExpr(expr.X), len(funcType.Params), len(expr.Args))
		} else {
			for i, arg := range expr.Args {
				paramType := funcType.Params[i].T

				if !v.c.tryCastLiteral(arg, paramType) {
					continue
				}

				argType := arg.Type()
				if !argType.IsEqual(paramType) {
					v.c.error(arg.FirstPos(), "type mismatch: argument %d of function '%s' expects type %s but got %s",
						i, PrintExpr(expr.X), paramType, argType)
				}
			}
			expr.T = funcType.Return
		}
	}

	if expr.T == nil {
		expr.T = TBuiltinUntyped
	}

	return expr
}
