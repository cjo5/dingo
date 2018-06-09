package semantics

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

func toBasicLit(expr ir.Expr) *ir.BasicLit {
	if constExpr, ok := expr.(*ir.ConstExpr); ok {
		expr = constExpr.X
	}
	res, _ := expr.(*ir.BasicLit)
	return res
}

func (v *typeChecker) VisitBinaryExpr(expr *ir.BinaryExpr) ir.Expr {
	expr.Left = ir.VisitExpr(v, expr.Left)
	expr.Right = ir.VisitExpr(v, expr.Right)
	left := expr.Left
	right := expr.Right

	if ir.IsUntyped(left.Type()) || ir.IsUntyped(right.Type()) {
		expr.T = ir.TBuiltinUntyped
		return expr
	}

	// Attempt to set concrete/compatible types for the operands before checking them
	err := false

	if !ir.IsCompilerType(left.Type()) {
		if !v.tryMakeTypedLit(right, left.Type()) {
			err = true
		}
	} else if !ir.IsCompilerType(right.Type()) {
		if !v.tryMakeTypedLit(left, right.Type()) {
			err = true
		}
	} else if ir.IsTypeID(left.Type(), ir.TBigInt, ir.TBigFloat) && ir.IsTypeID(right.Type(), ir.TBigInt, ir.TBigFloat) {
		// If needed, promote BigInt to BigFloat
		if ir.IsTypeID(left.Type(), ir.TBigFloat) && ir.IsTypeID(right.Type(), ir.TBigInt) {
			lit := right.(*ir.BasicLit)
			lit.Raw = toBigFloat(lit.Raw.(*big.Int))
			lit.T = ir.NewBasicType(ir.TBigFloat)

		} else if ir.IsTypeID(right.Type(), ir.TBigFloat) && ir.IsTypeID(left.Type(), ir.TBigInt) {
			lit := left.(*ir.BasicLit)
			lit.Raw = toBigFloat(lit.Raw.(*big.Int))
			lit.T = ir.NewBasicType(ir.TBigFloat)
		}
	}

	if err {
		expr.T = ir.TBuiltinUntyped
		return expr
	}

	// Check types are compatible/equal

	tleft := left.Type()
	tright := right.Type()

	if tleft.Equals(tright) {
		// OK
	} else if tleft.ImplicitCastOK(tright) {
		cast := &ir.CastExpr{}
		cast.X = left
		cast.T = tright
		left = cast
	} else if tright.ImplicitCastOK(tleft) {
		cast := &ir.CastExpr{}
		cast.X = right
		cast.T = tleft
		right = cast
	} else {
		v.c.errorNode(expr, "type mismatch %s and %s", left.Type(), right.Type())
		expr.T = ir.TBuiltinUntyped
		return expr
	}

	// Check if operation can be performed on the type, and attempt constant folding.

	badop := false
	logicop := expr.Op.OneOf(token.Land, token.Lor)
	eqop := expr.Op.OneOf(token.Eq, token.Neq)
	orderop := expr.Op.OneOf(token.Gt, token.GtEq, token.Lt, token.LtEq)
	mathop := expr.Op.OneOf(token.Add, token.Sub, token.Mul, token.Div, token.Mod)

	toperand := expr.Left.Type() // Left and right should have same type at this point
	texpr := toperand
	if logicop || eqop || orderop {
		texpr = ir.TBuiltinBool
	}

	fold := false
	foldCmpRes := 0
	foldLogicRes := false
	var foldMathRes interface{}

	leftLit := toBasicLit(left)
	rightLit := toBasicLit(right)

	if ir.IsNumericType(toperand) {
		if logicop {
			badop = true
		} else if leftLit != nil && rightLit != nil {
			if ir.IsIntegerType(toperand) {
				leftInt := leftLit.Raw.(*big.Int)
				rightInt := rightLit.Raw.(*big.Int)
				if eqop || orderop {
					foldCmpRes = leftInt.Cmp(rightInt)
					fold = true
				} else if expr.Op.OneOf(token.Div, token.Mod) && rightInt.Cmp(ir.BigIntZero) == 0 {
					v.c.errorNode(expr, "division by zero")
					err = true
				} else {
					intRes := big.NewInt(0)
					switch expr.Op {
					case token.Add:
						intRes.Add(leftInt, rightInt)
					case token.Sub:
						intRes.Sub(leftInt, rightInt)
					case token.Mul:
						intRes.Mul(leftInt, rightInt)
					case token.Div:
						intRes.Div(leftInt, rightInt)
					case token.Mod:
						intRes.Mod(leftInt, rightInt)
					}
					foldMathRes = intRes
					fold = true
					if integerOverflows(intRes, toperand.ID()) {
						v.c.errorNode(expr, "result from operation '%s' overflows type %s", expr.Op, toperand)
					}
				}
			} else if ir.IsFloatType(toperand) {
				leftFloat := leftLit.Raw.(*big.Float)
				rightFloat := rightLit.Raw.(*big.Float)
				if eqop || orderop {
					foldCmpRes = leftFloat.Cmp(rightFloat)
					fold = true
				} else if expr.Op.Is(token.Div) && rightFloat.Cmp(ir.BigFloatZero) == 0 {
					v.c.errorNode(expr, "division by zero")
					err = true
				} else if expr.Op.Is(token.Mod) {
					badop = true
				} else {
					floatRes := big.NewFloat(0)
					switch expr.Op {
					case token.Add:
						floatRes.Add(leftFloat, rightFloat)
					case token.Sub:
						floatRes.Sub(leftFloat, rightFloat)
					case token.Mul:
						floatRes.Mul(leftFloat, rightFloat)
					case token.Div:
						floatRes.Quo(leftFloat, rightFloat)
					}
					foldMathRes = floatRes
					fold = true
					if floatOverflows(floatRes, toperand.ID()) {
						v.c.errorNode(expr, "result from operation '%s' overflows type %s", expr.Op, toperand)
					}
				}
			}
		}
	} else if ir.IsTypeID(toperand, ir.TBool) {
		if orderop || mathop {
			badop = true
		} else if leftLit != nil && rightLit != nil {
			leftTrue := leftLit.Tok.Is(token.True)
			rightTrue := rightLit.Tok.Is(token.True)
			switch expr.Op {
			case token.Land:
				foldLogicRes = leftTrue && rightTrue
			case token.Lor:
				foldLogicRes = leftTrue || rightTrue
			case token.Eq:
				foldCmpRes = 1
				if leftTrue == rightTrue {
					foldCmpRes = 0
				}
			case token.Neq:
				foldCmpRes = 1
				if leftTrue != rightTrue {
					foldCmpRes = 0
				}
			}
			fold = true
		}
	} else if ir.IsTypeID(toperand, ir.TPointer) {
		if orderop || mathop {
			badop = true
		}
	} else {
		badop = true
	}

	if badop {
		v.c.errorNode(expr, "operator '%s' cannot be performed on type %s", expr.Op, toperand)
		err = true
	}

	if err {
		expr.T = ir.TBuiltinUntyped
	} else {
		expr.T = texpr
		if fold {
			litRes := &ir.BasicLit{Value: ""}
			litRes.SetRange(left.Pos(), right.EndPos())
			litRes.T = texpr
			litRes.Tok = leftLit.Tok
			litRes.Rewrite = 1
			if mathop {
				litRes.Raw = foldMathRes
			} else {
				litRes.Raw = nil
				istrue := false
				if logicop {
					istrue = foldLogicRes
				} else if eqop || orderop {
					switch expr.Op {
					case token.Eq:
						istrue = (foldCmpRes == 0)
					case token.Neq:
						istrue = (foldCmpRes != 0)
					case token.Gt:
						istrue = (foldCmpRes > 0)
					case token.GtEq:
						istrue = (foldCmpRes >= 0)
					case token.Lt:
						istrue = (foldCmpRes < 0)
					case token.LtEq:
						istrue = (foldCmpRes <= 0)
					}
				}
				if istrue {
					litRes.Tok = token.True
				} else {
					litRes.Tok = token.False
				}
			}
			return litRes
		}
	}

	return expr
}

