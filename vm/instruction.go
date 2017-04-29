package vm

import (
	"fmt"
	"strconv"
)

type Opcode int

type Instruction struct {
	Op   Opcode
	arg1 int
}

// List of opcodes.
const (
	opArg0Start Opcode = iota

	Nop
	Halt
	Dup
	Print

	opBinaryStart
	BinaryAdd
	BinarySub
	BinaryMul
	BinaryDiv
	BinaryMod
	opBinaryEnd

	opArg0End

	opArg1Start
	Iload  // Push immediate
	Cload  // Push constant
	Gload  // Push global
	Gstore // Pop and store global

	// Branch opcodes
	Goto
	opCmpStart
	CmpEq
	CmpNe
	CmpGt
	CmpGe
	CmpLt
	CmpLe
	opCmpEnd

	opArg1End
)

var mnemonics = [...]string{
	Nop:   "nop",
	Halt:  "halt",
	Dup:   "dup",
	Print: "print",

	BinaryAdd: "add",
	BinarySub: "sub",
	BinaryMul: "mul",
	BinaryDiv: "div",
	BinaryMod: "mod",

	Iload:  "iload",
	Cload:  "cload",
	Gload:  "gload",
	Gstore: "gstore",

	Goto:  "goto",
	CmpEq: "cmpeq",
	CmpNe: "cmpne",
	CmpGt: "cmpgt",
	CmpLt: "cmplt",
	CmpLe: "cmple",
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
		return fmt.Sprintf("%s 0x%x", in.Op, in.arg1)
	}
	return in.Op.String()
}

// NewInstr0 creates an instruction with 0 arguments.
func NewInstr0(op Opcode) Instruction {
	return Instruction{Op: op}
}

// NewInstr1 creates an instruction with 1 argument.
func NewInstr1(op Opcode, arg1 int) Instruction {
	return Instruction{Op: op, arg1: arg1}
}
