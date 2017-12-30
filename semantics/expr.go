package semantics

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

func (v *typeChecker) makeTypedExpr(expr ir.Expr, t ir.Type) ir.Expr {
	expr = ir.VisitExpr(v, expr)
	if ir.IsUntyped(expr.Type()) || (t != nil && ir.IsUntyped(t)) {
		return expr
	}

	if !ir.IsActualType(expr.Type()) {
		if t != nil {
			v.tryMakeTypedLit(expr, t)
		} else {
			v.tryMakeDefaultTypedLit(expr)
		}
	}

	if !checkCompleteType(expr.Type()) {
		v.c.error(expr.FirstPos(), "invalid expression '%s' (incomplete type %s)", PrintExpr(expr), expr.Type())
		switch t2 := expr.(type) {
		case *ir.SliceExpr:
			t2.T = ir.TBuiltinUntyped
		}
		return expr
	}

	if t != nil {
		t2 := expr.Type()

		if t2.ImplicitCastOK(t) {
			cast := &ir.CastExpr{}
			cast.X = expr
			cast.T = t
			return cast
		}
	}

	return expr
}

func (v *typeChecker) tryMakeTypedLit(expr ir.Expr, target ir.Type) bool {
	switch lit := expr.(type) {
	case *ir.BasicLit:
		if ir.IsTypeID(lit.T, ir.TBigInt, ir.TBigFloat) && ir.IsNumericType(target) {
			castResult := typeCastNumericLit(lit, target)

			if castResult == numericCastOK {
				return true
			}

			if castResult == numericCastOverflows {
				v.c.error(lit.Value.Pos, "constant expression %s overflows %s", lit.Value.Literal, target)
			} else if castResult == numericCastTruncated {
				v.c.error(lit.Value.Pos, "type mismatch: constant float expression %s not compatible with %s", lit.Value.Literal, target)
			} else {
				panic(fmt.Sprintf("Unhandled numeric cast result %d", castResult))
			}

			return false
		} else if ir.IsTypeID(expr.Type(), ir.TPointer) && ir.IsTypeID(target, ir.TSlice, ir.TPointer, ir.TFunc) {
			common.Assert(lit.Value.ID == token.Null, "pointer literal should be null")
			tpointer := expr.Type().(*ir.PointerType)
			if ir.IsUntyped(tpointer.Underlying) {
				switch ttarget := target.(type) {
				case *ir.SliceType:
					lit.T = ir.NewSliceType(ttarget.Elem, true, true)
				case *ir.PointerType:
					lit.T = ir.NewPointerType(ttarget.Underlying, true)
				case *ir.FuncType:
					lit.T = ir.NewFuncType(ttarget.Params, ttarget.Return)
				default:
					panic(fmt.Sprintf("Unhandled target type %T", ttarget))
				}
			}
		}
	case *ir.ArrayLit:
		if ir.IsTypeID(expr.Type(), ir.TArray) && ir.IsTypeID(target, ir.TArray) {
			tarray := expr.Type().(*ir.ArrayType)
			targetTarray := target.(*ir.ArrayType)
			err := false
			for _, init := range lit.Initializers {
				if !v.tryMakeTypedLit(init, targetTarray.Elem) {
					err = true
					break
				}
			}
			if !err && len(lit.Initializers) > 0 {
				tarray.Elem = lit.Initializers[0].Type()
			}
		}
	}

	return true
}

func (v *typeChecker) tryMakeDefaultTypedLit(expr ir.Expr) bool {
	t := expr.Type()
	if t.ID() == ir.TBigInt {
		return v.tryMakeTypedLit(expr, ir.TBuiltinInt32)
	} else if t.ID() == ir.TBigFloat {
		return v.tryMakeTypedLit(expr, ir.TBuiltinFloat64)
	} else if t.ID() == ir.TArray {
		if lit, ok := expr.(*ir.ArrayLit); ok {
			err := false
			for _, init := range lit.Initializers {
				if !v.tryMakeDefaultTypedLit(init) {
					err = true
					break
				}
			}
			size := len(lit.Initializers)
			if !err && size > 0 {
				telem := lit.Initializers[0].Type()
				lit.T = ir.NewArrayType(telem, size)
			}
		}
	}
	return true
}