func (v *typeChecker) VisitUnaryExpr(expr *ir.UnaryExpr) ir.Expr {
	expr.X = ir.VisitExpr(v, expr.X)
	tx := expr.X.Type()

	if ir.IsUntyped(tx) {
		expr.T = ir.TBuiltinUntyped
		return expr
	}

	switch expr.Op {
	case token.Sub:
		if ir.IsNumericType(tx) {
			expr.T = tx
			if lit := toBasicLit(expr.X); lit != nil {
				var raw interface{}
				overflow := false

				switch n := lit.Raw.(type) {
				case *big.Int:
					intRes := big.NewInt(0)
					raw = intRes.Neg(n)
					if integerOverflows(intRes, tx.ID()) {
						overflow = true
					}
				case *big.Float:
					floatRes := big.NewFloat(0)
					raw = floatRes.Neg(n)
					if floatOverflows(floatRes, tx.ID()) {
						overflow = true
					}
				default:
					panic(fmt.Sprintf("Unhandled raw type %T", n))
				}

				if overflow {
					v.c.errorNode(expr, "result from additive inverse '%s' overflows type %s", expr.Op, lit.T)
				}

				litRes := &ir.BasicLit{Value: ""}
				litRes.Tok = lit.Tok
				litRes.Rewrite = 1
				litRes.Raw = raw
				litRes.T = tx
				return litRes
			}
		} else {
			v.c.error(expr.Pos(), "additive inverse cannot be performed on type %s", tx)
		}
	case token.Lnot:
		if tx.ID() == ir.TBool {
			expr.T = tx
			if lit := toBasicLit(expr.X); lit != nil {
				litRes := &ir.BasicLit{Value: ""}
				litRes.SetRange(expr.Pos(), expr.EndPos())
				litRes.Tok = token.True
				if lit.Tok.Is(token.True) {
					litRes.Tok = token.False
				}
				litRes.Rewrite = 1
				litRes.Raw = nil
				litRes.T = tx
				return litRes
			}
		} else {
			v.c.error(expr.Pos(), "logical not cannot be performed on type %s)", tx)
		}
	case token.Mul:
		lvalue := false

		if deref, ok := expr.X.(*ir.UnaryExpr); ok {
			if deref.Op == token.And {
				// Inverse
				lvalue = deref.X.Lvalue()
			}
		} else {
			lvalue = expr.X.Lvalue()
		}

		if !lvalue {
			v.c.error(expr.X.Pos(), "expression cannot be dereferenced (not an lvalue)")
		} else {
			switch t := tx.(type) {
			case *ir.PointerType:
				expr.T = t.Underlying
			default:
				v.c.error(expr.X.Pos(), "expression cannot be dereferenced (has type %s)", tx)
			}
		}
	default:
		panic(fmt.Sprintf("Unhandled unary op %s", expr.Op))
	}

	if expr.T == nil {
		expr.T = ir.TBuiltinUntyped
	}

	return expr
}

