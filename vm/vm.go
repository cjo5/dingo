package vm

import (
	"fmt"
	"os"
	"strconv"
)

type CodeMemory []Instruction

type DataMemory struct {
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

// Exec executes a bytecode program
func (vm *VM) Exec(code CodeMemory, mem DataMemory) {
	if !vm.inited {
		vm.reset()
	}

	for vm.ip < len(code) && !vm.halt {
		in := code[vm.ip]
		ip2 := vm.ip + 1

		switch in.Op {
		case HALT:
			vm.halt = true
		case PRINT:
			if arg := vm.pop(); arg != nil {
				str := ""
				switch t := arg.(type) {
				case int:
					str = strconv.Itoa(t)
				case bool:
					str = strconv.FormatBool(t)
				case string:
					str = t
				default:
					vm.panic(in.Op, fmt.Sprintf("incompatible type '%T'", t))
					break
				}
				os.Stdout.Write([]byte(str))
			} else {
				vm.panic(in.Op, "1 argument required")
			}
		case BINARY_ADD:
			fallthrough
		case BINARY_SUB:
			fallthrough
		case BINARY_MUL:
			fallthrough
		case BINARY_DIV:
			fallthrough
		case BINARY_MOD:
			arg2 := vm.pop()
			arg1 := vm.pop()
			if arg1 != nil && arg2 != nil {
				arg1Int, ok1 := arg1.(int)
				arg2Int, ok2 := arg2.(int)
				if ok1 && ok2 {
					res := 0
					switch in.Op {
					case BINARY_ADD:
						res = arg1Int + arg2Int
					case BINARY_SUB:
						res = arg1Int - arg2Int
					case BINARY_MUL:
						res = arg1Int * arg2Int
					case BINARY_DIV:
						res = arg1Int / arg2Int
					case BINARY_MOD:
						res = arg1Int % arg2Int
					}
					vm.push(res)
				} else {
					vm.panic(in.Op, fmt.Sprintf("incompatible types '%T' and '%T'", arg1, arg2))
				}
			} else {
				vm.panic(in.Op, "2 arguments required")
			}
		case IPUSH:
			vm.push(in.arg1)
		case CPUSH:
			if in.arg1 < len(mem.Constants) {
				vm.push(mem.Constants[in.arg1])
			} else {
				vm.panic(in.Op, fmt.Sprintf("index '%d' out of range", in.arg1))
			}
		}
		vm.ip = ip2
	}

	vm.inited = false
}

func (vm *VM) push(arg interface{}) {
	vm.operands = append(vm.operands, arg)
}

func (vm *VM) pop() interface{} {
	if len(vm.operands) == 0 {
		return nil
	}
	index := len(vm.operands) - 1
	arg := vm.operands[index]
	vm.operands = vm.operands[:index]
	return arg
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
