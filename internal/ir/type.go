package ir

import (
	"bytes"
	"fmt"
	"math"
	"math/big"

	"github.com/jhnl/dingo/internal/token"
)

// TypeID identifies the base type.
type TypeID int

// Type IDs
const (
	TUntyped TypeID = iota
	TVoid
	TBool

	TUInt8
	TInt8
	TUInt16
	TInt16
	TUInt32
	TInt32
	TUInt64
	TInt64
	TFloat32
	TFloat64

	// Only used as intermediary types when evaluating constant expressions.
	TBigInt
	TBigFloat
	//

	TModule
	TStruct
	TArray
	TSlice
	TPointer
	TFunc
)

var types = [...]string{
	TUntyped:  "untyped",
	TVoid:     "void",
	TBool:     "bool",
	TUInt8:    "u8",
	TInt8:     "i8",
	TUInt16:   "u16",
	TInt16:    "i16",
	TUInt32:   "u32",
	TInt32:    "i32",
	TUInt64:   "u64",
	TInt64:    "i64",
	TFloat32:  "f32",
	TFloat64:  "f64",
	TBigInt:   "int",
	TBigFloat: "float",
	TModule:   "module",
	TStruct:   "struct",
	TArray:    "array",
	TSlice:    "slice",
	TPointer:  "pointer",
	TFunc:     "fun",
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

// CABI name.
const CABI = "c"

// Built-in types.
var (
	TBuiltinUntyped = Type(NewBasicType(TUntyped))
	TBuiltinVoid    = Type(NewBasicType(TVoid))
	TBuiltinBool    = Type(NewBasicType(TBool))
	TBuiltinUInt64  = Type(NewBasicType(TUInt64))
	TBuiltinInt64   = Type(NewBasicType(TInt64))
	TBuiltinUInt32  = Type(NewBasicType(TUInt32))
	TBuiltinInt32   = Type(NewBasicType(TInt32))
	TBuiltinUInt16  = Type(NewBasicType(TUInt16))
	TBuiltinInt16   = Type(NewBasicType(TInt16))
	TBuiltinUInt8   = Type(NewBasicType(TUInt8))
	TBuiltinInt8    = Type(NewBasicType(TInt8))
	TBuiltinFloat64 = Type(NewBasicType(TFloat64))
	TBuiltinFloat32 = Type(NewBasicType(TFloat32))
)

// Big ints and floats are used when evaluating constant expressions and checking for overflow.
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

// Type interface is implemented by all types, and is the main representation of types in the compiler.
type Type interface {
	ID() TypeID
	String() string
	// Equals and CastableTo are symmetric
	Equals(Type) bool
	CastableTo(Type) bool
}

// Target interface is implemented by the backend and used for platform dependent type information.
type Target interface {
	Sizeof(Type) int
}

type baseType struct {
	id TypeID
}

func (t *baseType) ID() TypeID {
	return t.id
}

type BasicType struct {
	baseType
}

func (t *BasicType) String() string {
	return t.id.String()
}

func (t *BasicType) Equals(other Type) bool {
	if t2, ok := other.(*BasicType); ok {
		return t.id == t2.id
	}
	return false
}

func (t *BasicType) CastableTo(other Type) bool {
	switch {
	case IsNumericType(t):
		if IsNumericType(other) {
			return true
		}
	}
	return false
}

type ModuleType struct {
	baseType
	Sym   *Symbol
	Scope *Scope
}

func (t *ModuleType) String() string {
	return t.Sym.ModFQN()
}

func (t *ModuleType) Equals(other Type) bool {
	return false
}

func (t *ModuleType) CastableTo(other Type) bool {
	return false
}

type Field struct {
	Name string
	T    Type
}

type StructType struct {
	baseType
	Sym    *Symbol
	Scope  *Scope
	Fields []Field
}

func (t *StructType) String() string {
	return t.Sym.FQN()
}

func (t *StructType) Equals(other Type) bool {
	if t2, ok := other.(*StructType); ok {
		return t.Sym == t2.Sym
	}
	return false
}

func (t *StructType) CastableTo(other Type) bool {
	return t.Equals(other)
}

func (t *StructType) Opaque() bool {
	return !t.Sym.IsDefined()
}

func (t *StructType) FieldIndex(fieldName string) int {
	for i, field := range t.Fields {
		if field.Name == fieldName {
			return i
		}
	}
	return -1
}

type ArrayType struct {
	baseType
	Elem Type
	Size int
}

func (t *ArrayType) String() string {
	return fmt.Sprintf("[%s:%d]", t.Elem.String(), t.Size)
}

func (t *ArrayType) Equals(other Type) bool {
	if t2, ok := other.(*ArrayType); ok {
		return t.Size == t2.Size && t.Elem.Equals(t2.Elem)
	}
	return false
}

func (t *ArrayType) CastableTo(other Type) bool {
	return t.Equals(other)
}

type SliceType struct {
	baseType
	Elem     Type
	ReadOnly bool // Applies to the Elem type
	Ptr      bool // SliceType is only valid once it has "absorbed" one pointer indirection
}

func (t *SliceType) String() string {
	extra := ""
	if t.Ptr {
		extra = token.Pointer.String()
		if !t.ReadOnly {
			extra += token.Var.String() + " "
		}
	}
	return fmt.Sprintf("%s[%s]", extra, t.Elem.String())
}

func (t *SliceType) Equals(other Type) bool {
	if t2, ok := other.(*SliceType); ok {
		return t.ReadOnly == t2.ReadOnly && t.Elem.Equals(t2.Elem)
	}
	return false
}

func (t *SliceType) CastableTo(other Type) bool {
	if t2, ok := other.(*SliceType); ok {
		switch {
		case t.Elem.Equals(t2.Elem):
			return true
		}
	}
	return false
}

type PointerType struct {
	baseType
	Elem     Type
	ReadOnly bool // Applies to the Base type
}

func (t *PointerType) String() string {
	extra := ""
	if !t.ReadOnly {
		extra = token.Var.String() + " "
	}
	return fmt.Sprintf("%s%s%s", token.Pointer.String(), extra, t.Elem)
}

func (t *PointerType) Equals(other Type) bool {
	if t2, ok := other.(*PointerType); ok {
		return t.ReadOnly == t2.ReadOnly && t.Elem.Equals(t2.Elem)
	}
	return false
}

func (t *PointerType) CastableTo(other Type) bool {
	if t2, ok := other.(*PointerType); ok {
		switch {
		case t.Elem.ID() == TVoid || t2.Elem.ID() == TVoid:
			return true
		case t.Elem.Equals(t2.Elem):
			return true
		}
	}
	return false
}

type FuncType struct {
	baseType
	C      bool
	Params []Field
	Return Type
}

func (t *FuncType) String() string {
	var buf bytes.Buffer
	buf.WriteString("fun")
	if t.C {
		buf.WriteString("[c]")
	}
	buf.WriteString("(")
	for i, param := range t.Params {
		buf.WriteString(param.T.String())
		if (i + 1) < len(t.Params) {
			buf.WriteString(", ")
		}
	}
	buf.WriteString(")")
	if t.Return.ID() != TVoid {
		buf.WriteString(" ")
		buf.WriteString(t.Return.String())
	}
	return buf.String()
}

func (t *FuncType) Equals(other Type) bool {
	if t2, ok := other.(*FuncType); ok {
		if t.C != t2.C {
			return false
		}
		if len(t.Params) != len(t2.Params) {
			return false
		}
		if !t.Return.Equals(t2.Return) {
			return false
		}
		for i := 0; i < len(t.Params); i++ {
			if !t.Params[i].T.Equals(t2.Params[i].T) {
				return false
			}
		}
		return true
	}
	return false
}

func (t *FuncType) CastableTo(other Type) bool {
	return t.Equals(other)
}

func NewBasicType(id TypeID) *BasicType {
	t := &BasicType{}
	t.id = id
	return t
}

func NewModuleType(sym *Symbol, scope *Scope) *ModuleType {
	t := &ModuleType{Sym: sym, Scope: scope}
	t.id = TModule
	return t
}

func NewStructType(sym *Symbol, scope *Scope) *StructType {
	t := &StructType{Sym: sym, Scope: scope}
	t.id = TStruct
	return t
}

func (t *StructType) SetBody(fields []Field) {
	t.Fields = fields
}

func NewArrayType(elem Type, size int) *ArrayType {
	t := &ArrayType{Elem: elem, Size: size}
	t.id = TArray
	return t
}

func NewSliceType(elem Type, readOnly bool, absorbedPtr bool) *SliceType {
	t := &SliceType{Elem: elem, ReadOnly: readOnly, Ptr: absorbedPtr}
	t.id = TSlice
	return t
}

func NewPointerType(elem Type, readOnly bool) *PointerType {
	t := &PointerType{Elem: elem, ReadOnly: readOnly}
	t.id = TPointer
	return t
}

func NewFuncType(params []Field, ret Type, c bool) *FuncType {
	t := &FuncType{Params: params, Return: ret, C: c}
	t.id = TFunc
	return t
}

func ToBaseType(t Type) Type {
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

func IsImplicitCastNeeded(from Type, to Type) bool {
	switch tfrom := from.(type) {
	case *SliceType:
		if tto, ok := to.(*SliceType); ok {
			if !tfrom.ReadOnly && tto.ReadOnly {
				return tfrom.Elem.Equals(tto.Elem)
			}
		}
	case *PointerType:
		if tto, ok := to.(*PointerType); ok {
			if !tfrom.ReadOnly && tto.ReadOnly {
				return (tto.Elem.ID() == TVoid) || (tfrom.Elem.Equals(tto.Elem))
			} else if tfrom.ReadOnly == tto.ReadOnly {
				return (tto.Elem.ID() == TVoid) && (tfrom.Elem.ID() != TVoid)
			}
		}
	}
	return false
}

func IsIncompleteType(t1 Type, outer Type) bool {
	incomplete := false
	switch t2 := t1.(type) {
	case *BasicType:
		if t2.ID() == TVoid {
			if outer == nil || outer.ID() != TPointer {
				incomplete = true
			}
		}
	case *StructType:
		if t2.Opaque() {
			if outer == nil || outer.ID() != TPointer {
				incomplete = true
			}
		}
	case *ArrayType:
		incomplete = IsIncompleteType(t2.Elem, t2)
	case *SliceType:
		if !t2.Ptr {
			incomplete = true
		} else {
			incomplete = IsIncompleteType(t2.Elem, t2)
		}
	case *PointerType:
		incomplete = IsIncompleteType(t2.Elem, t2)
	}
	return incomplete
}

func IsCompilerType(t1 Type) bool {
	switch t2 := t1.(type) {
	case *PointerType:
		return IsUntyped(t2.Elem)
	case *ArrayType:
		return IsCompilerType(t2.Elem)
	case *BasicType:
		return IsTypeID(t2, TUntyped, TBigInt, TBigFloat)
	default:
		return true
	}
}

func IsUntyped(t Type) bool {
	return t.ID() == TUntyped
}

func IsUntypedPointer(t Type) bool {
	if tptr, ok := t.(*PointerType); ok {
		return IsUntyped(tptr.Elem)
	}
	return false
}

func IsSignedType(t Type) bool {
	return IsTypeID(t, TBigInt, TInt64, TInt32, TInt16, TInt8)
}

func IsUnsignedType(t Type) bool {
	return IsTypeID(t, TBigInt, TUInt64, TUInt32, TUInt16, TUInt8)
}

func IsIntegerType(t Type) bool {
	return IsTypeID(t, TBigInt, TUInt64, TInt64, TUInt32, TInt32, TUInt16, TInt16, TUInt8, TInt8)
}

func IsFloatType(t Type) bool {
	return IsTypeID(t, TBigFloat, TFloat64, TFloat32)
}

func IsNumericType(t Type) bool {
	if IsIntegerType(t) || IsFloatType(t) {
		return true
	}
	return false
}