// TODO: Move to lexer and add better support for escape sequences.
func (v *typeChecker) unescapeStringLiteral(lit *ir.BasicLit) (string, bool) {
	escaped := []rune(lit.Value)
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
				v.c.error(lit.Pos(), "invalid escape sequence '\\%c'", ch2)
				return "", false
			}
		}
		unescaped = append(unescaped, ch1)
	}

	return string(unescaped), true
}

func removeUnderscores(lit string) string {
	res := strings.Replace(lit, "_", "", -1)
	return res
}

func (v *typeChecker) VisitBasicLit(expr *ir.BasicLit) ir.Expr {
	if expr.Tok == token.False || expr.Tok == token.True {
		expr.T = ir.TBuiltinBool
	} else if expr.Tok == token.Char {
		if expr.Raw == nil {
			if raw, ok := v.unescapeStringLiteral(expr); ok {
				common.Assert(len(raw) == 1, "Unexpected length on char literal")

				val := big.NewInt(0)
				val.SetUint64(uint64(raw[0]))
				expr.Raw = val
				expr.T = ir.NewBasicType(ir.TBigInt)
			} else {
				expr.T = ir.TBuiltinUntyped
			}
		}
	} else if expr.Tok == token.String {
		if expr.Raw == nil {
			if raw, ok := v.unescapeStringLiteral(expr); ok {
				if expr.Prefix == nil {
					expr.T = ir.NewSliceType(ir.TBuiltinInt8, true, true)
					expr.Raw = raw
				} else if expr.Prefix.Literal == "c" {
					expr.T = ir.NewPointerType(ir.TBuiltinInt8, true)
					expr.Raw = raw
				} else {
					v.c.error(expr.Prefix.Pos(), "invalid string prefix '%s'", expr.Prefix.Literal)
					expr.T = ir.TBuiltinUntyped
				}
			} else {
				expr.T = ir.TBuiltinUntyped
			}
		}
	} else if expr.Tok == token.Integer {
		if expr.Raw == nil {
			base := ir.TBigInt
			target := ir.TBigInt

			if expr.Suffix != nil {
				switch expr.Suffix.Literal {
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
					v.c.error(expr.Suffix.Pos(), "invalid int suffix '%s'", expr.Suffix.Literal)
					base = ir.TUntyped
				}
			}

			if base != ir.TUntyped {
				normalized := removeUnderscores(expr.Value)

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
					v.c.error(expr.Pos(), "unable to interpret int literal '%s'", normalized)
				}
			}

			if expr.T == nil {
				expr.T = ir.TBuiltinUntyped
			}
		}
	} else if expr.Tok == token.Float {
		if expr.Raw == nil {
			base := ir.TBigFloat
			target := ir.TBigFloat

			if expr.Suffix != nil {
				switch expr.Suffix.Literal {
				case ir.TFloat64.String():
					target = ir.TFloat64
				case ir.TFloat32.String():
					target = ir.TFloat32
				default:
					v.c.error(expr.Suffix.Pos(), "invalid float suffix '%s'", expr.Suffix.Literal)
					base = ir.TUntyped
				}
			}

			if base != ir.TUntyped {
				val := big.NewFloat(0)
				normalized := removeUnderscores(expr.Value)
				_, ok := val.SetString(normalized)
				if ok {
					expr.T = ir.NewBasicType(base)
					expr.Raw = val

					if target != ir.TBigFloat {
						v.tryMakeTypedLit(expr, ir.NewBasicType(target))
					}
				} else {
					v.c.error(expr.Pos(), "unable to interpret float literal '%s'", normalized)
				}
			}

			if expr.T == nil {
				expr.T = ir.TBuiltinUntyped
			}
		}
	} else if expr.Tok == token.Null {
		expr.T = ir.NewPointerType(ir.TBuiltinUntyped, false)
	} else {
		panic(fmt.Sprintf("Unhandled literal %s at %s", expr.Tok, expr.Pos()))
	}

	return expr
}

