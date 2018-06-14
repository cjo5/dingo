package ir

import (
	"bytes"
	"fmt"
	"math"
	"math/big"

	"github.com/jhnl/dingo/internal/common"
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

// Built-in types
var (
	TBuiltinUntyped = NewBasicType(TUntyped)
	TBuiltinVoid    = NewBasicType(TVoid)
	TBuiltinBool    = NewBasicType(TBool)
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
	Equals(Type) bool
	ImplicitCastOK(Type) bool
	ExplicitCastOK(Type) bool
	String() string
}

type Target interface {
	Sizeof(Type) int
}

type baseType struct {
	id TypeID
}

func (t *baseType) ID() TypeID {
	return t.id
}

func (t *baseType) ImplicitCastOK(other Type) bool {
	return false
}

type BasicType struct {
	baseType
}

func (t *BasicType) Equals(other Type) bool {
	return t.id == other.ID()
}

func (t *BasicType) ExplicitCastOK(other Type) bool {
	switch {
	case t.Equals(other):
		return true
	case IsNumericType(t):
		if IsNumericType(other) {
			return true
		}
	}

	return false
}

func (t *BasicType) String() string {
	return t.id.String()
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

type ModuleType struct {
	baseType
	FQN   string
	Scope *Scope
}

func (t *ModuleType) Equals(other Type) bool {
	return false
}

func (t *ModuleType) ExplicitCastOK(other Type) bool {
	return false
}

func (t *ModuleType) String() string {
	return fmt.Sprintf("%s", t.FQN)
}

type StructType struct {
	baseType
	Sym    *Symbol
	Scope  *Scope
	Fields []Field
}

func (t *StructType) Equals(other Type) bool {
	if t2, ok := other.(*StructType); ok {
		return t.Sym == t2.Sym
	}
	return false
}

func (t *StructType) ExplicitCastOK(other Type) bool {
	return t.Equals(other)
}

func (t *StructType) String() string {
	return fmt.Sprintf("%s", t.Name())
}

func (t *StructType) Name() string {
	if t.Sym != nil {
		return t.Sym.Name
	}
	return "<anon>"
}

func (t *StructType) FieldIndex(fieldName string) int {
	for i, field := range t.Fields {
		if field.Sym.Name == fieldName {
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

func (t *ArrayType) Equals(other Type) bool {
	otherArray, ok := other.(*ArrayType)
	if !ok {
		return false
	}
	if t.Size != otherArray.Size {
		return false
	}

	return t.Elem.Equals(otherArray.Elem)
}

func (t *ArrayType) ExplicitCastOK(other Type) bool {
	return t.Equals(other)
}

func (t *ArrayType) String() string {
	return fmt.Sprintf("[%s:%d]", t.Elem.String(), t.Size)
}

type SliceType struct {
	baseType
	Elem     Type
	ReadOnly bool // Applies to the Elem type
	Ptr      bool // SliceType is only valid once it has "absorbed" one pointer indirection
}

func (t *SliceType) Equals(other Type) bool {
	otherSlice, ok := other.(*SliceType)
	if !ok {
		return false
	}

	return t.ReadOnly == otherSlice.ReadOnly && t.Elem.Equals(otherSlice.Elem)
}

func (t *SliceType) ImplicitCastOK(other Type) bool {
	if otherSlice, ok := other.(*SliceType); ok {
		switch {
		case t.Equals(otherSlice):
			return true
		case !t.ReadOnly && t.Elem.Equals(otherSlice.Elem):
			return true
		}
	}
	return false
}

func (t *SliceType) ExplicitCastOK(other Type) bool {
	if otherSlice, ok := other.(*SliceType); ok {
		switch {
		case t.Equals(otherSlice):
			return true
		case t.Elem.Equals(otherSlice.Elem):
			return true
		}
	}
	return false
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

type PointerType struct {
	baseType
	Underlying Type
	ReadOnly   bool // Applies to the Underlying type
}

func (t *PointerType) Equals(other Type) bool {
	if otherPtr, ok := other.(*PointerType); ok {
		return t.ReadOnly == otherPtr.ReadOnly && t.Underlying.Equals(otherPtr.Underlying)
	}
	return false
}

func (t *PointerType) ImplicitCastOK(to Type) bool {
	if toPtr, ok := to.(*PointerType); ok {
		switch {
		case t.Equals(toPtr):
			return true
		case !t.ReadOnly || toPtr.ReadOnly:
			if toPtr.Underlying.ID() == TVoid || t.Underlying.Equals(toPtr.Underlying) {
				return true
			}
		}
	}
	return false
}

func (t *PointerType) ExplicitCastOK(to Type) bool {
	if toPtr, ok := to.(*PointerType); ok {
		switch {
		case t.Equals(toPtr):
			return true
		case t.Underlying.ID() == TVoid || toPtr.Underlying.ID() == TVoid:
			return true
		case t.Underlying.Equals(toPtr.Underlying):
			return true
		}
	}
	return false
}

func (t *PointerType) String() string {
	extra := ""
	if !t.ReadOnly {
		extra = token.Var.String() + " "
	}
	return fmt.Sprintf("%s%s%s", token.Pointer.String(), extra, t.Underlying)
}

type FuncType struct {
	baseType
	C      bool
	Params []Field
	Return Type
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

func (t *FuncType) ExplicitCastOK(other Type) bool {
	return t.Equals(other)
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

func NewBasicType(id TypeID) Type {
	t := &BasicType{}
	t.id = id
	return t
}

func NewModuleType(fqn string, scope *Scope) Type {
	t := &ModuleType{FQN: fqn, Scope: scope}
	t.id = TModule
	return t
}

func NewIncompleteStructType(decl *StructDecl) Type {
	t := &StructType{Sym: decl.Sym, Scope: decl.Scope}
	t.id = TStruct
	return t
}

func (t *StructType) SetBody(decl *StructDecl) Type {
	common.Assert(t.Sym == decl.Sym, "Different struct decl")
	for _, field := range decl.Fields {
		t.Fields = append(t.Fields, Field{Sym: field.Sym, T: field.Type.Type()})
	}
	return t
}

func NewArrayType(elem Type, size int) Type {
	t := &ArrayType{Size: size, Elem: elem}
	t.id = TArray
	return t
}

func NewSliceType(elem Type, readOnly bool, absorbedPtr bool) Type {
	t := &SliceType{Elem: elem, ReadOnly: readOnly, Ptr: absorbedPtr}
	t.id = TSlice
	return t
}

func NewPointerType(Underlying Type, readOnly bool) Type {
	t := &PointerType{Underlying: Underlying, ReadOnly: readOnly}
	t.id = TPointer
	return t
}

func NewFuncType(params []Field, ret Type, c bool) Type {
	t := &FuncType{Params: params, Return: ret, C: c}
	t.id = TFunc
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

func IsIncompleteType(t1 Type, outer Type) bool {
	incomplete := false
	switch t2 := t1.(type) {
	case *BasicType:
		if t2.ID() == TVoid {
			if outer == nil || outer.ID() != TPointer {
				incomplete = true
			}
		}
	case *SliceType:
		if !t2.Ptr {
			incomplete = true
		} else {
			incomplete = IsIncompleteType(t2.Elem, t2)
		}
	case *ArrayType:
		incomplete = IsIncompleteType(t2.Elem, t2)
	case *PointerType:
		incomplete = IsIncompleteType(t2.Underlying, t2)
	}
	return incomplete
}

func IsCompilerType(t1 Type) bool {
	switch t2 := t1.(type) {
	case *PointerType:
		return IsUntyped(t2.Underlying)
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
		return IsUntyped(tptr.Underlying)
	}
	return false
}

func IsSignedType(t Type) bool {
	switch t.ID() {
	case TBigInt, TInt64, TInt32, TInt16, TInt8:
		return true
	default:
		return false
	}
}

func IsUnsignedType(t Type) bool {
	switch t.ID() {
	case TBigInt, TUInt64, TUInt32, TUInt16, TUInt8:
		return true
	default:
		return false
	}
}

func IsIntegerType(t Type) bool {
	switch t.ID() {
	case TBigInt, TUInt64, TInt64, TUInt32, TInt32, TUInt16, TInt16, TUInt8, TInt8:
		return true
	default:
		return false
	}
}

func IsFloatType(t Type) bool {
	switch t.ID() {
	case TBigFloat, TFloat64, TFloat32:
		return true
	default:
		return false
	}
}

func IsNumericType(t Type) bool {
	if IsIntegerType(t) || IsFloatType(t) {
		return true
	}
	return false
}