func tryDeref(expr ir.Expr) ir.Expr {
	var tres ir.Type
	switch t1 := expr.Type().(type) {
	case *ir.PointerType:
		switch t2 := t1.Underlying.(type) {
		case *ir.StructType:
			tres = t2
		case *ir.ArrayType:
			tres = t2
		case *ir.SliceType:
			tres = t2
		}
	}
	if tres != nil {
		star := token.Synthetic(token.Mul, token.Mul.String())
		starX := &ir.UnaryExpr{Op: star, X: expr}
		starX.T = tres
		return starX
	}
	return expr
}

func (v *typeChecker) VisitPointerTypeExpr(expr *ir.PointerTypeExpr) ir.Expr {
	expr.X = ir.VisitExpr(v, expr.X)

	ro := expr.Decl.Is(token.Val)
	typ := expr.X.Type()

	if ir.IsUntyped(typ) {
		expr.T = ir.TBuiltinUntyped
	} else if tslice, ok := typ.(*ir.SliceType); ok {
		if !tslice.Ptr {
			tslice.Ptr = true
			tslice.ReadOnly = ro
			expr.T = tslice
		} else {
			expr.T = ir.NewPointerType(typ, ro)
		}
	} else {
		expr.T = ir.NewPointerType(typ, ro)
	}
	return expr
}

func (v *typeChecker) VisitArrayTypeExpr(expr *ir.ArrayTypeExpr) ir.Expr {
	size := 0

	if expr.Size != nil {
		expr.Size = v.makeTypedExpr(expr.Size, ir.TBuiltinInt32)
		sizeType := expr.Size.Type()

		err := false

		if sizeType.ID() != ir.TInt32 {
			v.c.error(expr.Size.FirstPos(), "array size must be of type %s (got %s)", ir.TInt32, sizeType)
			err = true
		} else if lit, ok := expr.Size.(*ir.BasicLit); !ok {
			v.c.error(expr.Size.FirstPos(), "array size '%s' is not a constant expression", PrintExpr(expr.Size))
			err = true
		} else if lit.NegatigeInteger() {
			v.c.error(expr.Size.FirstPos(), "array size cannot be negative")
			err = true
		} else if lit.Zero() {
			v.c.error(expr.Size.FirstPos(), "array size cannot be zero")
			err = true
		} else {
			size = int(lit.AsU64())
		}

		if err {
			expr.T = ir.TBuiltinUntyped
			return expr
		}
	}

	expr.X = ir.VisitExpr(v, expr.X)
	if expr.X.Type().Equals(ir.TBuiltinUntyped) {
		expr.T = ir.TBuiltinUntyped
	} else if size != 0 {
		expr.T = ir.NewArrayType(expr.X.Type(), size)
	} else {
		expr.T = ir.NewSliceType(expr.X.Type(), true, false)
	}

	return expr
}