func (v *typeChecker) VisitStructLit(expr *ir.StructLit) ir.Expr {
	expr.Name = v.visitType(expr.Name)
	tname := expr.Name.Type()

	if ir.IsUntyped(tname) {
		expr.T = ir.TBuiltinUntyped
		return expr
	} else if typeSym := ir.ExprSymbol(expr.Name); typeSym != nil {
		if typeSym.ID != ir.TypeSymbol && typeSym.T.ID() != ir.TStruct {
			v.c.error(expr.Name.Pos(), "'%s' is not a struct", typeSym.Name)
			expr.T = ir.TBuiltinUntyped
			return expr
		}
	}

	err := false
	inits := make(map[string]ir.Expr)
	tstruct := tname.(*ir.StructType)

	for _, kv := range expr.Initializers {
		if existing, ok := inits[kv.Key.Literal]; ok {
			if existing != nil {
				v.c.error(kv.Key.Pos(), "duplicate field key '%s'", kv.Key.Literal)
			}
			inits[kv.Key.Literal] = nil
			err = true
			continue
		}

		fieldSym := tstruct.Scope.Lookup(kv.Key.Literal)
		if fieldSym == nil {
			v.c.error(kv.Key.Pos(), "'%s' undefined struct field", kv.Key.Literal)
			inits[kv.Key.Literal] = nil
			err = true
			continue
		}

		kv.Value = v.makeTypedExpr(kv.Value, fieldSym.T)

		if ir.IsUntyped(fieldSym.T) || ir.IsUntyped(kv.Value.Type()) {
			inits[kv.Key.Literal] = nil
			err = true
			continue
		}

		if !checkTypes(v.c, fieldSym.T, kv.Value.Type()) {
			v.c.error(kv.Key.Pos(), "field '%s' expects type %s (got type %s)",
				kv.Key.Literal, fieldSym.T, kv.Value.Type())
			inits[kv.Key.Literal] = nil
			err = true
			continue
		}

		inits[kv.Key.Literal] = kv.Value
	}

	if err {
		expr.T = ir.TBuiltinUntyped
		return expr
	}

	expr.T = tstruct
	return createStructLit(tstruct, expr)
}

