package vm

import (
	"fmt"
	"os"
	"strconv"
)

// CodeMemory represents a program's instructions.
type CodeMemory []Instruction

// DataMemory represents a program's globals and constants.
type DataMemory struct {
	Globals   []interface{}
	Constants []interface{}
}

// VM is the execution context for a bytecode program.
type VM struct {
	ip       int // Instruction pointer
	halt     bool
	operands []interface{}
	inited   bool

	output *os.File
	Err    string
}

// DumpMemory prints the content of a program's data memory.
func DumpMemory(mem []interface{}, output *os.File) {
	for i, c := range mem {
		s := ""
		if tmp, ok := c.(string); ok {
			s = strconv.Quote(tmp)
		} else {
			s = fmt.Sprintf("%v", c)
		}
		output.WriteString(fmt.Sprintf("%04d: %s <%T>\n", i, s, c))
	}
}

// Disasm prints a bytecode program in human-readable format.
func Disasm(code CodeMemory, output *os.File) {
	for i, c := range code {
		output.WriteString(fmt.Sprintf("%04x: %s\n", i, c))
	}
}

// NewMachine creates a new VM.
func NewMachine(output *os.File) *VM {
	vm := &VM{}
	vm.output = output
	vm.reset()
	return vm
}

// RuntimeError returns true if an error occured during program execution.
func (vm *VM) RuntimeError() bool {
	return len(vm.Err) > 0
}

// Exec executes a bytecode program.
//
// TODO:
// - Support for floats
//
func (vm *VM) Exec(code CodeMemory, mem DataMemory) {
	if !vm.inited {
		vm.reset()
	}

	for vm.ip < len(code) && !vm.halt {
		in := code[vm.ip]
		ip2 := vm.ip + 1

		switch op := in.Op; {
		case op == Halt:
			vm.halt = true
		case op == Dup:
			if arg := vm.peek1Arg(op); arg != nil {
				vm.push(arg)
			}
		case op == Print:
			if arg := vm.pop1Arg(op); arg != nil {
				str := ""
				switch t := arg.(type) {
				case int:
					str = strconv.Itoa(t)
				case bool:
					str = strconv.FormatBool(t)
				case string:
					str = t
				default:
					vm.panic(op, fmt.Sprintf("incompatible type '%T'", t))
					break
				}
				vm.output.WriteString(str)
			}
		case opBinaryStart < op && op < opBinaryEnd:
			if arg1, arg2, ok := vm.pop2IntArgs(op); ok {
				res := 0
				switch op {
				case BinaryAdd:
					res = arg1 + arg2
				case BinarySub:
					res = arg1 - arg2
				case BinaryMul:
					res = arg1 * arg2
				case BinaryDiv:
					res = arg1 / arg2
				case BinaryMod:
					res = arg1 % arg2
				case CmpEq:
					if arg1 == arg2 {
						res = 1
					}
				case CmpNe:
					if arg1 != arg2 {
						res = 1
					}
				case CmpGt:
					if arg1 > arg2 {
						res = 1
					}
				case CmpGe:
					if arg1 >= arg2 {
						res = 1
					}
				case CmpLt:
					if arg1 < arg2 {
						res = 1
					}
				case CmpLe:
					if arg1 <= arg2 {
						res = 1
					}
				}
				vm.push(res)
			}
		case op == Iload:
			vm.push(in.Arg1)
		case op == Cload:
			if in.Arg1 < len(mem.Constants) {
				vm.push(mem.Constants[in.Arg1])
			} else {
				vm.panic(op, fmt.Sprintf("index '%d' out of range", in.Arg1))
			}
		case op == Gload:
			if in.Arg1 < len(mem.Globals) {
				vm.push(mem.Globals[in.Arg1])
			} else {
				vm.panic(op, fmt.Sprintf("index '%d' out of range", in.Arg1))
			}
		case op == Gstore:
			if arg := vm.pop1Arg(op); arg != nil {
				if in.Arg1 < len(mem.Globals) {
					mem.Globals[in.Arg1] = arg
				} else {
					vm.panic(op, fmt.Sprintf("index '%d' out of range", in.Arg1))
				}
			}
		case op == Goto:
			ip2 = in.Arg1
		case op == IfFalse:
			if arg, ok := vm.pop1IntArg(op); ok {
				if arg == 0 {
					ip2 = in.Arg1
				}
			}
		case op == IfTrue:
			if arg, ok := vm.pop1IntArg(op); ok {
				if arg != 0 {
					ip2 = in.Arg1
				}
			}
		}
		vm.ip = ip2
	}

	vm.inited = false
}

func (vm *VM) pop2IntArgs(op Opcode) (int, int, bool) {
	arg2 := vm.pop()
	arg1 := vm.pop()
	if arg1 == nil || arg2 == nil {
		vm.panic(op, "2 arguments required")
		return 0, 0, false
	}
	arg1Int, ok1 := arg1.(int)
	arg2Int, ok2 := arg2.(int)
	if !ok1 || !ok2 {
		vm.panic(op, fmt.Sprintf("incompatible types '%T' and '%T'", arg1, arg2))
		return 0, 0, false
	}
	return arg1Int, arg2Int, true
}

func (vm *VM) pop1IntArg(op Opcode) (int, bool) {
	arg := vm.pop1Arg(op)
	if arg == nil {
		return 0, false
	}
	argInt, ok := arg.(int)
	if !ok {
		vm.panic(op, fmt.Sprintf("incompatible type '%T'", arg))
		return 0, false
	}
	return argInt, true
}

func (vm *VM) pop1Arg(op Opcode) interface{} {
	if arg := vm.pop(); arg != nil {
		return arg
	}
	vm.panic(op, "1 argument required")
	return nil
}

func (vm *VM) peek1Arg(op Opcode) interface{} {
	if arg := vm.peek(); arg != nil {
		return arg
	}
	vm.panic(op, "1 argument required")
	return nil
}

func (vm *VM) push(arg interface{}) {
	vm.operands = append(vm.operands, arg)
}

func (vm *VM) pop() interface{} {
	n := len(vm.operands)
	if n == 0 {
		return nil
	}
	index := n - 1
	arg := vm.operands[index]
	vm.operands = vm.operands[:index]
	return arg
}

func (vm *VM) peek() interface{} {
	n := len(vm.operands)
	if n == 0 {
		return nil
	}
	return vm.operands[n-1]
}

func (vm *VM) reset() {
	vm.ip = 0
	vm.halt = false
	vm.operands = nil
	vm.inited = true
	vm.Err = ""
}

func (vm *VM) panic(op Opcode, msg string) {
	vm.Err = fmt.Sprintf("%s: %s", op, msg)
	vm.halt = true
}
