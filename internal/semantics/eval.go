package semantics

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

func toBasicLit(expr ir.Expr) *ir.BasicLit {
	if constExpr, ok := expr.(*ir.ConstExpr); ok {
		expr = constExpr.X
	}
	res, _ := expr.(*ir.BasicLit)
	return res
}

func (c *checker) checkBinaryExpr(expr *ir.BinaryExpr) ir.Expr {
	expr.Left = c.typeswitchExpr(expr.Left)
	expr.Right = c.typeswitchExpr(expr.Right)
	left := expr.Left
	right := expr.Right

	if ir.IsUntyped(left.Type()) || ir.IsUntyped(right.Type()) {
		expr.T = ir.TBuiltinUntyped
		return expr
	}

	// Attempt to set concrete/compatible types for the operands before checking them
	err := false

	if !ir.IsCompilerType(left.Type()) {
		if !c.tryMakeTypedLit(right, left.Type()) {
			err = true
		}
	} else if !ir.IsCompilerType(right.Type()) {
		if !c.tryMakeTypedLit(left, right.Type()) {
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
	} else if ir.IsImplicitCastNeeded(tleft, tright) {
		cast := &ir.CastExpr{}
		cast.X = left
		cast.T = tright
		left = cast
	} else if ir.IsImplicitCastNeeded(tright, tleft) {
		cast := &ir.CastExpr{}
		cast.X = right
		cast.T = tleft
		right = cast
	} else {
		c.errorNode(expr, "type mismatch %s and %s", left.Type(), right.Type())
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
					c.errorNode(expr, "division by zero")
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
						c.errorNode(expr, "result from operation '%s' overflows type %s", expr.Op, toperand)
					}
				}
			} else if ir.IsFloatType(toperand) {
				leftFloat := leftLit.Raw.(*big.Float)
				rightFloat := rightLit.Raw.(*big.Float)
				if eqop || orderop {
					foldCmpRes = leftFloat.Cmp(rightFloat)
					fold = true
				} else if expr.Op.Is(token.Div) && rightFloat.Cmp(ir.BigFloatZero) == 0 {
					c.errorNode(expr, "division by zero")
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
						c.errorNode(expr, "result from operation '%s' overflows type %s", expr.Op, toperand)
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
		c.errorNode(expr, "operator '%s' cannot be performed on type %s", expr.Op, toperand)
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

func (c *checker) checkUnaryExpr(expr *ir.UnaryExpr) ir.Expr {
	expr.X = c.typeswitchExpr(expr.X)
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
					c.errorNode(expr, "result from additive inverse '%s' overflows type %s", expr.Op, lit.T)
				}

				litRes := &ir.BasicLit{Value: ""}
				litRes.Tok = lit.Tok
				litRes.Rewrite = 1
				litRes.Raw = raw
				litRes.T = tx
				return litRes
			}
		} else {
			c.error(expr.Pos(), "additive inverse cannot be performed on type %s", tx)
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
			c.error(expr.Pos(), "logical not cannot be performed on type %s)", tx)
		}
	case token.Deref:
		switch t := ir.ToBaseType(tx).(type) {
		case *ir.PointerType:
			expr.T = t.Elem
		default:
			c.error(expr.X.Pos(), "expression cannot be dereferenced (has type %s)", tx)
		}
	case token.Addr:
		ro := expr.Decl.Is(token.Val)
		if !expr.X.Lvalue() {
			c.error(expr.X.Pos(), "expression is not an lvalue")
		} else if expr.X.ReadOnly() && !ro {
			c.error(expr.X.Pos(), "expression is read-only")
		} else {
			if tslice, ok := tx.(*ir.SliceType); ok {
				if !tslice.Ptr {
					tslice.Ptr = true
					tslice.ReadOnly = ro
					expr.T = tslice
				} else {
					expr.T = ir.NewPointerType(tslice, ro)
				}
			} else {
				expr.T = ir.NewPointerType(tx, ro)
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
func (c *checker) unescapeStringLiteral(lit *ir.BasicLit) (string, bool) {
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
				c.error(lit.Pos(), "invalid escape sequence '\\%c'", ch2)
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

func (c *checker) checkBasicLit(expr *ir.BasicLit) ir.Expr {
	if expr.Tok == token.False || expr.Tok == token.True {
		expr.T = ir.TBuiltinBool
	} else if expr.Tok == token.Char {
		if expr.Raw == nil {
			if raw, ok := c.unescapeStringLiteral(expr); ok {
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
			if raw, ok := c.unescapeStringLiteral(expr); ok {
				if expr.Prefix == nil {
					expr.T = ir.NewSliceType(ir.TBuiltinInt8, true, true)
					expr.Raw = raw
				} else {
					prefix := ir.ExprNameToText(expr.Prefix)
					if prefix == "c" {
						expr.T = ir.NewPointerType(ir.TBuiltinInt8, true)
						expr.Raw = raw
					} else {
						c.error(expr.Prefix.Pos(), "invalid string prefix '%s'", prefix)
						expr.T = ir.TBuiltinUntyped
					}
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
				suffix := ir.ExprNameToText(expr.Suffix)
				switch suffix {
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
					c.error(expr.Suffix.Pos(), "invalid int suffix '%s'", suffix)
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
						c.tryMakeTypedLit(expr, ir.NewBasicType(target))
					}
				} else {
					c.error(expr.Pos(), "unable to interpret int literal '%s'", normalized)
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
				suffix := ir.ExprNameToText(expr.Suffix)
				switch suffix {
				case ir.TFloat64.String():
					target = ir.TFloat64
				case ir.TFloat32.String():
					target = ir.TFloat32
				default:
					c.error(expr.Suffix.Pos(), "invalid float suffix '%s'", suffix)
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
						c.tryMakeTypedLit(expr, ir.NewBasicType(target))
					}
				} else {
					c.error(expr.Pos(), "unable to interpret float literal '%s'", normalized)
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

func (c *checker) checkIdent(expr *ir.Ident) ir.Expr {
	sym := expr.Sym
	if sym == nil {
		sym = c.lookup(expr.Literal)
	} else {
		expr.SetSymbol(nil)
	}

	err := false

	if sym == nil {
		c.error(expr.Pos(), "'%s' undefined", expr.Literal)
		err = true
	} else if sym.T == nil || sym.IsUntyped() {
		// Cycle or an error has already occurred
		err = true
	} else if c.exprMode != exprModeDot {
		if c.exprMode == exprModeType {
			if !sym.IsType() {
				c.error(expr.Pos(), "'%s' is not a type", sym.Name)
				err = true
			}
		} else {
			if sym.ID == ir.ModuleSymbol {
				c.error(expr.Pos(), "module '%s' cannot be used in an expression", sym.Name)
				err = true
			} else if sym.IsType() {
				c.error(expr.Pos(), "type %s cannot be used in an expression", sym.T)
				err = true
			}
		}
	}

	if err {
		expr.T = ir.TBuiltinUntyped
	} else {
		decl := c.topDecl()
		// Check symbol in other module is public (struct fields are exempted).
		if !sym.Public && sym.ModFQN() != decl.Symbol().ModFQN() && sym.Parent.ID != ir.FieldScope {
			c.error(expr.Pos(), "'%s' is not public", expr.Literal)
			expr.T = ir.TBuiltinUntyped
			err = true
		} else {
			expr.SetSymbol(sym)
		}
	}

	if !err && sym.ID == ir.ConstSymbol {
		constExpr := &ir.ConstExpr{X: c.constExprs[sym]}
		constExpr.T = expr.T
		constExpr.SetRange(expr.Pos(), expr.EndPos())
		return constExpr
	}

	return expr
}

type numericCastResult int

const (
	numericCastOK numericCastResult = iota
	numericCastFails
	numericCastOverflows
	numericCastTruncated
)

func toBigFloat(val *big.Int) *big.Float {
	res := big.NewFloat(0)
	res.SetInt(val)
	return res
}

func toBigInt(val *big.Float) *big.Int {
	if !val.IsInt() {
		return nil
	}
	res := big.NewInt(0)
	val.Int(res)
	return res
}

func integerOverflows(val *big.Int, t ir.TypeID) bool {
	fits := true

	switch t {
	case ir.TBigInt:
		// OK
	case ir.TUInt64:
		fits = 0 <= val.Cmp(ir.BigIntZero) && val.Cmp(ir.MaxU64) <= 0
	case ir.TUInt32:
		fits = 0 <= val.Cmp(ir.BigIntZero) && val.Cmp(ir.MaxU32) <= 0
	case ir.TUInt16:
		fits = 0 <= val.Cmp(ir.BigIntZero) && val.Cmp(ir.MaxU16) <= 0
	case ir.TUInt8:
		fits = 0 <= val.Cmp(ir.BigIntZero) && val.Cmp(ir.MaxU8) <= 0
	case ir.TInt64:
		fits = 0 <= val.Cmp(ir.MinI64) && val.Cmp(ir.MaxI64) <= 0
	case ir.TInt32:
		fits = 0 <= val.Cmp(ir.MinI32) && val.Cmp(ir.MaxI32) <= 0
	case ir.TInt16:
		fits = 0 <= val.Cmp(ir.MinI16) && val.Cmp(ir.MaxI16) <= 0
	case ir.TInt8:
		fits = 0 <= val.Cmp(ir.MinI8) && val.Cmp(ir.MaxI8) <= 0
	}

	return !fits
}

func floatOverflows(val *big.Float, t ir.TypeID) bool {
	fits := true

	switch t {
	case ir.TBigFloat:
		// OK
	case ir.TFloat64:
		fits = 0 <= val.Cmp(ir.MinF64) && val.Cmp(ir.MaxF64) <= 0
	case ir.TFloat32:
		fits = 0 <= val.Cmp(ir.MinF32) && val.Cmp(ir.MaxF32) <= 0
	}

	return !fits
}

func typeCastNumericLit(lit *ir.BasicLit, target ir.Type) numericCastResult {
	res := numericCastOK
	id := target.ID()

	switch t := lit.Raw.(type) {
	case *big.Int:
		switch id {
		case ir.TBigInt, ir.TUInt64, ir.TUInt32, ir.TUInt16, ir.TUInt8, ir.TInt64, ir.TInt32, ir.TInt16, ir.TInt8:
			if integerOverflows(t, id) {
				res = numericCastOverflows
			}
		case ir.TBigFloat, ir.TFloat64, ir.TFloat32:
			fval := toBigFloat(t)
			if floatOverflows(fval, id) {
				res = numericCastOverflows
			} else {
				lit.Raw = fval
			}
		default:
			return numericCastFails
		}
	case *big.Float:
		switch id {
		case ir.TBigInt, ir.TUInt64, ir.TUInt32, ir.TUInt16, ir.TUInt8, ir.TInt64, ir.TInt32, ir.TInt16, ir.TInt8:
			if ival := toBigInt(t); ival != nil {
				if integerOverflows(ival, id) {
					res = numericCastOverflows
				} else {
					lit.Raw = ival
				}
			} else {
				res = numericCastTruncated
			}
		case ir.TBigFloat, ir.TFloat64, ir.TFloat32:
			if floatOverflows(t, id) {
				res = numericCastOverflows
			}
		default:
			return numericCastFails
		}
	default:
		return numericCastFails
	}

	if res == numericCastOK {
		lit.T = target
	}

	return res
}