func (v *typeChecker) VisitArrayLit(expr *ir.ArrayLit) ir.Expr {
	texpr := ir.TBuiltinUntyped
	tbackup := ir.TBuiltinUntyped

	for i, init := range expr.Initializers {
		init = ir.VisitExpr(v, init)
		expr.Initializers[i] = init

		if ir.IsUntyped(texpr) && !ir.IsCompilerType(init.Type()) {
			texpr = init.Type()
		}
		if ir.IsUntyped(tbackup) && !ir.IsUntyped(init.Type()) {
			tbackup = init.Type()
		}
	}

	if ir.IsUntyped(texpr) {
		texpr = tbackup
	} else {
		for _, init := range expr.Initializers {
			if !v.tryMakeTypedLit(init, texpr) {
				break
			}
		}
	}

	if !ir.IsUntyped(texpr) {
		for _, init := range expr.Initializers {
			if ir.IsUntyped(init.Type()) {
				texpr = ir.TBuiltinUntyped
			} else if !checkTypes(v.c, texpr, init.Type()) {
				v.c.error(init.Pos(), "array elements must be of the same type (expected type %s, got type %s)", texpr, init.Type())
				texpr = ir.TBuiltinUntyped
				break
			}
		}
	}

	if len(expr.Initializers) == 0 {
		v.c.error(expr.Pos(), "array literal cannot have 0 elements")
		texpr = ir.TBuiltinUntyped
	}

	if ir.IsUntyped(texpr) {
		expr.T = texpr
	} else {
		expr.T = ir.NewArrayType(texpr, len(expr.Initializers))
	}

	return expr
}

func (v *typeChecker) VisitSizeExpr(expr *ir.SizeExpr) ir.Expr {
	expr.X = v.visitType(expr.X)
	expr.T = ir.TBuiltinUntyped
	tx := expr.X.Type()

	if ir.IsUntyped(tx) {
		return expr
	}

	if tx.ID() == ir.TModule || tx.ID() == ir.TVoid {
		v.c.errorNode(expr.X, "type %s does not have a size", tx)
		return expr
	}

	size := v.c.target.Sizeof(tx)
	return createIntLit(size, ir.TBigInt)
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

func createIntLit(val int, tid ir.TypeID) *ir.BasicLit {
	lit := &ir.BasicLit{Tok: token.Integer, Value: strconv.FormatInt(int64(val), 10)}
	lit.T = ir.NewBasicType(tid)
	if val == 0 {
		lit.Raw = ir.BigIntZero
	} else {
		lit.Raw = big.NewInt(int64(val))
	}
	return lit
}

func createDefaultBasicLit(t ir.Type) *ir.BasicLit {
	var lit *ir.BasicLit
	if ir.IsTypeID(t, ir.TBool) {
		lit = &ir.BasicLit{Tok: token.False, Value: token.False.String()}
		lit.T = ir.NewBasicType(ir.TBool)
	} else if ir.IsTypeID(t, ir.TUInt64, ir.TInt64, ir.TUInt32, ir.TInt32, ir.TUInt16, ir.TInt16, ir.TUInt8, ir.TInt8) {
		lit = createIntLit(0, t.ID())
	} else if ir.IsTypeID(t, ir.TFloat64, ir.TFloat32) {
		lit = &ir.BasicLit{Tok: token.Float, Value: "0"}
		lit.Raw = ir.BigFloatZero
		lit.T = ir.NewBasicType(t.ID())
	} else if ir.IsTypeID(t, ir.TSlice) {
		lit = &ir.BasicLit{Tok: token.Null, Value: token.Null.String()}
		slice := t.(*ir.SliceType)
		lit.T = ir.NewSliceType(slice.Elem, true, true)
	} else if ir.IsTypeID(t, ir.TPointer) {
		lit = &ir.BasicLit{Tok: token.Null, Value: token.Null.String()}
		ptr := t.(*ir.PointerType)
		lit.T = ir.NewPointerType(ptr.Underlying, true)
	} else if ir.IsTypeID(t, ir.TFunc) {
		lit = &ir.BasicLit{Tok: token.Null, Value: token.Null.String()}
		fun := t.(*ir.FuncType)
		lit.T = ir.NewFuncType(fun.Params, fun.Return, fun.C)
	} else if !ir.IsTypeID(t, ir.TUntyped) {
		panic(fmt.Sprintf("Unhandled init value for type %s", t.ID()))
	}
	return lit
}

func createStructLit(tstruct *ir.StructType, lit *ir.StructLit) *ir.StructLit {
	var initializers []*ir.KeyValue
	for _, f := range tstruct.Fields {
		name := f.Name()
		found := false
		for _, init := range lit.Initializers {
			if init.Key.Literal == name {
				initializers = append(initializers, init)
				found = true
				break
			}
		}
		if found {
			continue
		}
		kv := &ir.KeyValue{}
		kv.Key = ir.NewIdent2(token.Ident, name)

		kv.Value = createDefaultLit(f.T)
		initializers = append(initializers, kv)
	}
	lit.Initializers = initializers
	return lit
}
