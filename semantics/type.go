package semantics

import (
	"math"
	"math/big"
)

type TypeID int

// Type IDs
const (
	TUntyped TypeID = iota
	TVoid
	TBool
	TString
	TModule
	TStruct

	// Only used as intermediary types when evaluating constant expressions.
	TBigInt
	TBigFloat
	//

	TUInt64
	TInt64
	TUInt32
	TInt32
	TUInt16
	TInt16
	TUInt8
	TInt8
	TFloat64
	TFloat32
)

var types = [...]string{
	TUntyped:  "untyped",
	TVoid:     "void",
	TBool:     "bool",
	TString:   "str",
	TModule:   "module",
	TStruct:   "struct",
	TBigInt:   "integer",
	TBigFloat: "float",
	TUInt64:   "u64",
	TInt64:    "i64",
	TUInt32:   "u32",
	TInt32:    "i32",
	TUInt16:   "u16",
	TInt16:    "i16",
	TUInt8:    "u8",
	TInt8:     "i8",
	TFloat64:  "f64",
	TFloat32:  "f32",
}

// Built-in types
var (
	TBuiltinUntyped = NewTypeFromID(TUntyped)
	TBuiltinVoid    = NewTypeFromID(TVoid)
	TBuiltinBool    = NewTypeFromID(TBool)
	TBuiltinString  = NewTypeFromID(TString)
	TBuiltinModule  = NewTypeFromID(TModule)
	TBuiltinUInt64  = NewTypeFromID(TUInt64)
	TBuiltinInt64   = NewTypeFromID(TInt64)
	TBuiltinUInt32  = NewTypeFromID(TUInt32)
	TBuiltinInt32   = NewTypeFromID(TInt32)
	TBuiltinUInt16  = NewTypeFromID(TUInt16)
	TBuiltinInt16   = NewTypeFromID(TInt16)
	TBuiltinUInt8   = NewTypeFromID(TUInt8)
	TBuiltinInt8    = NewTypeFromID(TInt8)
	TBuiltinFloat64 = NewTypeFromID(TFloat64)
	TBuiltinFloat32 = NewTypeFromID(TFloat32)
)

// Big ints used when evaluating constant expressions and checking for overflow.
var (
	MaxU64 = big.NewInt(0).SetUint64(math.MaxUint64)
	MaxU32 = big.NewInt(math.MaxUint32)
	MaxU16 = big.NewInt(math.MaxUint16)
	MaxU8  = big.NewInt(math.MaxUint8)

	MaxI64 = big.NewInt(math.MaxInt64)
	MinI64 = big.NewInt(math.MinInt64)
	MaxI32 = big.NewInt(math.MaxInt32)
	MinI32 = big.NewInt(math.MinInt32)
	MaxI16 = big.NewInt(math.MaxInt16)
	MinI16 = big.NewInt(math.MinInt16)
	MaxI8  = big.NewInt(math.MaxInt8)
	MinI8  = big.NewInt(math.MinInt8)

	MaxF64 = big.NewFloat(math.MaxFloat64)
	MinF64 = big.NewFloat(-math.MaxFloat64)
	MaxF32 = big.NewFloat(math.MaxFloat32)
	MinF32 = big.NewFloat(-math.MaxFloat32)

	BigIntZero   = big.NewInt(0)
	BigFloatZero = big.NewFloat(0)
)

type TType struct {
	ID   TypeID
	Name string
}

func NewType(id TypeID, name string) *TType {
	return &TType{ID: id, Name: name}
}

func NewTypeFromID(id TypeID) *TType {
	return NewType(id, id.String())
}

func (t *TType) OneOf(ids ...TypeID) bool {
	for _, id := range ids {
		if t.ID == id {
			return true
		}
	}
	return false
}

