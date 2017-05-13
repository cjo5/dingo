package vm

import (
	"fmt"
	"strconv"
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

	opBinaryStart
	BinaryAdd
	BinarySub
	BinaryMul
	BinaryDiv
	BinaryMod
	CmpEq
	CmpNe
	CmpGt
	CmpGe
	CmpLt
	CmpLe
	opBinaryEnd

	opArg0End

	opArg1Start
	Iload  // Push immediate
	Cload  // Push constant
	Gload  // Push global variable
	Gstore // Pop and store global variable
	Load   // Push local variable
	Store  // Pop loal variable

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

	BinaryAdd: "add",
	BinarySub: "sub",
	BinaryMul: "mul",
	BinaryDiv: "div",
	BinaryMod: "mod",
	CmpEq:     "cmpeq",
	CmpNe:     "cmpne",
	CmpGt:     "cmpgt",
	CmpLt:     "cmplt",
	CmpLe:     "cmple",

	Iload:  "iload",
	Cload:  "cload",
	Gload:  "gload",
	Gstore: "gstore",
	Load:   "load",
	Store:  "store",

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
