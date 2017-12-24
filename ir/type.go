package ir

import (
	"bytes"
	"fmt"
	"math"
	"math/big"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/token"
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

	TString
	TStruct
	TArray
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
	TString:   "str",
	TStruct:   "struct",
	TArray:    "array",
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
	Equals(Type) bool
	String() string
}

type BasicType struct {
	id TypeID
}

func (t *BasicType) ID() TypeID {
	return t.id
}

func (t *BasicType) Equals(other Type) bool {
	return t.id == other.ID()
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

type StructType struct {
	Sym    *Symbol
	Scope  *Scope
	Fields []*Field
}

func (t *StructType) ID() TypeID {
	return TStruct
}

func (t *StructType) Equals(other Type) bool {
	if t2, ok := other.(*StructType); ok {
		return t.Sym == t2.Sym
	}
	return false
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
	Size  int
	Elem  Type
	Scope *Scope
}

func (t *ArrayType) ID() TypeID {
	return TArray
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

func (t *ArrayType) String() string {
	return fmt.Sprintf("[%d]%s", t.Size, t.Elem.String())
}

type PointerType struct {
	Underlying Type
	ReadOnly   bool // Applies to the Underlying type
}

func (t *PointerType) ID() TypeID {
	return TPointer
}

func (t *PointerType) Equals(other Type) bool {
	otherPointer, ok := other.(*PointerType)
	if !ok {
		return false
	}
	return t.ReadOnly == otherPointer.ReadOnly && t.Underlying.Equals(otherPointer.Underlying)
}

func (t *PointerType) String() string {
	ro := ""
	if t.ReadOnly {
		ro = token.Val.String() + " "
	}
	return "*" + ro + t.Underlying.String()
}

type FuncType struct {
	Sym    *Symbol
	Params []*Field
	Return Type
}

func (t *FuncType) ID() TypeID {
	return TFunc
}

func (t *FuncType) Equals(other Type) bool {
	if t2, ok := other.(*FuncType); ok {
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

func NewIncompleteStructType(decl *StructDecl) Type {
	t := &StructType{Sym: decl.Sym, Scope: decl.Scope}
	return t
}

func (t *StructType) SetBody(decl *StructDecl) Type {
	common.Assert(t.Sym == decl.Sym, "Different struct decl")
	for _, field := range decl.Fields {
		t.Fields = append(t.Fields, &Field{Sym: field.Sym, T: field.Type.Type()})
	}
	return t
}

func NewArrayType(size int, elem Type) Type {
	scope := NewScope(FieldScope, nil)
	len := NewSymbol(ValSymbol, FieldScope, "len", token.NoPosition, nil)
	len.Flags |= SymFlagReadOnly
	len.T = NewBasicType(TInt32)
	scope.Insert(len)

	return &ArrayType{Size: size, Elem: elem, Scope: scope}
}

func NewPointerType(Underlying Type, readOnly bool) Type {
	return &PointerType{Underlying: Underlying, ReadOnly: readOnly}
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

func IsActualType(t1 Type) bool {
	switch t2 := t1.(type) {
	case *PointerType:
		return IsActualType(t2.Underlying)
	case *ArrayType:
		return IsActualType(t2.Elem)
	case *BasicType:
		return !IsTypeID(t2, TUntyped, TBigInt, TBigFloat)
	default:
		return true
	}
}

func IsUntyped(t Type) bool {
	return t.ID() == TUntyped
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

func IsFloatingType(t Type) bool {
	switch t.ID() {
	case TBigFloat, TFloat64, TFloat32:
		return true
	default:
		return false
	}
}

func IsNumericType(t Type) bool {
	if IsIntegerType(t) || IsFloatingType(t) {
		return true
	}
	return false
}

func CompatibleTypes(from Type, to Type) bool {
	switch {
	case from.Equals(to):
		return true
	case from.ID() == TPointer && to.ID() == TPointer:
		t1 := from.(*PointerType)
		t2 := to.(*PointerType)
		return t1.Underlying.Equals(t2.Underlying)
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

func CompareBitSize(t1 Type, t2 Type) int {
	// TODO: Handle incompatible types
	return int(t1.ID() - t1.ID())
}