func (t *TType) IsNumericType() bool {
	switch t.ID {
	case TBigInt, TBigFloat, TUInt64, TInt64, TUInt32, TInt32, TUInt16, TInt16, TUInt8, TInt8, TFloat64, TFloat32:
		return true
	default:
		return false
	}
}

func (t *TType) IsEqual(other *TType) bool {
	if t.ID != other.ID {
		return false
	}
	return t.Name == other.Name
}

func (t *TType) String() string {
	if t.ID == TModule {
		return "module " + t.Name
	} else if t.ID == TStruct {
		return "struct " + t.Name
	}
	return t.Name
}

func (id TypeID) String() string {
	s := ""
	if 0 <= id && id < TypeID(len(types)) {
		s = types[id]
	} else {
		s = "unknown"
	}
	return s
}

func compatibleTypes(from *TType, to *TType) bool {
	switch {
	case from.IsEqual(to):
		return true
	case from.IsNumericType():
		switch {
		case to.IsNumericType():
			return true
		default:
			return false
		}
	default:
		return false
	}
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

func integerOverflows(val *big.Int, t TypeID) bool {
	fits := true

	switch t {
	case TBigInt:
		// OK
	case TUInt64:
		fits = 0 <= val.Cmp(BigIntZero) && val.Cmp(MaxU64) <= 0
	case TUInt32:
		fits = 0 <= val.Cmp(BigIntZero) && val.Cmp(MaxU32) <= 0
	case TUInt16:
		fits = 0 <= val.Cmp(BigIntZero) && val.Cmp(MaxU16) <= 0
	case TUInt8:
		fits = 0 <= val.Cmp(BigIntZero) && val.Cmp(MaxU8) <= 0
	case TInt64:
		fits = 0 <= val.Cmp(MinI64) && val.Cmp(MaxI64) <= 0
	case TInt32:
		fits = 0 <= val.Cmp(MinI32) && val.Cmp(MaxI32) <= 0
	case TInt16:
		fits = 0 <= val.Cmp(MinI16) && val.Cmp(MaxI16) <= 0
	case TInt8:
		fits = 0 <= val.Cmp(MinI8) && val.Cmp(MaxI8) <= 0
	}

	return !fits
}

func floatOverflows(val *big.Float, t TypeID) bool {
	fits := true

	switch t {
	case TBigFloat:
		// OK
	case TFloat64:
		fits = 0 <= val.Cmp(MinF64) && val.Cmp(MaxF64) <= 0
	case TFloat32:
		fits = 0 <= val.Cmp(MinF32) && val.Cmp(MaxF32) <= 0
	}

	return !fits
}

func typeCastNumericLiteral(lit *Literal, target *TType) numericCastResult {
	res := numericCastOK

	switch t := lit.Raw.(type) {
	case *big.Int:
		switch target.ID {
		case TBigInt, TUInt64, TUInt32, TUInt16, TUInt8, TInt64, TInt32, TInt16, TInt8:
			if integerOverflows(t, target.ID) {
				res = numericCastOverflows
			}
		case TBigFloat, TFloat64, TFloat32:
			fval := toBigFloat(t)
			if floatOverflows(fval, target.ID) {
				res = numericCastOverflows
			} else {
				lit.Raw = fval
			}
		default:
			return numericCastFails
		}
	case *big.Float:
		switch target.ID {
		case TBigInt, TUInt64, TUInt32, TUInt16, TUInt8, TInt64, TInt32, TInt16, TInt8:
			if ival := toBigInt(t); ival != nil {
				if integerOverflows(ival, target.ID) {
					res = numericCastOverflows
				} else {
					lit.Raw = ival
				}
			} else {
				res = numericCastTruncated
			}
		case TBigFloat, TFloat64, TFloat32:
			if floatOverflows(t, target.ID) {
				res = numericCastOverflows
			}
		default:
			return numericCastFails
		}
	default:
		return numericCastFails
	}

	if res == numericCastOK {
		lit.T = NewTypeFromID(target.ID)
	}

	return res
}