func (v *typeChecker) VisitFuncTypeExpr(expr *ir.FuncTypeExpr) ir.Expr {
	var tparams []ir.Type
	untyped := false
	for i, param := range expr.Params {
		expr.Params[i] = ir.VisitExpr(v, param)
		tparam := expr.Params[i].Type()
		tparams = append(tparams, tparam)
		if ir.IsUntyped(tparam) {
			untyped = true
		}
	}

	expr.Return = ir.VisitExpr(v, expr.Return)
	if ir.IsUntyped(expr.Return.Type()) {
		untyped = true
	}

	if !untyped {
		expr.T = ir.NewFuncType(tparams, expr.Return.Type())
	} else {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

// TODO: Evaluate constant boolean expressions

func (v *typeChecker) VisitBinaryExpr(expr *ir.BinaryExpr) ir.Expr {
	expr.Left = ir.VisitExpr(v, expr.Left)
	expr.Right = ir.VisitExpr(v, expr.Right)

	leftType := expr.Left.Type()
	rightType := expr.Right.Type()

	binType := ir.TUntyped
	boolOp := expr.Op.OneOf(token.Eq, token.Neq, token.Gt, token.GtEq, token.Lt, token.LtEq)
	arithOp := expr.Op.OneOf(token.Add, token.Sub, token.Mul, token.Div, token.Mod)
	typeNotSupported := ir.TBuiltinUntyped

	if expr.Op.OneOf(token.Land, token.Lor) {
		if leftType.ID() != ir.TBool || rightType.ID() != ir.TBool {
			v.c.error(expr.Op.Pos, "type mismatch: expression %s have types %s and %s (expected %s)",
				PrintExpr(expr), leftType, rightType, ir.TBool)
		} else {
			binType = ir.TBool
		}
	} else if boolOp || arithOp {
		leftLit, _ := expr.Left.(*ir.BasicLit)
		rightLit, _ := expr.Right.(*ir.BasicLit)

		if ir.IsNumericType(leftType) && ir.IsNumericType(rightType) {
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
				if (rightBigInt != nil && rightBigInt.Cmp(ir.BigIntZero) == 0) ||
					(rightBigFloat != nil && rightBigFloat.Cmp(ir.BigFloatZero) == 0) {
					v.c.error(rightLit.Value.Pos, "division by zero")
					expr.T = ir.NewBasicType(ir.TUntyped)
					return expr
				}
			}

			// Convert integer literals to floats

			if leftBigInt != nil && rightBigFloat != nil {
				leftBigFloat = big.NewFloat(0)
				leftBigFloat.SetInt(leftBigInt)
				leftLit.Raw = leftBigFloat
				leftLit.T = ir.NewBasicType(ir.TBigFloat)
				leftType = leftLit.T
				leftBigInt = nil
			}

			if rightBigInt != nil && leftBigFloat != nil {
				rightBigFloat = big.NewFloat(0)
				rightBigFloat.SetInt(rightBigInt)
				rightLit.Raw = rightBigFloat
				rightLit.T = ir.NewBasicType(ir.TBigFloat)
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
							typeNotSupported = leftType
						default:
							panic(fmt.Sprintf("Unhandled binop %s", expr.Op.ID))
						}
					}
				}

				if ir.IsTypeID(typeNotSupported, ir.TUntyped) {
					if boolOp {
						if boolRes {
							leftLit.Value.ID = token.True
						} else {
							leftLit.Value.ID = token.False
						}
						leftLit.T = ir.NewBasicType(ir.TBool)
						leftLit.Raw = nil
					}

					leftLit.Value.Literal = "(" + leftLit.Value.Literal + " " + expr.Op.Literal + " " + rightLit.Value.Literal + ")"
					leftLit.Rewrite++
					return leftLit
				}
			} else if leftBigInt != nil && rightBigInt == nil {
				typeCastNumericLit(leftLit, rightType)
				leftType = leftLit.T
			} else if leftBigInt == nil && rightBigInt != nil {
				typeCastNumericLit(rightLit, leftType)
				rightType = rightLit.T
			} else if leftBigFloat != nil && rightBigFloat == nil {
				typeCastNumericLit(leftLit, rightType)
				leftType = leftLit.T
			} else if leftBigFloat == nil && rightBigFloat != nil {
				typeCastNumericLit(rightLit, leftType)
				rightType = rightLit.T
			}
		} else if leftType.ID() == ir.TPointer && rightType.ID() == ir.TPointer {
			if arithOp {
				typeNotSupported = leftType
			}

			leftPtr := leftType.(*ir.PointerType)
			rightPtr := rightType.(*ir.PointerType)
			leftNull := leftLit != nil && leftLit.Value.Is(token.Null)
			rightNull := rightLit != nil && rightLit.Value.Is(token.Null)

			if leftNull && rightNull {
				leftLit.T = ir.NewPointerType(ir.NewBasicType(ir.TUInt8), false)
				leftType = leftLit.T
				rightLit.T = ir.NewPointerType(ir.NewBasicType(ir.TUInt8), false)
				rightType = rightLit.T
			} else if leftNull {
				leftLit.T = ir.NewPointerType(rightPtr.Underlying, false)
				leftType = leftLit.T
			} else if rightNull {
				rightLit.T = ir.NewPointerType(leftPtr.Underlying, false)
				rightType = rightLit.T
			}
		} else if leftType.ID() == ir.TBool && rightType.ID() == ir.TBool {
			if arithOp || expr.Op.OneOf(token.Gt, token.GtEq, token.Lt, token.LtEq) {
				typeNotSupported = leftType
			}
		} else if leftType.Equals(rightType) {
			typeNotSupported = leftType
		}

		if !checkTypes(v.c, leftType, rightType) {
			v.c.error(expr.Op.Pos, "type mismatch: '%s' have different types (%s and %s)",
				PrintExpr(expr), leftType, rightType)
		} else if !ir.IsUntyped(leftType) && !ir.IsUntyped(rightType) {
			if !ir.IsTypeID(typeNotSupported, ir.TUntyped) {
				v.c.error(expr.Op.Pos, "type mismatch: cannot perform '%s' with type %s", PrintExpr(expr), typeNotSupported)
			} else {
				if boolOp {
					binType = ir.TBool
				} else {
					binType = leftType.ID()
				}
			}
		}
	} else {
		panic(fmt.Sprintf("Unhandled binop %s", expr.Op.ID))
	}

	expr.T = ir.NewBasicType(binType)
	return expr
}

