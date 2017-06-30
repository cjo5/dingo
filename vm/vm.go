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

type FunctionDescriptor struct {
	Name       string
	Address    int
	ArgCount   int
	LocalCount int
}

type Frame struct {
	function   *FunctionDescriptor
	retAddress int
	locals     []interface{}
}

func (fd FunctionDescriptor) String() string {
	return fmt.Sprintf(".%s(address=0x%x, argCount=%d, localCount=%d)", fd.Name, fd.Address, fd.ArgCount, fd.LocalCount)
}

// VM is the execution context for a bytecode program.
type VM struct {
	inited    bool
	halt      bool
	ip        int // Instruction pointer
	operands  []interface{}
	callStack []*Frame
	currFrame *Frame

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

// Exec a bytecode program.
func (vm *VM) Exec(ip int, code CodeMemory, mem DataMemory) {
	if !vm.inited {
		vm.reset()
	}
	vm.currFrame = &Frame{}
	vm.ip = ip
	for vm.ip < len(code) && !vm.halt {
		in := code[vm.ip]
		ip2 := vm.ip + 1

		switch op := in.Op; op {
		case Nop:
			// Do nothing
		case Halt:
			vm.halt = true
		case Dup:
			if arg := vm.peek1Arg(op); arg != nil {
				vm.push(arg)
			}
		case Pop:
			vm.pop()
		case Ret:
			if len(vm.callStack) > 0 {
				idx := len(vm.callStack) - 1
				ip2 = vm.currFrame.retAddress
				vm.currFrame = vm.callStack[idx]
				vm.callStack = vm.callStack[:idx]
			} else {
				internalPanic(op, "empty call stack")
			}
		case Print:
			if arg := vm.pop1Arg(op); arg != nil {
				str := ""
				switch t := arg.(type) {
				case uint64:
					str = strconv.FormatUint(uint64(t), 10)
				case int64:
					str = strconv.FormatInt(int64(t), 10)
				case uint32:
					str = strconv.FormatUint(uint64(t), 10)
				case int32:
					str = strconv.FormatInt(int64(t), 10)
				case bool:
					str = strconv.FormatBool(t)
				case string:
					str = t
				default:
					vm.runtimePanic(op, fmt.Sprintf("incompatible type '%T'", t))
					break
				}
				vm.output.WriteString(str)
			}
		case Not:
			if arg, ok := vm.pop1I32Arg(op); ok {
				res := 0
				if arg == 0 {
					res = 1
				}
				vm.push(res)
			}
		case U32Add, U32Sub, U32Mul, U32Div, U32Mod:
			if arg1, arg2, ok := vm.pop2U32Args(op); ok {
				res := uint32(0)
				switch op {
				case U32Add:
					res = arg1 + arg2
				case U32Sub:
					res = arg1 - arg2
				case U32Mul:
					res = arg1 * arg2
				case U32Div:
					res = arg1 / arg2
				case U32Mod:
					res = arg1 % arg2
				}
				vm.push(res)
			}
		case I32Add, I32Sub, I32Mul, I32Div, I32Mod:
			if arg1, arg2, ok := vm.pop2I32Args(op); ok {
				res := int32(0)
				switch op {
				case I32Add:
					res = arg1 + arg2
				case I32Sub:
					res = arg1 - arg2
				case I32Mul:
					res = arg1 * arg2
				case I32Div:
					res = arg1 / arg2
				case I32Mod:
					res = arg1 % arg2
				}
				vm.push(res)
			}
		case CmpEq, CmpNe, CmpGt, CmpGe, CmpLt, CmpLe:
			if arg1, arg2, ok := vm.pop2I32Args(op); ok {
				res := int32(0)
				switch op {
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
		case U32ToI32:
			if arg, ok := vm.pop1U32Arg(op); ok {
				vm.push(int32(arg))
			}
		case I32ToU32:
			if arg, ok := vm.pop1I32Arg(op); ok {
				vm.push(uint32(arg))
			}
		case U32Load:
			vm.push(uint32(in.Arg1))
		case I32Load:
			vm.push(int32(in.Arg1))
		case CLoad:
			if in.Arg1 < len(mem.Constants) {
				vm.push(mem.Constants[in.Arg1])
			} else {
				indexOutOfRange(op, in.Arg1)
			}
		case GLoad:
			if in.Arg1 < len(mem.Globals) {
				vm.push(mem.Globals[in.Arg1])
			} else {
				indexOutOfRange(op, in.Arg1)
			}
		case GStore:
			if arg := vm.pop1Arg(op); arg != nil {
				if in.Arg1 < len(mem.Globals) {
					mem.Globals[in.Arg1] = arg
				} else {
					indexOutOfRange(op, in.Arg1)
				}
			}
		case Load:
			if in.Arg1 < len(vm.currFrame.locals) {
				vm.push(vm.currFrame.locals[in.Arg1])
			} else {
				indexOutOfRange(op, in.Arg1)
			}
		case Store:
			if arg := vm.pop1Arg(op); arg != nil {
				if in.Arg1 < len(vm.currFrame.locals) {
					vm.currFrame.locals[in.Arg1] = arg
				} else {
					indexOutOfRange(op, in.Arg1)
				}
			}
		case Goto:
			ip2 = in.Arg1
		case IfFalse:
			if arg, ok := vm.pop1I32Arg(op); ok {
				if arg == 0 {
					ip2 = in.Arg1
				}
			}
		case IfTrue:
			if arg, ok := vm.pop1I32Arg(op); ok {
				if arg != 0 {
					ip2 = in.Arg1
				}
			}
		case Call:
			if in.Arg1 < len(mem.Constants) {
				c := mem.Constants[in.Arg1]
				if fun, ok := c.(*FunctionDescriptor); ok {
					vm.callStack = append(vm.callStack, vm.currFrame)
					vm.currFrame = &Frame{}
					vm.currFrame.function = fun
					vm.currFrame.retAddress = ip2
					vm.currFrame.locals = make([]interface{}, fun.LocalCount+fun.ArgCount)
					ip2 = fun.Address
					for i := 0; i < fun.ArgCount; i++ {
						if arg := vm.pop1Arg(op); arg != nil {
							vm.currFrame.locals[i] = arg
						} else {
							internalPanic(op, fmt.Sprintf("invalid arg count %d", fun.ArgCount))
							break
						}
					}
				} else {
					vm.runtimePanic(op, fmt.Sprintf("expected FunctionDescriptor, got %T", c))
				}
			} else {
				indexOutOfRange(op, in.Arg1)
			}
		default:
			internalPanic(op, "not implemented")
		}
		vm.ip = ip2
	}

	vm.inited = false
}

func (vm *VM) pop2U32Args(op Opcode) (uint32, uint32, bool) {
	arg2 := vm.pop()
	arg1 := vm.pop()
	if arg1 == nil || arg2 == nil {
		internalPanic(op, "2 arguments required")
		return 0, 0, false
	}
	arg1Int, ok1 := arg1.(uint32)
	arg2Int, ok2 := arg2.(uint32)
	if !ok1 || !ok2 {
		internalPanic(op, "2 u32 arguments expected (got %T and %T)", arg1, arg2)
		return 0, 0, false
	}
	return arg1Int, arg2Int, true
}

func (vm *VM) pop2I32Args(op Opcode) (int32, int32, bool) {
	arg2 := vm.pop()
	arg1 := vm.pop()
	if arg1 == nil || arg2 == nil {
		internalPanic(op, "2 arguments required")
		return 0, 0, false
	}
	arg1Int, ok1 := arg1.(int32)
	arg2Int, ok2 := arg2.(int32)
	if !ok1 || !ok2 {
		internalPanic(op, "2xi32 arguments expected (got %T and %T)", arg1, arg2)
		return 0, 0, false
	}
	return arg1Int, arg2Int, true
}

func (vm *VM) pop1U32Arg(op Opcode) (uint32, bool) {
	arg := vm.pop1Arg(op)
	if arg == nil {
		return 0, false
	}
	argInt, ok := arg.(uint32)
	if !ok {
		vm.runtimePanic(op, fmt.Sprintf("1xu32 argument expected got (%T)", arg))
		return 0, false
	}
	return argInt, true
}

func (vm *VM) pop1I32Arg(op Opcode) (int32, bool) {
	arg := vm.pop1Arg(op)
	if arg == nil {
		return 0, false
	}
	argInt, ok := arg.(int32)
	if !ok {
		vm.runtimePanic(op, fmt.Sprintf("1xi32 argument expected got (%T)", arg))
		return 0, false
	}
	return argInt, true
}

func (vm *VM) pop1Arg(op Opcode) interface{} {
	if arg := vm.pop(); arg != nil {
		return arg
	}
	internalPanic(op, "1 argument required")
	return nil
}

func (vm *VM) peek1Arg(op Opcode) interface{} {
	if arg := vm.peek(); arg != nil {
		return arg
	}
	internalPanic(op, "1 argument required")
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
	vm.inited = true
	vm.halt = false
	vm.ip = 0
	vm.operands = nil
	vm.callStack = nil
	vm.currFrame = nil
	vm.Err = ""
}

func (vm *VM) runtimePanic(op Opcode, format string, args ...interface{}) {
	vm.Err = fmt.Sprintf("%s: %s", op, fmt.Sprintf(format, args...))
	vm.halt = true
}

func internalPanic(op Opcode, format string, args ...interface{}) {
	panic(fmt.Sprintf("%s: %s", op, fmt.Sprintf(format, args...)))
}

func indexOutOfRange(op Opcode, index int) {
	internalPanic(op, "index %d out of range", index)
}
