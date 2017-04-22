package vm

import (
	"fmt"
	"os"
	"strconv"
)

type CodeMemory []Instruction

type DataMemory struct {
	Globals   []interface{}
	Constants []interface{}
}

type VM struct {
	ip       int // Instruction pointer
	halt     bool
	operands []interface{}

	err    string
	inited bool
}

func NewMachine() *VM {
	vm := &VM{}
	vm.reset()
	return vm
}

func (vm *VM) Disasm(code CodeMemory, mem DataMemory) {
	fmt.Println("Constants")
	for i, c := range mem.Constants {
		s := ""
		if tmp, ok := c.(string); ok {
			s = strconv.Quote(tmp)
		}
		fmt.Printf("%04d: %s <%T>\n", i, s, c)
	}
	fmt.Println("\nCode")
	for i, c := range code {
		fmt.Printf("%04x: %s\n", i, c)
	}
	fmt.Println()
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
		case op == HALT:
			vm.halt = true
		case op == DUP:
			if arg := vm.peek1Arg(op); arg != nil {
				vm.push(arg)
			}
		case op == PRINT:
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
				os.Stdout.Write([]byte(str))
			}
		case opBinaryStart < op && op < opBinaryEnd:
			if arg1, arg2, ok := vm.pop2IntArgs(op); ok {
				res := 0
				switch op {
				case BINARY_ADD:
					res = arg1 + arg2
				case BINARY_SUB:
					res = arg1 - arg2
				case BINARY_MUL:
					res = arg1 * arg2
				case BINARY_DIV:
					res = arg1 / arg2
				case BINARY_MOD:
					res = arg1 % arg2
				}
				vm.push(res)
			}
		case op == ILOAD:
			vm.push(in.arg1)
		case op == CLOAD:
			if in.arg1 < len(mem.Constants) {
				vm.push(mem.Constants[in.arg1])
			} else {
				vm.panic(op, fmt.Sprintf("index '%d' out of range", in.arg1))
			}
		case op == GLOAD:
			if in.arg1 < len(mem.Globals) {
				vm.push(mem.Globals[in.arg1])
			} else {
				vm.panic(op, fmt.Sprintf("index '%d' out of range", in.arg1))
			}
		case op == GSTORE:
			if arg := vm.pop1Arg(op); arg != nil {
				if in.arg1 < len(mem.Globals) {
					mem.Globals[in.arg1] = arg
				} else {
					vm.panic(op, fmt.Sprintf("index '%d' out of range", in.arg1))
				}
			}
		case op == GOTO:
			ip2 = in.arg1
		case opCmpStart < op && op < opCmpEnd:
			if arg1, arg2, ok := vm.pop2IntArgs(op); ok {
				switch op {
				case CMP_EQ:
					if arg1 == arg2 {
						ip2 = in.arg1
					}
				case CMP_NE:
					if arg1 != arg2 {
						ip2 = in.arg1
					}
				case CMP_GT:
					if arg1 > arg2 {
						ip2 = in.arg1
					}
				case CMP_GE:
					if arg1 >= arg2 {
						ip2 = in.arg1
					}
				case CMP_LT:
					if arg1 < arg2 {
						ip2 = in.arg1
					}
				case CMP_LE:
					if arg1 <= arg2 {
						ip2 = in.arg1
					}
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
	vm.err = ""
	vm.inited = true
}

func (vm *VM) panic(op Opcode, msg string) {
	vm.err = fmt.Sprintf("%s: %s", op, msg)
	vm.halt = true
}

func (vm *VM) RuntimeError() bool {
	return len(vm.err) > 0
}

func (vm *VM) PrintTrace() {
	fmt.Println("Runtime error:", vm.err)
}
