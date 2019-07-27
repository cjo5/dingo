package ir

import (
	"bytes"
	"fmt"
	"math"
	"math/big"

	"github.com/cjo5/dingo/internal/token"
)

// TypeKind identifies the base type.
type TypeKind int

// Type kinds.
const (
	TUnknown TypeKind = iota
	TInvalid

	TVoid
	TBool
	TNull

	TConstInt
	TConstFloat

	TInt8
	TUInt8
	TInt16
	TUInt16
	TInt32
	TUInt32
	TInt64
	TUInt64
	TUSize
	TFloat32
	TFloat64

	TModule
	TStruct
	TArray
	TSlice
	TPointer
	TFunc
)

var types = [...]string{
	TUnknown:    "Unknown",
	TInvalid:    "Invalid",
	TVoid:       "Void",
	TBool:       "Bool",
	TNull:       "Null",
	TConstInt:   "UntypedInt",
	TConstFloat: "UntypedFloat",
	TInt8:       "I8",
	TUInt8:      "U8",
	TInt16:      "I16",
	TUInt16:     "U16",
	TInt32:      "I32",
	TUInt32:     "U32",
	TInt64:      "I64",
	TUInt64:     "U64",
	TUSize:      "USize",
	TFloat32:    "F32",
	TFloat64:    "F64",
	TModule:     "Module",
	TStruct:     "Struct",
	TArray:      "Array",
	TSlice:      "Slice",
	TPointer:    "Pointer",
	TFunc:       "Fun",
}

func (id TypeKind) String() string {
	s := ""
	if 0 <= id && id < TypeKind(len(types)) {
		s = types[id]
	} else {
		s = "unknown"
	}
	return s
}

// Built-in types.
var (
	TBuiltinUnknown    = Type(NewBasicType(TUnknown))
	TBuiltinInvalid    = Type(NewBasicType(TInvalid))
	TBuiltinVoid       = Type(NewBasicType(TVoid))
	TBuiltinBool       = Type(NewBasicType(TBool))
	TBuiltinConstInt   = Type(NewBasicType(TConstInt))
	TBuiltinConstFloat = Type(NewBasicType(TConstFloat))
	TBuiltinInt8       = Type(NewBasicType(TInt8))
	TBuiltinUInt8      = Type(NewBasicType(TUInt8))
	TBuiltinInt16      = Type(NewBasicType(TInt16))
	TBuiltinUInt16     = Type(NewBasicType(TUInt16))
	TBuiltinInt32      = Type(NewBasicType(TInt32))
	TBuiltinUInt32     = Type(NewBasicType(TUInt32))
	TBuiltinInt64      = Type(NewBasicType(TInt64))
	TBuiltinUInt64     = Type(NewBasicType(TUInt64))
	TBuiltinUSize      = Type(NewBasicType(TUSize))
	TBuiltinFloat32    = Type(NewBasicType(TFloat32))
	TBuiltinFloat64    = Type(NewBasicType(TFloat64))

	TBuiltinByte  = Type(NewAliasType("Byte", TBuiltinInt8))
	TBuiltinUByte = Type(NewAliasType("UByte", TBuiltinUInt8))
	TBuiltinInt   = Type(NewAliasType("Int", TBuiltinInt32))
	TBuiltinUInt  = Type(NewAliasType("UInt", TBuiltinUInt32))
	TBuiltinFloat = Type(NewAliasType("Float", TBuiltinFloat32))
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

// Type interface is implemented by all types and is the main representation of types in the compiler.
type Type interface {
	Kind() TypeKind
	String() string
	// Equals and CastableTo are symmetric
	Equals(Type) bool
	CastableTo(Type) bool
}

// Target interface is implemented by the backend and used for platform dependent type information.
type Target interface {
	Sizeof(Type) int
}

type AliasType struct {
	Name string
	T    Type
}

func (t *AliasType) Kind() TypeKind {
	return t.T.Kind()
}

func (t *AliasType) String() string {
	return fmt.Sprintf("%s(%s)", t.Name, t.T.String())
}

func (t *AliasType) Equals(other Type) bool {
	return t.T.Equals(other)
}

func (t *AliasType) CastableTo(other Type) bool {
	return t.T.CastableTo(other)
}

type baseType struct {
	kind TypeKind
}

func (t *baseType) Kind() TypeKind {
	return t.kind
}

type BasicType struct {
	baseType
}

func (t *BasicType) String() string {
	return t.kind.String()
}

func (t *BasicType) Equals(other Type) bool {
	other = ToBaseType(other)
	if t2, ok := other.(*BasicType); ok {
		return t.kind == t2.kind
	}
	return false
}

func (t *BasicType) CastableTo(other Type) bool {
	other = ToBaseType(other)
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
	return fmt.Sprintf("%s", t.Sym.ModFQN)
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
	TypedBody bool
	Sym       *Symbol
	Scope     *Scope
	Fields    []Field
}

func (t *StructType) String() string {
	return t.Sym.FQN()
}

func (t *StructType) Equals(other Type) bool {
	other = ToBaseType(other)
	if t2, ok := other.(*StructType); ok {
		return t.Sym.FQN() == t2.Sym.FQN()
	}
	return false
}

func (t *StructType) CastableTo(other Type) bool {
	other = ToBaseType(other)
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
	other = ToBaseType(other)
	if t2, ok := other.(*ArrayType); ok {
		return t.Size == t2.Size && t.Elem.Equals(t2.Elem)
	}
	return false
}

func (t *ArrayType) CastableTo(other Type) bool {
	other = ToBaseType(other)
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
		extra = token.Reference.String()
		if !t.ReadOnly {
			extra += token.Var.String() + " "
		}
	}
	return fmt.Sprintf("%s[%s]", extra, t.Elem.String())
}

