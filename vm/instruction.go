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

// List of opcodes
const (
	opArg0Start Opcode = iota

	NOP
	HALT
	DUP
	PRINT

	opBinaryStart
	BINARY_ADD
	BINARY_SUB
	BINARY_MUL
	BINARY_DIV
	BINARY_MOD
	opBinaryEnd

	opArg0End

	opArg1Start
	ILOAD  // Push immediate
	CLOAD  // Push constant
	GLOAD  // Push global
	GSTORE // Pop and store global

	// Branch opcodes
	GOTO
	opCmpStart
	CMP_EQ
	CMP_NE
	CMP_GT
	CMP_GE
	CMP_LT
	CMP_LE
	opCmpEnd

	opArg1End
)

var mnemonics = [...]string{
	NOP:   "nop",
	HALT:  "halt",
	DUP:   "dup",
	PRINT: "print",

	BINARY_ADD: "add",
	BINARY_SUB: "sub",
	BINARY_MUL: "mul",
	BINARY_DIV: "div",
	BINARY_MOD: "mod",

	ILOAD:  "iload",
	CLOAD:  "cload",
	GLOAD:  "gload",
	GSTORE: "gstore",

	GOTO:   "goto",
	CMP_EQ: "cmpeq",
	CMP_NE: "cmpne",
	CMP_GT: "cmpgt",
	CMP_LT: "cmplt",
	CMP_LE: "cmple",
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

// NewInstr0 creates an instruction with 0 arguments
func NewInstr0(op Opcode) Instruction {
	return Instruction{Op: op}
}

// NewInstr1 creates an instruction with 1 arguments
func NewInstr1(op Opcode, arg1 int) Instruction {
	return Instruction{Op: op, arg1: arg1}
}
