package semantics

import (
	"math/big"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
)

type typeChecker struct {
	ir.BaseVisitor
	signature bool
	exprMode  int
	c         *context
}

func typeCheck(c *context) {
	v := &typeChecker{c: c}
	c.resetWalkState()
	ir.VisitModuleSet(v, c.set)
}

func checkCompleteType(t1 ir.Type) bool {
	complete := true
	switch t2 := t1.(type) {
	case *ir.SliceType:
		if !t2.Ptr {
			complete = false
		}
	}
	return complete
}

// Returns false if an error should be reported
func checkTypes(c *context, t1 ir.Type, t2 ir.Type) bool {
	if ir.IsUntyped(t1) || ir.IsUntyped(t2) {
		// TODO: Improve assert to check that an actual type error was reported for t1 and/or t2
		common.Assert(c.errors.Count() > 0, "t1 or t2 are untyped and no error was reported")
		return true
	}
	return t1.Equals(t2)
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
		lit.T = ir.NewBasicType(id)
	}

	return res
}

func (v *typeChecker) checkCompileTimeContant(expr ir.Expr) bool {
	switch init := expr.(type) {
	case *ir.BasicLit:
	case *ir.StructLit:
		for _, field := range init.Initializers {
			if !v.checkCompileTimeContant(field.Value) {
				return false
			}
		}
	case *ir.ArrayLit:
		for _, elem := range init.Initializers {
			if !v.checkCompileTimeContant(elem) {
				return false
			}
		}
	default:
		v.c.error(expr.FirstPos(), "'%s' is not a compile-time constant", PrintExpr(init))
		return false
	}
	return true
}