func (v *typeChecker) VisitUnaryExpr(expr *ir.UnaryExpr) ir.Expr {
	expr.X = ir.VisitExpr(v, expr.X)
	expr.T = expr.X.Type()
	switch expr.Op.ID {
	case token.Sub:
		if !ir.IsNumericType(expr.T) {
			v.c.error(expr.Op.Pos, "type mismatch: expression '%s' has type %s (expected integer or float)", PrintExpr(expr), expr.T)
		} else if lit, ok := expr.X.(*ir.BasicLit); ok {
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
		if expr.T.ID() != ir.TBool {
			v.c.error(expr.Op.Pos, "type mismatch: expression '%s' has type %s (expected %s)", PrintExpr(expr), expr.T, ir.TBuiltinBool)
		}
	case token.Mul:
		lvalue := false

		if deref, ok := expr.X.(*ir.UnaryExpr); ok {
			if deref.Op.ID == token.And {
				// Inverse
				lvalue = deref.X.Lvalue()
			}
		} else {
			lvalue = expr.X.Lvalue()
		}

		if !lvalue {
			v.c.error(expr.X.FirstPos(), "cannot dereference '%s' (not an lvalue)", PrintExpr(expr.X))
		} else {
			switch t := expr.T.(type) {
			case *ir.PointerType:
				expr.T = t.Underlying
			case *ir.SliceType:
				tslice := ir.NewSliceType(t.Elem, t.ReadOnly, false)
				v.c.error(expr.X.FirstPos(), "cannot dereference '%s' (type %s is incomplete)", PrintExpr(expr.X), tslice)
			default:
				v.c.error(expr.X.FirstPos(), "cannot dereference '%s' (type %s is not a pointer)", PrintExpr(expr.X), expr.T)
			}
		}

		if expr.T == nil {
			expr.T = ir.TBuiltinUntyped
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

func (v *typeChecker) VisitBasicLit(expr *ir.BasicLit) ir.Expr {
	if expr.Value.ID == token.False || expr.Value.ID == token.True {
		expr.T = ir.TBuiltinBool
	} else if expr.Value.ID == token.String {
		if expr.Raw == nil {
			raw := unescapeStringLiteral(expr.Value.Literal)
			if expr.Prefix == nil {
				expr.T = ir.NewSliceType(ir.TBuiltinInt8, true, true)
				expr.Raw = raw
			} else if expr.Prefix.Literal() == "c" {
				expr.T = ir.NewPointerType(ir.TBuiltinInt8, true)
				expr.Raw = raw
			} else {
				v.c.error(expr.Prefix.FirstPos(), "invalid string prefix '%s'", PrintExpr(expr.Prefix))
				expr.T = ir.TBuiltinUntyped
			}
		}
	} else if expr.Value.ID == token.Integer {
		if expr.Raw == nil {
			base := ir.TBigInt
			target := ir.TBigInt

			if expr.Suffix != nil {
				switch expr.Suffix.Literal() {
				case ir.TFloat64.String():
					base = ir.TBigFloat
					target = ir.TFloat64
				case ir.TFloat32.String():
					base = ir.TBigFloat
					target = ir.TFloat32
				case ir.TUInt64.String():
					target = ir.TUInt64
				case ir.TUInt32.String():
					target = ir.TUInt32
				case ir.TUInt16.String():
					target = ir.TUInt16
				case ir.TUInt8.String():
					target = ir.TUInt8
				case ir.TInt64.String():
					target = ir.TInt64
				case ir.TInt32.String():
					target = ir.TInt32
				case ir.TInt16.String():
					target = ir.TInt16
				case ir.TInt8.String():
					target = ir.TInt8
				default:
					v.c.error(expr.Suffix.FirstPos(), "invalid int suffix '%s'", PrintExpr(expr.Suffix))
					base = ir.TUntyped
				}
			}

			if base != ir.TUntyped {
				normalized := removeUnderscores(expr.Value.Literal)

				if base == ir.TBigInt {
					val := big.NewInt(0)
					_, ok := val.SetString(normalized, 0)
					if ok {
						expr.Raw = val
					}
				} else if base == ir.TBigFloat {
					val := big.NewFloat(0)
					_, ok := val.SetString(normalized)
					if ok {
						expr.Raw = val
					}
				}

				if expr.Raw != nil {
					expr.T = ir.NewBasicType(base)
					if target != ir.TBigInt && target != ir.TBigFloat {
						v.tryMakeTypedLit(expr, ir.NewBasicType(target))
					}
				} else {
					v.c.error(expr.Value.Pos, "unable to interpret int literal '%s'", normalized)
				}
			}

			if expr.T == nil {
				expr.T = ir.TBuiltinUntyped
			}
		}
	} else if expr.Value.ID == token.Float {
		if expr.Raw == nil {
			base := ir.TBigFloat
			target := ir.TBigFloat

			if expr.Suffix != nil {
				switch expr.Suffix.Literal() {
				case ir.TFloat64.String():
					target = ir.TFloat64
				case ir.TFloat32.String():
					target = ir.TFloat32
				default:
					v.c.error(expr.Suffix.FirstPos(), "invalid float suffix '%s'", PrintExpr(expr.Suffix))
					base = ir.TUntyped
				}
			}

			if base != ir.TUntyped {
				val := big.NewFloat(0)
				normalized := removeUnderscores(expr.Value.Literal)
				_, ok := val.SetString(normalized)
				if ok {
					expr.T = ir.NewBasicType(base)
					expr.Raw = val

					if target != ir.TBigFloat {
						v.tryMakeTypedLit(expr, ir.NewBasicType(target))
					}
				} else {
					v.c.error(expr.Value.Pos, "unable to interpret float literal '%s'", normalized)
				}
			}

			if expr.T == nil {
				expr.T = ir.TBuiltinUntyped
			}
		}
	} else if expr.Value.ID == token.Null {
		expr.T = ir.NewPointerType(ir.TBuiltinUntyped, false)
	} else {
		panic(fmt.Sprintf("Unhandled literal %s", expr.Value.ID))
	}

	return expr
}

func (v *typeChecker) VisitStructLit(expr *ir.StructLit) ir.Expr {
	prevMode := v.exprMode
	v.exprMode = exprModeType
	expr.Name = ir.VisitExpr(v, expr.Name)
	v.exprMode = prevMode

	t := expr.Name.Type()
	if ir.IsUntyped(t) {
		expr.T = ir.TBuiltinUntyped
		return expr
	} else if t.ID() != ir.TStruct {
		v.c.error(expr.Name.FirstPos(), "'%s' is not a struct", PrintExpr(expr.Name))
		expr.T = ir.TBuiltinUntyped
		return expr
	}

	err := false
	inits := make(map[string]ir.Expr)
	structt, _ := t.(*ir.StructType)

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

		kv.Value = v.makeTypedExpr(kv.Value, fieldSym.T)

		if ir.IsUntyped(fieldSym.T) {
			inits[kv.Key.Literal] = nil
			continue
		}

		if !checkTypes(v.c, fieldSym.T, kv.Value.Type()) {
			v.c.error(kv.Key.Pos, "type mismatch: field '%s' expects type %s but got %s",
				kv.Key.Literal, fieldSym.T, kv.Value.Type())
			inits[kv.Key.Literal] = nil
			continue
		}
		inits[kv.Key.Literal] = kv.Value
	}

	if err {
		expr.T = ir.TBuiltinUntyped
		return expr
	}

	expr.T = structt
	return createStructLit(structt, expr)
}

func (v *typeChecker) VisitArrayLit(expr *ir.ArrayLit) ir.Expr {
	t := ir.TBuiltinUntyped
	backup := ir.TBuiltinUntyped

	for i, init := range expr.Initializers {
		init = ir.VisitExpr(v, init)
		expr.Initializers[i] = init

		if t == ir.TBuiltinUntyped && ir.IsActualType(init.Type()) {
			t = init.Type()
		}
		if backup == ir.TBuiltinUntyped && !ir.IsUntyped(init.Type()) {
			backup = init.Type()
		}
	}

	if t != ir.TBuiltinUntyped {
		for _, init := range expr.Initializers {
			if !v.tryMakeTypedLit(init, t) {
				break
			}
		}
	} else {
		t = backup
	}

	if t != ir.TBuiltinUntyped {
		for _, init := range expr.Initializers {
			if !checkTypes(v.c, t, init.Type()) {
				v.c.error(init.FirstPos(), "type mismatch: array elements must be of the same type (expected %s, got %s)", t, init.Type())
				break
			}
		}
	}

	if len(expr.Initializers) == 0 {
		v.c.error(expr.Lbrack.Pos, "array literal cannot have 0 elements")
	}

	if t == ir.TBuiltinUntyped {
		expr.T = t
	} else {
		expr.T = ir.NewArrayType(t, len(expr.Initializers))
	}
	return expr
}

func createDefaultLit(t ir.Type) ir.Expr {
	if t.ID() == ir.TStruct {
		tstruct := t.(*ir.StructType)
		lit := &ir.StructLit{}
		lit.T = t
		return createStructLit(tstruct, lit)
	} else if t.ID() == ir.TArray {
		tarray := t.(*ir.ArrayType)
		lit := &ir.ArrayLit{}
		lit.T = tarray
		for i := 0; i < tarray.Size; i++ {
			init := createDefaultLit(tarray.Elem)
			lit.Initializers = append(lit.Initializers, init)
		}
		return lit
	}
	return createDefaultBasicLit(t)
}

func createDefaultBasicLit(t ir.Type) *ir.BasicLit {
	var lit *ir.BasicLit
	if ir.IsTypeID(t, ir.TBool) {
		lit = &ir.BasicLit{Value: token.Synthetic(token.False, token.False.String())}
		lit.T = ir.NewBasicType(ir.TBool)
	} else if ir.IsTypeID(t, ir.TUInt64, ir.TInt64, ir.TUInt32, ir.TInt32, ir.TUInt16, ir.TInt16, ir.TUInt8, ir.TInt8) {
		lit = &ir.BasicLit{Value: token.Synthetic(token.Integer, "0")}
		lit.Raw = ir.BigIntZero
		lit.T = ir.NewBasicType(t.ID())
	} else if ir.IsTypeID(t, ir.TFloat64, ir.TFloat32) {
		lit = &ir.BasicLit{Value: token.Synthetic(token.Float, "0")}
		lit.Raw = ir.BigFloatZero
		lit.T = ir.NewBasicType(t.ID())
	} else if ir.IsTypeID(t, ir.TSlice) {
		lit = &ir.BasicLit{Value: token.Synthetic(token.Null, token.Null.String())}
		slice := t.(*ir.SliceType)
		lit.T = ir.NewSliceType(slice.Elem, true, true)
	} else if ir.IsTypeID(t, ir.TPointer) {
		lit = &ir.BasicLit{Value: token.Synthetic(token.Null, token.Null.String())}
		ptr := t.(*ir.PointerType)
		lit.T = ir.NewPointerType(ptr.Underlying, true)
	} else if ir.IsTypeID(t, ir.TFunc) {
		lit = &ir.BasicLit{Value: token.Synthetic(token.Null, token.Null.String())}
		fun := t.(*ir.FuncType)
		lit.T = ir.NewFuncType(fun.Params, fun.Return)
	} else if !ir.IsTypeID(t, ir.TUntyped) {
		panic(fmt.Sprintf("Unhandled init value for type %s", t.ID()))
	}
	return lit
}

func createStructLit(structt *ir.StructType, lit *ir.StructLit) *ir.StructLit {
	var initializers []*ir.KeyValue
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
		kv := &ir.KeyValue{}
		kv.Key = token.Synthetic(token.Ident, key)

		kv.Value = createDefaultLit(f.T)
		initializers = append(initializers, kv)
	}
	lit.Initializers = initializers
	return lit
}

func (v *typeChecker) VisitIdent(expr *ir.Ident) ir.Expr {
	sym := v.c.lookup(expr.Name.Literal)
	if sym == nil {
		v.c.error(expr.Pos(), "'%s' undefined", expr.Name.Literal)
		expr.T = ir.TBuiltinUntyped
	} else if sym.Untyped() || sym.DepCycle() {
		expr.T = ir.TBuiltinUntyped
	} else if v.exprMode != exprModeType && v.exprMode != exprModeFunc && sym.ID == ir.TypeSymbol {
		v.c.error(expr.Pos(), "type %s cannot be used in an expression", sym.T)
		expr.T = ir.TBuiltinUntyped
	} else {
		expr.SetSymbol(sym)
	}
	return expr
}

func (v *typeChecker) VisitDotExpr(expr *ir.DotExpr) ir.Expr {
	expr.X = ir.VisitExpr(v, expr.X)
	if ir.IsUntyped(expr.X.Type()) {
		expr.T = ir.TBuiltinUntyped
		return expr
	}

	var scope *ir.Scope
	untyped := false

	expr.X = tryDeref(expr.X)
	switch t := expr.X.Type().(type) {
	case *ir.StructType:
		scope = t.Scope
	case *ir.ArrayType:
		scope = t.Scope
	case *ir.SliceType:
		scope = t.Scope
	case *ir.BasicType:
		if t.ID() == ir.TUntyped {
			untyped = true
		}
	}

	if scope != nil {
		defer setScope(setScope(v.c, scope))
		v.VisitIdent(expr.Name)
		expr.T = expr.Name.Type()
	} else if !untyped {
		v.c.error(expr.X.FirstPos(), "invalid field access '%s' (type %s)", PrintExpr(expr), expr.X.Type())
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitCastExpr(expr *ir.CastExpr) ir.Expr {
	prevMode := v.exprMode
	v.exprMode = exprModeType
	expr.ToTyp = ir.VisitExpr(v, expr.ToTyp)
	v.exprMode = prevMode

	ident := ir.TypeExprToIdent(expr.ToTyp)
	err := true
	if ident == nil {
		v.c.error(expr.ToTyp.FirstPos(), "'%s' is not a valid type specifier", PrintExpr(expr.ToTyp))
	} else if ident.Sym != nil {
		if ident.Sym.ID != ir.TypeSymbol {
			v.c.error(expr.ToTyp.FirstPos(), "'%s' is not a type", PrintExpr(expr.ToTyp))
		} else {
			err = false
		}
	}

	expr.X = ir.VisitExpr(v, expr.X)

	if !err {
		t1 := expr.ToTyp.Type()
		t2 := expr.X.Type()
		if t2.ExplicitCastOK(t1) {
			expr.T = t1
		} else {
			v.c.error(expr.X.FirstPos(), "type mismatch: %s cannot be converted to %s", t2, t1)
		}
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitFuncCall(expr *ir.FuncCall) ir.Expr {
	prevMode := v.exprMode
	v.exprMode = exprModeFunc
	expr.X = ir.VisitExpr(v, expr.X)
	v.exprMode = prevMode

	t := expr.X.Type()

	if ir.IsUntyped(t) {
		expr.T = ir.TBuiltinUntyped
		return expr
	}

	if t.ID() != ir.TFunc {
		v.c.error(expr.X.FirstPos(), "'%s' is not callable (has type %s)", PrintExpr(expr), t)
		expr.T = ir.TBuiltinUntyped
		return expr
	}

	tfun := t.(*ir.FuncType)

	if len(tfun.Params) != len(expr.Args) {
		v.c.error(expr.X.FirstPos(), "'%s' takes %d argument(s) but called with %d", PrintExpr(expr.X), len(tfun.Params), len(expr.Args))
	} else {
		for i, arg := range expr.Args {
			expr.Args[i] = v.makeTypedExpr(arg, tfun.Params[i])
			tparam := tfun.Params[i]

			targ := arg.Type()
			if !checkTypes(v.c, targ, tparam) {
				v.c.error(arg.FirstPos(), "type mismatch: argument %d of function '%s' expects type %s (got type %s)",
					i, PrintExpr(expr.X), tparam, targ)
			}
		}
		expr.T = tfun.Return
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitAddressExpr(expr *ir.AddressExpr) ir.Expr {
	ro := expr.Decl.Is(token.Val)
	expr.X = ir.VisitExpr(v, expr.X)
	if !expr.X.Lvalue() {
		v.c.error(expr.X.FirstPos(), "cannot create pointer to '%s' (not an lvalue)", PrintExpr(expr.X))
	} else if expr.X.ReadOnly() && !ro {
		v.c.error(expr.X.FirstPos(), "cannot create writable pointer to '%s' (read-only)", PrintExpr(expr.X))
	} else {
		if tslice, ok := expr.X.Type().(*ir.SliceType); ok {
			if !tslice.Ptr {
				tslice.Ptr = true
				tslice.ReadOnly = ro
				expr.T = tslice
			} else {
				expr.T = ir.NewPointerType(tslice, ro)
			}
		} else {
			expr.T = ir.NewPointerType(expr.X.Type(), ro)
		}
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitIndexExpr(expr *ir.IndexExpr) ir.Expr {
	expr.X = v.makeTypedExpr(expr.X, nil)
	expr.Index = v.makeTypedExpr(expr.Index, nil)

	var telem ir.Type
	untyped := false

	expr.X = tryDeref(expr.X)
	switch t := expr.X.Type().(type) {
	case *ir.ArrayType:
		telem = t.Elem
	case *ir.SliceType:
		telem = t.Elem
	case *ir.BasicType:
		if t.ID() == ir.TUntyped {
			untyped = true
		}
	}

	if telem != nil {
		if !ir.IsUntyped(expr.Index.Type()) {
			if !ir.IsIntegerType(expr.Index.Type()) {
				v.c.error(expr.Index.FirstPos(), "'%s' cannot be used as an index (has type %s)", PrintExpr(expr.Index), expr.Index.Type())
			} else {
				expr.T = telem
			}
		}
	} else if !untyped {
		v.c.error(expr.X.FirstPos(), "invalid index access '%s' (type %s)", PrintExpr(expr), expr.X.Type())
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

func (v *typeChecker) VisitSliceExpr(expr *ir.SliceExpr) ir.Expr {
	expr.X = v.makeTypedExpr(expr.X, nil)

	if expr.Start != nil {
		expr.Start = v.makeTypedExpr(expr.Start, nil)
	}

	if expr.End != nil {
		expr.End = v.makeTypedExpr(expr.End, nil)
	}

	var telem ir.Type
	var lenSym *ir.Symbol
	untyped := false

	expr.X = tryDeref(expr.X)
	switch t := expr.X.Type().(type) {
	case *ir.ArrayType:
		telem = t.Elem
		lenSym = t.Scope.Lookup(ir.LenField)
	case *ir.SliceType:
		telem = t.Elem
		lenSym = t.Scope.Lookup(ir.LenField)
	case *ir.BasicType:
		if t.ID() == ir.TUntyped {
			untyped = true
		}
	}

	if telem != nil {
		err := false

		if expr.Start != nil {
			if !ir.IsUntyped(expr.Start.Type()) && !ir.IsIntegerType(expr.Start.Type()) {
				v.c.error(expr.Start.FirstPos(), "'%s' cannot be used as slice index (has type %s)", PrintExpr(expr.Start), expr.Start.Type())
				err = true
			}
		}

		if expr.End != nil {
			if !ir.IsUntyped(expr.End.Type()) && !ir.IsIntegerType(expr.End.Type()) {
				v.c.error(expr.End.FirstPos(), "'%s' cannot be used as slice index (has type %s)", PrintExpr(expr.End), expr.End.Type())
				err = true
			}
		}

		if !err {
			if expr.Start == nil {
				tstart := ir.TBuiltinInt32
				if expr.End != nil {
					tstart = expr.End.Type()
				}
				expr.Start = createDefaultBasicLit(tstart)
			} else if expr.Start.Type().ID() != ir.TInt32 {
				cast := &ir.CastExpr{X: expr.Start}
				cast.T = ir.TBuiltinInt32
				expr.Start = cast
			}

			if expr.End == nil {
				name := &ir.Ident{Name: token.Synthetic(token.Ident, lenSym.Name)}
				name.Sym = lenSym
				len := &ir.DotExpr{
					X:    expr.X,
					Dot:  token.Synthetic(token.Dot, token.Dot.String()),
					Name: name,
				}
				len.T = lenSym.T
				expr.End = len
			} else if expr.End.Type().ID() != ir.TInt32 {
				cast := &ir.CastExpr{X: expr.End}
				cast.T = ir.TBuiltinInt32
				expr.End = cast
			}

			expr.T = ir.NewSliceType(telem, true, false)
		}
	} else if !untyped {
		v.c.error(expr.X.FirstPos(), "invalid slice '%s' (type %s)", PrintExpr(expr), expr.X.Type())
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}
