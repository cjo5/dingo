package vm

import (
	"fmt"
	"strconv"

	"github.com/jhnl/interpreter/semantics"
)

type Opcode int

type Instruction struct {
	Op   Opcode
	Arg1 int64
}

// List of opcodes.
const (
	opArg0Start Opcode = iota

	Nop
	Halt
	Dup
	Pop
	Ret
	Print

	Not

	U64Add
	U64Sub
	U64Mul
	U64Div
	U64Mod
	U64Cmp

	I64Add
	I64Mul
	I64Sub
	I64Div
	I64Mod
	I64Cmp

	U32Add
	U32Sub
	U32Mul
	U32Div
	U32Mod
	U32Cmp

	I32Add
	I32Sub
	I32Mul
	I32Div
	I32Mod
	I32Cmp

	U16Add
	U16Sub
	U16Mul
	U16Div
	U16Mod
	U16Cmp

	I16Add
	I16Sub
	I16Mul
	I16Div
	I16Mod
	I16Cmp

	U8Add
	U8Sub
	U8Mul
	U8Div
	U8Mod
	U8Cmp

	I8Add
	I8Sub
	I8Mul
	I8Div
	I8Mod
	I8Cmp

	F64Add
	F64Sub
	F64Mul
	F64Div
	F64Cmp

	F32Add
	F32Sub
	F32Mul
	F32Div
	F32Cmp

	BoolCmp

	CmpEq
	CmpNe
	CmpGt
	CmpGe
	CmpLt
	CmpLe

	NumCast   // Numeric cast. Cast operand1 to type of operand2.
	NewStruct // Create new struct

	opArg0End

	opArg1Start
	U64Load     // Push immediate u64
	U32Load     // Push immediate u32
	U16Load     // Push immediate u16
	U8Load      // Push immediate u8
	I64Load     // Push immediate i64
	I32Load     // Push immediate i32
	I16Load     // Push immediate i16
	I8Load      // Push immediate i8
	BoolLoad    // Push boolean
	ConstLoad   // Push constant
	GlobalLoad  // Push global value
	GlobalStore // Pop and store in global variable
	FieldLoad   // Push field
	FieldStore  // Pop and store in field
	Load        // Push local variable
	Store       // Pop local variable
	SetMod      // Set current mod

	// Branch opcodes
	Goto
	IfTrue  // Branch if true
	IfFalse // Branch if false
	//

	Call // Call function

	opArg1End
)

var mnemonics = [...]string{
	Nop:   "nop",
	Halt:  "halt",
	Dup:   "dup",
	Pop:   "pop",
	Ret:   "ret",
	Print: "print",

	Not: "not",

	U64Add: "u64add",
	U64Sub: "u64sub",
	U64Mul: "u64mul",
	U64Div: "u64div",
	U64Mod: "u64mod",
	U64Cmp: "u64cmp",

	I64Add: "i64add",
	I64Sub: "i64sub",
	I64Mul: "i64mul",
	I64Div: "i64div",
	I64Mod: "i64mod",
	I64Cmp: "i64cmp",

	U32Add: "u32add",
	U32Sub: "u32sub",
	U32Mul: "u32mul",
	U32Div: "u32div",
	U32Mod: "u32mod",
	U32Cmp: "u32cmp",

	I32Add: "i32add",
	I32Sub: "i32sub",
	I32Mul: "i32mul",
	I32Div: "i32div",
	I32Mod: "i32mod",
	I32Cmp: "i32cmp",

	U16Add: "u16add",
	U16Sub: "u16sub",
	U16Mul: "u16mul",
	U16Div: "u16div",
	U16Mod: "u16mod",
	U16Cmp: "u16cmp",

	I16Add: "i16add",
	I16Sub: "i16sub",
	I16Mul: "i16mul",
	I16Div: "i16div",
	I16Mod: "i16mod",
	I16Cmp: "i16cmp",

	U8Add: "u8add",
	U8Sub: "u8sub",
	U8Mul: "u8mul",
	U8Div: "u8div",
	U8Mod: "u8mod",
	U8Cmp: "u8cmp",

	I8Add: "i8add",
	I8Sub: "i8sub",
	I8Mul: "i8mul",
	I8Div: "i8div",
	I8Mod: "i8mod",
	I8Cmp: "i8cmp",

	F64Add: "f64add",
	F64Sub: "f64sub",
	F64Mul: "f64mul",
	F64Div: "f64div",
	F64Cmp: "f64cmp",

	F32Add: "f32add",
	F32Sub: "f32sub",
	F32Mul: "f32mul",
	F32Div: "f32div",
	F32Cmp: "f32cmp",

	BoolCmp: "boolcmp",

	CmpEq: "cmpeq",
	CmpNe: "cmpne",
	CmpGt: "cmpgt",
	CmpLt: "cmplt",
	CmpLe: "cmple",

	NumCast:   "numcast",
	NewStruct: "newstruct",

	U64Load:  "u64load",
	U32Load:  "u32load",
	U16Load:  "u16load",
	U8Load:   "u8load",
	I64Load:  "i64load",
	I32Load:  "i32load",
	I16Load:  "i16load",
	I8Load:   "i8load",
	BoolLoad: "boolload",

	ConstLoad:   "constload",
	GlobalLoad:  "globalload",
	GlobalStore: "globalstore",
	FieldLoad:   "fieldload",
	FieldStore:  "fieldstore",
	Load:        "load",
	Store:       "store",
	SetMod:      "setmod",

	Goto:    "goto",
	IfTrue:  "iftrue",
	IfFalse: "iffalse",

	Call: "call",
}

func (op Opcode) String() string {
	s := ""
	if 0 <= op && op < Opcode(len(mnemonics)) {
		s = mnemonics[op]
	}
	if s == "" {
		s = "opcode(" + strconv.Itoa(int(op)) + ")"
	}
	return s
}

