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
	TBigInt // Only used as an intermediary type when evaluating constant expressions.
	TUInt64
	TInt64
	TUInt32
	TInt32
	TUInt16
	TInt16
	TUInt8
	TInt8
)

var types = [...]string{
	TUntyped: "untyped",
	TVoid:    "void",
	TBool:    "bool",
	TString:  "str",
	TBigInt:  "integer",
	TUInt64:  "u64",
	TInt64:   "i64",
	TUInt32:  "u32",
	TInt32:   "i32",
	TUInt16:  "u16",
	TInt16:   "i16",
	TUInt8:   "u8",
	TInt8:    "i8",
}

// Built-in types
var (
	TBuiltinUntyped = NewType(TUntyped)
	TBuiltinVoid    = NewType(TVoid)
	TBuiltinBool    = NewType(TBool)
	TBuiltinString  = NewType(TString)
	TBuiltinUInt64  = NewType(TUInt64)
	TBuiltinInt64   = NewType(TInt64)
	TBuiltinUInt32  = NewType(TUInt32)
	TBuiltinInt32   = NewType(TInt32)
	TBuiltinUInt16  = NewType(TUInt16)
	TBuiltinInt16   = NewType(TInt16)
	TBuiltinUInt8   = NewType(TUInt8)
	TBuiltinInt8    = NewType(TInt8)
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

	BigZero = big.NewInt(0)
)

type TType struct {
	ID TypeID
}

func NewType(id TypeID) *TType {
	return &TType{ID: id}
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
	case TBigInt, TUInt64, TInt64, TUInt32, TInt32, TUInt16, TInt16, TUInt8, TInt8:
		return true
	default:
		return false
	}
}

func CompatibleTypes(a *TType, b *TType) bool {
	switch {
	case a.IsNumericType():
		switch {
		case b.IsNumericType():
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func CompatibleNumericType(val *big.Int, dst *TType) bool {
	switch dst.ID {
	case TBigInt:
		return true
	case TUInt64:
		return 0 <= val.Cmp(BigZero) && val.Cmp(MaxU64) <= 0
	case TUInt32:
		return 0 <= val.Cmp(BigZero) && val.Cmp(MaxU32) <= 0
	case TUInt16:
		return 0 <= val.Cmp(BigZero) && val.Cmp(MaxU16) <= 0
	case TUInt8:
		return 0 <= val.Cmp(BigZero) && val.Cmp(MaxU8) <= 0
	case TInt64:
		return 0 <= val.Cmp(MinI64) && val.Cmp(MaxI64) <= 0
	case TInt32:
		return 0 <= val.Cmp(MinI32) && val.Cmp(MaxI32) <= 0
	case TInt16:
		return 0 <= val.Cmp(MinI16) && val.Cmp(MaxI16) <= 0
	case TInt8:
		return 0 <= val.Cmp(MinI8) && val.Cmp(MaxI8) <= 0
	default:
		return false
	}
}

func (t *TType) String() string {
	return t.ID.String()
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
