package vm

import (
	"fmt"
	"strconv"

	"github.com/jhnl/interpreter/semantics"
)

type Opcode int

type Instruction struct {
	Op   Opcode
	Arg1 int
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
	U32Add
	I32Add
	U32Sub
	I32Sub
	U32Mul
	I32Mul
	U32Div
	I32Div
	U32Mod
	I32Mod
	CmpEq
	CmpNe
	CmpGt
	CmpGe
	CmpLt
	CmpLe

	I32ToU32
	U32ToI32

	opArg0End

	opArg1Start
	I32Load // Push immediate i32
	U32Load // Push immediate u32
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
	U32Add: "u32add",
	I32Add: "i32add",
	U32Sub: "u32sub",
	I32Sub: "i32sub",
	U32Mul: "u32mul",
	I32Mul: "i32mul",
	U32Div: "u32div",
	I32Div: "i32div",
	U32Mod: "u32mod",
	I32Mod: "i32mod",
	CmpEq:  "cmpeq",
	CmpNe:  "cmpne",
	CmpGt:  "cmpgt",
	CmpLt:  "cmplt",
	CmpLe:  "cmple",

	U32ToI32: "u32toi32",
	I32ToU32: "i32tou32",

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

// NewInstr0 creates an instruction with 0 arguments.
func NewInstr0(op Opcode) Instruction {
	return Instruction{Op: op}
}

// NewInstr1 creates an instruction with 1 argument.
func NewInstr1(op Opcode, arg1 int) Instruction {
	return Instruction{Op: op, Arg1: arg1}
}

func AddOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt32:
		op = U32Add
	case semantics.TInt32:
		op = I32Add
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}

func SubOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt32:
		op = U32Sub
	case semantics.TInt32:
		op = I32Sub
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}

func MulOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt32:
		op = U32Mul
	case semantics.TInt32:
		op = I32Mul
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}

func DivOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt32:
		op = U32Div
	case semantics.TInt32:
		op = I32Div
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}

func ModOp(t semantics.TypeID) Opcode {
	op := Nop
	switch t {
	case semantics.TUInt32:
		op = U32Mod
	case semantics.TInt32:
		op = I32Mod
	default:
		panic(fmt.Sprintf("Unhandled type %s", t))
	}
	return op
}
