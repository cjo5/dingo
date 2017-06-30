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
	I64Add
	U32Add
	I32Add
	U64Sub
	I64Sub
	U32Sub
	I32Sub
	U64Mul
	I64Mul
	U32Mul
	I32Mul
	U64Div
	I64Div
	U32Div
	I32Div
	U64Mod
	I64Mod
	U32Mod
	I32Mod
	U64Cmp
	I64Cmp
	U32Cmp
	I32Cmp
	CmpEq
	CmpNe
	CmpGt
	CmpGe
	CmpLt
	CmpLe

	NumCast // Numeric cast. Cast operand1 to type of operand2.

	opArg0End

	opArg1Start
	U64Load // Push immediate u64
	I64Load // Push immediate i64
	U32Load // Push immediate u32
	I32Load // Push immediate i32
	CLoad   // Push constant
	GLoad   // Push global variable
	GStore  // Pop and store global variable
	Load    // Push local variable
	Store   // Pop loal variable

	// Branch opcodes
	Goto
	IfTrue  // Branch if true
	IfFalse // Branch if false

	Call

	opArg1End
)

var mnemonics = [...]string{
	Nop:   "nop",
	Halt:  "halt",
	Dup:   "dup",
	Pop:   "pop",
	Ret:   "ret",
	Print: "print",

	Not:    "not",
	U64Add: "u64add",
	I64Add: "i64add",
	U32Add: "u32add",
	I32Add: "i32add",
	U64Sub: "u64sub",
	I64Sub: "i64sub",
	U32Sub: "u32sub",
	I32Sub: "i32sub",
	U64Mul: "u64mul",
	I64Mul: "i64mul",
	U32Mul: "u32mul",
	I32Mul: "i32mul",
	U64Div: "u64div",
	I64Div: "i64div",
	U32Div: "u32div",
	I32Div: "i32div",
	U64Mod: "u64mod",
	I64Mod: "i64mod",
	U32Mod: "u32mod",
	I32Mod: "i32mod",
	U64Cmp: "u64cmp",
	I64Cmp: "i64cmp",
	U32Cmp: "u32cmp",
	I32Cmp: "i32cmp",
	CmpEq:  "cmpeq",
	CmpNe:  "cmpne",
	CmpGt:  "cmpgt",
	CmpLt:  "cmplt",
	CmpLe:  "cmple",

	NumCast: "numcast",

	U64Load: "u64load",
	I64Load: "i64load",
	U32Load: "u32load",
	I32Load: "i32load",
	CLoad:   "cload",
	GLoad:   "gload",
	GStore:  "gstore",
	Load:    "load",
	Store:   "store",

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

// I64 interprets arg1 as int64.
func (in Instruction) I64() int64 {
	return int64(in.Arg1)
}

// U32 interprets arg1 as uint32.
func (in Instruction) U32() uint32 {
	return uint32(in.Arg1)
}

// I32 interprets arg1 as int32.
func (in Instruction) I32() int32 {
	return int32(in.Arg1)
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
	case semantics.TInt64:
		op = I64Add
	case semantics.TUInt32, semantics.TUInt16, semantics.TUInt8:
		op = U32Add
	case semantics.TInt32, semantics.TInt16, semantics.TInt8:
		op = I32Add
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
	case semantics.TInt64:
		op = I64Sub
	case semantics.TUInt32, semantics.TUInt16, semantics.TUInt8:
		op = U32Sub
	case semantics.TInt32, semantics.TInt16, semantics.TInt8:
		op = I32Sub
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
	case semantics.TInt64:
		op = I64Mul
	case semantics.TUInt32, semantics.TUInt16, semantics.TUInt8:
		op = U32Mul
	case semantics.TInt32, semantics.TInt16, semantics.TInt8:
		op = I32Mul
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
	case semantics.TInt64:
		op = I64Div
	case semantics.TUInt32, semantics.TUInt16, semantics.TUInt8:
		op = U32Div
	case semantics.TInt32, semantics.TInt16, semantics.TInt8:
		op = I32Div
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
	case semantics.TInt64:
		op = I64Mod
	case semantics.TUInt32, semantics.TUInt16, semantics.TUInt8:
		op = U32Mod
	case semantics.TInt32, semantics.TInt16, semantics.TInt8:
		op = I32Mod
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}

func CmpOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt64:
		op = U64Cmp
	case semantics.TInt64:
		op = I64Cmp
	case semantics.TUInt32, semantics.TUInt16, semantics.TUInt8:
		op = U32Cmp
	case semantics.TBool, semantics.TInt32, semantics.TInt16, semantics.TInt8:
		op = I32Cmp
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}

	return op
}
