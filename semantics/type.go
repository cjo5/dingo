package semantics

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
)

// TypeID identifies the base type.
type TypeID int

// Type IDs
const (
	TUntyped TypeID = iota
	TVoid
	TBool
	TString
	TModule
	TStruct
	TFunc

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
	TFunc:     "func",
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

func (id TypeID) String() string {
	s := ""
	if 0 <= id && id < TypeID(len(types)) {
		s = types[id]
	} else {
		s = "unknown"
	}
	return s
}

// Built-in types
var (
	TBuiltinUntyped = NewBasicType(TUntyped)
	TBuiltinVoid    = NewBasicType(TVoid)
	TBuiltinBool    = NewBasicType(TBool)
	TBuiltinString  = NewBasicType(TString)
	TBuiltinUInt64  = NewBasicType(TUInt64)
	TBuiltinInt64   = NewBasicType(TInt64)
	TBuiltinUInt32  = NewBasicType(TUInt32)
	TBuiltinInt32   = NewBasicType(TInt32)
	TBuiltinUInt16  = NewBasicType(TUInt16)
	TBuiltinInt16   = NewBasicType(TInt16)
	TBuiltinUInt8   = NewBasicType(TUInt8)
	TBuiltinInt8    = NewBasicType(TInt8)
	TBuiltinFloat64 = NewBasicType(TFloat64)
	TBuiltinFloat32 = NewBasicType(TFloat32)
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

type Type interface {
	ID() TypeID
	IsEqual(Type) bool
	String() string
}

type BasicType struct {
	id TypeID
}

func (t *BasicType) ID() TypeID {
	return t.id
}

func (t *BasicType) IsEqual(other Type) bool {
	return t.id == other.ID()
}

func (t *BasicType) String() string {
	return t.id.String()
}

type ModuleType struct {
	ModuleID int
	Scope    *Scope
}

func (t *ModuleType) ID() TypeID {
	return TModule
}

func (t *ModuleType) IsEqual(other Type) bool {
	if t2, ok := other.(*ModuleType); ok {
		return t.ModuleID == t2.ModuleID
	}
	return false
}

func (t *ModuleType) String() string {
	return fmt.Sprintf("module %d", t.ModuleID)
}

type Field struct {
	Sym *Symbol
	T   Type
}

func (f *Field) Name() string {
	if f.Sym != nil {
		return f.Sym.Name
	}
	return "<anon>"
}

type StructType struct {
	Sym    *Symbol
	Scope  *Scope
	Fields []*Field
}

func (t *StructType) ID() TypeID {
	return TStruct
}

func (t *StructType) IsEqual(other Type) bool {
	if t2, ok := other.(*StructType); ok {
		return t.Sym == t2.Sym
	}
	return false
}

func (t *StructType) String() string {
	return fmt.Sprintf("struct %s", t.Name())
}

func (t *StructType) Name() string {
	if t.Sym != nil {
		return t.Sym.Name
	}
	return "<anon>"
}

type FuncType struct {
	Sym    *Symbol
	Params []*Field
	Return Type
}

func (t *FuncType) ID() TypeID {
	return TFunc
}

func (t *FuncType) IsEqual(other Type) bool {
	if t2, ok := other.(*FuncType); ok {
		if len(t.Params) != len(t2.Params) {
			return false
		}
		if !t.Return.IsEqual(t2.Return) {
			return false
		}
		for i := 0; i < len(t.Params); i++ {
			if !t.Params[i].T.IsEqual(t2.Params[i].T) {
				return false
			}
		}
		return true
	}
	return false
}

func (t *FuncType) String() string {
	var buf bytes.Buffer
	buf.WriteString("(")
	for i, p := range t.Params {
		buf.WriteString(p.T.String())
		if (i + 1) < len(t.Params) {
			buf.WriteString(", ")
		}
	}
	buf.WriteString(fmt.Sprintf(") -> %s", t.Return))
	return buf.String()
}

func NewBasicType(id TypeID) Type {
	return &BasicType{id: id}
}

func NewModuleType(moduleID int, scope *Scope) Type {
	return &ModuleType{ModuleID: moduleID, Scope: scope}
}

func NewStructType(decl *StructDecl) Type {
	t := &StructType{Sym: decl.Sym, Scope: decl.Scope}
	for _, field := range decl.Fields {
		t.Fields = append(t.Fields, &Field{Sym: field.Sym, T: field.Type.Type()})
	}
	return t
}

func NewFuncType(decl *FuncDecl) Type {
	t := &FuncType{Sym: decl.Sym}
	for _, param := range decl.Params {
		t.Params = append(t.Params, &Field{Sym: param.Sym, T: param.Type.Type()})
	}
	t.Return = decl.TReturn.Type()
	return t
}

func IsTypeID(t Type, ids ...TypeID) bool {
	for _, id := range ids {
		if t.ID() == id {
			return true
		}
	}
	return false
}

func IsUntyped(t Type) bool {
	return t.ID() == TUntyped
}

func IsNumericType(t Type) bool {
	switch t.ID() {
	case TBigInt, TBigFloat, TUInt64, TInt64, TUInt32, TInt32, TUInt16, TInt16, TUInt8, TInt8, TFloat64, TFloat32:
		return true
	default:
		return false
	}
}

func compatibleTypes(from Type, to Type) bool {
	switch {
	case from.IsEqual(to):
		return true
	case IsNumericType(from):
		switch {
		case IsNumericType(to):
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

func typeCastNumericLiteral(lit *BasicLit, target Type) numericCastResult {
	res := numericCastOK
	id := target.ID()

	switch t := lit.Raw.(type) {
	case *big.Int:
		switch id {
		case TBigInt, TUInt64, TUInt32, TUInt16, TUInt8, TInt64, TInt32, TInt16, TInt8:
			if integerOverflows(t, id) {
				res = numericCastOverflows
			}
		case TBigFloat, TFloat64, TFloat32:
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
		case TBigInt, TUInt64, TUInt32, TUInt16, TUInt8, TInt64, TInt32, TInt16, TInt8:
			if ival := toBigInt(t); ival != nil {
				if integerOverflows(ival, id) {
					res = numericCastOverflows
				} else {
					lit.Raw = ival
				}
			} else {
				res = numericCastTruncated
			}
		case TBigFloat, TFloat64, TFloat32:
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
		lit.T = NewBasicType(id)
	}

	return res
}