func (t *SliceType) Equals(other Type) bool {
	other = ToBaseType(other)
	if t2, ok := other.(*SliceType); ok {
		return t.ReadOnly == t2.ReadOnly && t.Elem.Equals(t2.Elem)
	}
	return false
}

func (t *SliceType) CastableTo(other Type) bool {
	other = ToBaseType(other)
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
	ReadOnly bool // Applies to the Elem type
}

func (t *PointerType) String() string {
	extra := ""
	if !t.ReadOnly {
		extra = token.Var.String() + " "
	}
	return fmt.Sprintf("%s%s%s", token.Reference.String(), extra, t.Elem)
}

func (t *PointerType) Equals(other Type) bool {
	other = ToBaseType(other)
	if t2, ok := other.(*PointerType); ok {
		return t.ReadOnly == t2.ReadOnly && t.Elem.Equals(t2.Elem)
	}
	return false
}

func (t *PointerType) CastableTo(other Type) bool {
	other = ToBaseType(other)
	if t2, ok := other.(*PointerType); ok {
		switch {
		case t.Elem.Kind() == TVoid || t2.Elem.Kind() == TVoid:
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
	if t.C {
		buf.WriteString("extern ")
	}
	buf.WriteString("fun")
	buf.WriteString("(")
	for i, param := range t.Params {
		buf.WriteString(param.T.String())
		if (i + 1) < len(t.Params) {
			buf.WriteString(", ")
		}
	}
	buf.WriteString(")")
	if t.Return.Kind() != TVoid {
		buf.WriteString(" ")
		buf.WriteString(t.Return.String())
	}
	return buf.String()
}

func (t *FuncType) Equals(other Type) bool {
	other = ToBaseType(other)
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
	other = ToBaseType(other)
	return t.Equals(other)
}

func NewAliasType(name string, base Type) *AliasType {
	t := &AliasType{}
	t.Name = name
	t.T = base
	return t
}

func NewBasicType(kind TypeKind) *BasicType {
	t := &BasicType{}
	t.kind = kind
	return t
}

func NewModuleType(sym *Symbol, scope *Scope) *ModuleType {
	t := &ModuleType{Sym: sym, Scope: scope}
	t.kind = TModule
	return t
}

func NewStructType(sym *Symbol, scope *Scope) *StructType {
	t := &StructType{Sym: sym, Scope: scope}
	t.kind = TStruct
	return t
}

func (t *StructType) SetBody(fields []Field, typedBody bool) {
	t.Fields = fields
	t.TypedBody = typedBody
}

func NewArrayType(elem Type, size int) *ArrayType {
	t := &ArrayType{Elem: elem, Size: size}
	t.kind = TArray
	return t
}

func NewSliceType(elem Type, readOnly bool, absorbedPtr bool) *SliceType {
	t := &SliceType{Elem: elem, ReadOnly: readOnly, Ptr: absorbedPtr}
	t.kind = TSlice
	return t
}

func NewPointerType(elem Type, readOnly bool) *PointerType {
	t := &PointerType{Elem: elem, ReadOnly: readOnly}
	t.kind = TPointer
	return t
}

func NewFuncType(params []Field, ret Type, c bool) *FuncType {
	t := &FuncType{Params: params, Return: ret, C: c}
	t.kind = TFunc
	return t
}

func ToBaseType(t Type) Type {
	base := t
	for {
		if alias, ok := base.(*AliasType); ok {
			base = alias.T
		} else {
			break
		}
	}
	return base
}

func IsSignedType(t Type) bool {
	switch t.Kind() {
	case TConstInt, TInt64, TInt32, TInt16, TInt8:
		return true
	}
	return false
}

func IsUnsignedType(t Type) bool {
	switch t.Kind() {
	case TUSize, TUInt64, TUInt32, TUInt16, TUInt8:
		return true
	}
	return false
}

func IsIntegerType(t Type) bool {
	switch t.Kind() {
	case TConstInt, TUSize, TUInt64, TInt64, TUInt32, TInt32, TUInt16, TInt16, TUInt8, TInt8:
		return true
	}
	return false
}

func IsFloatType(t Type) bool {
	switch t.Kind() {
	case TConstFloat, TFloat64, TFloat32:
		return true
	}
	return false
}

func IsNumericType(t Type) bool {
	if IsIntegerType(t) || IsFloatType(t) {
		return true
	}
	return false
}
