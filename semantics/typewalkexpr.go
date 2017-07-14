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

	if expr.Op.OneOf(token.And, token.Or) {
		if leftType.ID != TBool || rightType.ID != TBool {
			v.c.error(expr.Op.Pos, "type mismatch: arguments to operation '%s' are not of type %s (got %s and %s)",
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
					v.c.error(rightLit.Value.Pos, "Division by zero")
					expr.T = NewTypeFromID(TUntyped)
					return expr
				}
			}

			// Convert integer literals to floats

			if leftBigInt != nil && rightBigFloat != nil {
				leftBigFloat = big.NewFloat(0)
				leftBigFloat.SetInt(leftBigInt)
				leftLit.Raw = leftBigFloat
				leftLit.T = NewTypeFromID(TBigFloat)
				leftType = leftLit.T
				leftBigInt = nil
			}

			if rightBigInt != nil && leftBigFloat != nil {
				rightBigFloat = big.NewFloat(0)
				rightBigFloat.SetInt(rightBigInt)
				rightLit.Raw = rightBigFloat
				rightLit.T = NewTypeFromID(TBigFloat)
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
						leftLit.T = NewTypeFromID(TBool)
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
			v.c.error(expr.Op.Pos, "operation '%s' does not support type %s", expr.Op.ID, typeNotSupported)
		} else if leftType.ID != rightType.ID {
			v.c.error(expr.Op.Pos, "type mismatch: arguments to operation '%s' are not compatible (got %s and %s)",
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

	expr.T = NewTypeFromID(binType)
	return expr
}

func (v *typeVisitor) VisitUnaryExpr(expr *UnaryExpr) Expr {
	expr.X = VisitExpr(v, expr.X)
	expr.T = expr.X.Type()
	switch expr.Op.ID {
	case token.Sub:
		if !expr.T.IsNumericType() {
			v.c.error(expr.Op.Pos, "type mismatch: operation '%s' expects a numeric type but got %s", token.Sub, expr.T.ID)
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
			v.c.error(expr.Op.Pos, "type mismatch: operation '%s' expects type %s but got %s", token.Lnot, TBool, expr.T.ID)
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

func (v *typeVisitor) VisitLiteral(expr *Literal) Expr {
	if expr.Value.ID == token.False || expr.Value.ID == token.True {
		expr.T = NewTypeFromID(TBool)
	} else if expr.Value.ID == token.String {
		if expr.Raw == nil {
			expr.T = NewTypeFromID(TString)
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
			expr.T = NewTypeFromID(TBigInt)
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
			expr.T = NewTypeFromID(TBigFloat)
			expr.Raw = val
		}
	} else {
		panic(fmt.Sprintf("Unhandled literal %s", expr.Value.ID))
	}
	return expr
}

func (v *typeVisitor) VisitStructLiteral(expr *StructLiteral) Expr {
	v.VisitIdent(expr.Name)
	sym := expr.Name.Sym
	if sym == nil {
		expr.T = TBuiltinUntyped
		return expr
	} else if sym.ID != StructSymbol {
		v.c.error(expr.Name.Pos(), "'%s' is not a struct", sym.Name)
		expr.T = TBuiltinUntyped
		return expr
	}

	err := false
	decl, _ := expr.Name.Sym.Src.(*StructDecl)
	inits := make(map[string]Expr)

	for _, kv := range expr.Initializers {
		if existing, ok := inits[kv.Key.Literal]; ok {
			if existing != nil {
				v.c.error(kv.Key.Pos, "duplicate field key '%s'", kv.Key.Literal)
			}
			inits[kv.Key.Literal] = nil
			continue
		}

		fieldSym := decl.Scope.Lookup(kv.Key.Literal)
		if fieldSym == nil {
			v.c.error(kv.Key.Pos, "'%s' undefined struct field", kv.Key.Literal)
			inits[kv.Key.Literal] = nil
			continue
		}

		kv.Value = VisitExpr(v, kv.Value)

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

	expr.T = sym.T
	return createStructLiteral(decl, expr)
}

func createDefaultLiteral(sym *Symbol) Expr {
	t := sym.T
	if t.OneOf(TStruct) {
		decl, _ := sym.Src.(*StructDecl)
		lit := &StructLiteral{}
		lit.Name = &Ident{Name: token.Synthetic(token.Ident, t.Name)}
		lit.Name.Sym = sym
		lit.T = t
		return createStructLiteral(decl, lit)
	}
	return createDefaultBasicLiteral(t)
}

func createDefaultBasicLiteral(t *TType) *Literal {
	var lit *Literal
	if t.OneOf(TBool) {
		lit = &Literal{Value: token.Synthetic(token.False, token.False.String())}
		lit.T = NewTypeFromID(TBool)
	} else if t.OneOf(TString) {
		lit = &Literal{Value: token.Synthetic(token.String, "")}
		lit.T = NewTypeFromID(TString)
	} else if t.OneOf(TUInt64, TInt64, TUInt32, TInt32, TUInt16, TInt16, TUInt8, TInt8) {
		lit = &Literal{Value: token.Synthetic(token.Integer, "0")}
		lit.T = NewTypeFromID(t.ID)
	} else if t.OneOf(TFloat64, TFloat32) {
		lit = &Literal{Value: token.Synthetic(token.Float, "0")}
		lit.T = NewTypeFromID(t.ID)
	} else {
		panic(fmt.Sprintf("Unhandled init value for type %s", t.ID))
	}
	return lit
}

func createStructLiteral(decl *StructDecl, lit *StructLiteral) *StructLiteral {
	for _, f := range decl.Fields {
		key := f.Name.Literal
		found := false
		for _, init := range lit.Initializers {
			if init.Key.Literal == key {
				found = true
				break
			}
		}
		if found {
			continue
		}
		kv := &KeyValue{}
		kv.Key = token.Synthetic(token.Ident, key)
		kv.Value = createDefaultLiteral(f.Sym)
		lit.Initializers = append(lit.Initializers, kv)
	}
	return lit
}

func (v *typeVisitor) VisitIdent(expr *Ident) Expr {
	expr.Sym = v.c.lookup(expr.Name)
	if expr.Sym == nil {
		v.c.error(expr.Name.Pos, "'%s' undefined", expr.Name.Literal)
	}
	return expr
}

func (v *typeVisitor) VisitFuncCall(expr *FuncCall) Expr {
	v.VisitIdent(expr.Name)
	sym := expr.Name.Sym
	if sym == nil {
		expr.T = TBuiltinUntyped
		return expr
	} else if sym.ID != FuncSymbol && !sym.Castable() {
		v.c.error(expr.Name.Pos(), "'%s' is not a function", sym.Name)
		expr.T = TBuiltinUntyped
		return expr
	}

	for i, arg := range expr.Args {
		expr.Args[i] = VisitExpr(v, arg)
	}

	if sym.ID == FuncSymbol {
		decl, _ := sym.Src.(*FuncDecl)
		if len(decl.Params) != len(expr.Args) {
			v.c.error(expr.Name.Pos(), "'%s' takes %d argument(s) but called with %d", sym.Name, len(decl.Params), len(expr.Args))
		} else {
			for i, arg := range expr.Args {
				paramType := decl.Params[i].Sym.T

				if !v.c.tryCastLiteral(arg, paramType) {
					continue
				}

				argType := arg.Type()
				if !argType.IsEqual(paramType) {
					v.c.error(arg.FirstPos(), "type mismatch: argument %d of function '%s' expects type %s but got %s",
						i, expr.Name.Literal(), paramType.ID, argType.ID)
				}
			}
			expr.T = decl.TReturn.Type()
		}
	} else {
		if len(expr.Args) != 1 {
			v.c.error(expr.Name.Pos(), "type conversion %s takes exactly 1 argument", sym.T.ID)
		} else if !compatibleTypes(expr.Args[0].Type(), sym.T) {
			v.c.error(expr.Name.Pos(), "type mismatch: %s cannot be converted to %s", expr.Args[0].Type(), sym.T)
		} else if v.c.tryCastLiteral(expr.Args[0], sym.T) {
			expr.T = sym.T
		}
	}

	if expr.T == nil {
		expr.T = TBuiltinUntyped
	}

	return expr
}

func (v *typeVisitor) VisitDotExpr(expr *DotExpr) Expr {
	sym := v.c.lookup(expr.Name.Name)

	if sym == nil {
		return expr
	}

	if sym.ID != ModuleSymbol {
		v.c.error(expr.Name.Pos(), "'%s' is not a module", expr.Name.Literal())
		expr.T = TBuiltinUntyped
		return expr
	}

	mod, _ := sym.Src.(*Module)
	defer setScope(setScope(v.c, mod.External))

	expr.X = VisitExpr(v, expr.X)
	expr.T = expr.X.Type()

	return expr
}