// ArgCount returns the number of arguments for the operation.
func (op Opcode) ArgCount() int {
	if opArg1Start <= op && op <= opArg1End {
		return 1
	}
	return 0
}

func (in Instruction) String() string {
	if in.Op.ArgCount() == 1 {
		return fmt.Sprintf("%s 0x%x", in.Op, in.Arg1)
	}
	return in.Op.String()
}

// Addr interprets arg1 as an address.
func (in Instruction) Addr() int {
	return int(in.Arg1)
}

// U64 interprets arg1 as uint64.
func (in Instruction) U64() uint64 {
	return uint64(in.Arg1)
}

// U32 interprets arg1 as uint32.
func (in Instruction) U32() uint32 {
	return uint32(in.Arg1)
}

// U16 interprets arg1 as uint16.
func (in Instruction) U16() uint16 {
	return uint16(in.Arg1)
}

// U8 interprets arg1 as uint8.
func (in Instruction) U8() uint8 {
	return uint8(in.Arg1)
}

// I64 interprets arg1 as int64.
func (in Instruction) I64() int64 {
	return int64(in.Arg1)
}

// I32 interprets arg1 as int32.
func (in Instruction) I32() int32 {
	return int32(in.Arg1)
}

// I16 interprets arg1 as int16.
func (in Instruction) I16() int16 {
	return int16(in.Arg1)
}

// I8 interprets arg1 as int8.
func (in Instruction) I8() int8 {
	return int8(in.Arg1)
}

// NewInstr0 creates an instruction with 0 arguments.
func NewInstr0(op Opcode) Instruction {
	return Instruction{Op: op}
}

// NewInstr1 creates an instruction with 1 argument.
func NewInstr1(op Opcode, arg1 int64) Instruction {
	return Instruction{Op: op, Arg1: arg1}
}

func AddOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt64:
		op = U64Add
	case semantics.TUInt32:
		op = U32Add
	case semantics.TUInt16:
		op = U16Add
	case semantics.TUInt8:
		op = U8Add
	case semantics.TInt64:
		op = I64Add
	case semantics.TInt32:
		op = I32Add
	case semantics.TInt16:
		op = I16Add
	case semantics.TInt8:
		op = I8Add
	case semantics.TFloat64:
		op = F64Add
	case semantics.TFloat32:
		op = F32Add
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}

func SubOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt64:
		op = U64Sub
	case semantics.TUInt32:
		op = U32Sub
	case semantics.TUInt16:
		op = U16Sub
	case semantics.TUInt8:
		op = U8Sub
	case semantics.TInt64:
		op = I64Sub
	case semantics.TInt32:
		op = I32Sub
	case semantics.TInt16:
		op = I16Sub
	case semantics.TInt8:
		op = I8Sub
	case semantics.TFloat64:
		op = F64Sub
	case semantics.TFloat32:
		op = F32Sub
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}

func MulOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt64:
		op = U64Mul
	case semantics.TUInt32:
		op = U32Mul
	case semantics.TUInt16:
		op = U16Mul
	case semantics.TUInt8:
		op = U8Mul
	case semantics.TInt64:
		op = I64Mul
	case semantics.TInt32:
		op = I32Mul
	case semantics.TInt16:
		op = I16Mul
	case semantics.TInt8:
		op = I8Mul
	case semantics.TFloat64:
		op = F64Mul
	case semantics.TFloat32:
		op = F32Mul
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}

func DivOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt64:
		op = U64Div
	case semantics.TUInt32:
		op = U32Div
	case semantics.TUInt16:
		op = U16Div
	case semantics.TUInt8:
		op = U8Div
	case semantics.TInt64:
		op = I64Div
	case semantics.TInt32:
		op = I32Div
	case semantics.TInt16:
		op = I16Div
	case semantics.TInt8:
		op = I8Div
	case semantics.TFloat64:
		op = F64Div
	case semantics.TFloat32:
		op = F32Div
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}

func ModOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt64:
		op = U64Mod
	case semantics.TUInt32:
		op = U32Mod
	case semantics.TUInt16:
		op = U16Mod
	case semantics.TUInt8:
		op = U8Mod
	case semantics.TInt64:
		op = I64Mod
	case semantics.TInt32:
		op = I32Mod
	case semantics.TInt16:
		op = I16Mod
	case semantics.TInt8:
		op = I8Mod
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}

func CmpOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TBool:
		op = I32Cmp
	case semantics.TUInt64:
		op = U64Cmp
	case semantics.TUInt32:
		op = U32Cmp
	case semantics.TUInt16:
		op = U16Cmp
	case semantics.TUInt8:
		op = U8Cmp
	case semantics.TInt64:
		op = I64Cmp
	case semantics.TInt32:
		op = I32Cmp
	case semantics.TInt16:
		op = I16Cmp
	case semantics.TInt8:
		op = I8Cmp
	case semantics.TFloat64:
		op = F64Cmp
	case semantics.TFloat32:
		op = F32Cmp
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}

func LoadOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt64:
		op = U64Load
	case semantics.TUInt32:
		op = U32Load
	case semantics.TUInt16:
		op = U16Load
	case semantics.TUInt8:
		op = U8Load
	case semantics.TInt64:
		op = I64Load
	case semantics.TInt32:
		op = I32Load
	case semantics.TInt16:
		op = I16Load
	case semantics.TInt8:
		op = I8Load
	case semantics.TFloat64, semantics.TFloat32:
		op = ConstLoad
	case semantics.TBool:
		op = BoolLoad
	default:
		panic(fmt.Sprintf("Unhandled type %T", t))
	}
	return op
}
