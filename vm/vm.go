package vm

import (
	"fmt"
	"os"
	"reflect"
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
		case U64Add, U64Sub, U64Mul, U64Div, U64Mod, U64Cmp:
			if arg1, arg2, ok := vm.pop2U64Args(op); ok {
				switch op {
				case U64Add:
					vm.push(arg1 + arg2)
				case U64Sub:
					vm.push(arg1 - arg2)
				case U64Mul:
					vm.push(arg1 * arg2)
				case U64Div:
					vm.push(arg1 / arg2)
				case U64Mod:
					vm.push(arg1 % arg2)
				case U64Cmp:
					res := int32(0)
					if arg1 < arg2 {
						res = -1
					} else if arg1 > arg2 {
						res = 1
					}
					vm.push(res)
				}
			}
		case I64Add, I64Sub, I64Mul, I64Div, I64Mod, I64Cmp:
			if arg1, arg2, ok := vm.pop2I64Args(op); ok {
				switch op {
				case I64Add:
					vm.push(arg1 + arg2)
				case I64Sub:
					vm.push(arg1 - arg2)
				case I64Mul:
					vm.push(arg1 * arg2)
				case I64Div:
					vm.push(arg1 / arg2)
				case I64Mod:
					vm.push(arg1 % arg2)
				case U64Cmp:
					res := int32(0)
					if arg1 < arg2 {
						res = -1
					} else if arg1 > arg2 {
						res = 1
					}
					vm.push(res)
				}

			}
		case U32Add, U32Sub, U32Mul, U32Div, U32Mod, U32Cmp:
			if arg1, arg2, ok := vm.pop2U32Args(op); ok {
				switch op {
				case U32Add:
					vm.push(arg1 + arg2)
				case U32Sub:
					vm.push(arg1 - arg2)
				case U32Mul:
					vm.push(arg1 * arg2)
				case U32Div:
					vm.push(arg1 / arg2)
				case U32Mod:
					vm.push(arg1 % arg2)
				case U32Cmp:
					res := int32(0)
					if arg1 < arg2 {
						res = -1
					} else if arg1 > arg2 {
						res = 1
					}
					vm.push(res)
				}
			}
		case I32Add, I32Sub, I32Mul, I32Div, I32Mod, I32Cmp:
			if arg1, arg2, ok := vm.pop2I32Args(op); ok {
				switch op {
				case I32Add:
					vm.push(arg1 + arg2)
				case I32Sub:
					vm.push(arg1 - arg2)
				case I32Mul:
					vm.push(arg1 * arg2)
				case I32Div:
					vm.push(arg1 / arg2)
				case I32Mod:
					vm.push(arg1 % arg2)
				case I32Cmp:
					res := int32(0)
					if arg1 < arg2 {
						res = -1
					} else if arg1 > arg2 {
						res = 1
					}
					vm.push(res)
				}
			}
		case CmpEq, CmpNe, CmpGt, CmpGe, CmpLt, CmpLe:
			if arg, ok := vm.pop1I32Arg(op); ok {
				res := int32(0)
				switch op {
				case CmpEq:
					if arg == 0 {
						res = 1
					}
				case CmpNe:
					if arg != 0 {
						res = 1
					}
				case CmpGt:
					if arg > 0 {
						res = 1
					}
				case CmpGe:
					if arg >= 0 {
						res = 1
					}
				case CmpLt:
					if arg < 0 {
						res = 1
					}
				case CmpLe:
					if arg <= 0 {
						res = 1
					}
				}
				vm.push(res)
			}
		case NumCast:
			if arg1, arg2, ok := vm.pop2Args(op); ok {
				tmp := int64(0)
				err := false

				switch t1 := arg1.(type) {
				case uint64:
					tmp = int64(t1)
				case int64:
					tmp = int64(t1)
				case uint32:
					tmp = int64(t1)
				case int32:
					tmp = int64(t1)
				default:
					err = true
				}

				if !err {
					t2 := reflect.TypeOf(arg2).Kind()
					switch t2 {
					case reflect.Uint64:
						vm.push(uint64(tmp))
					case reflect.Int64:
						vm.push(int64(tmp))
					case reflect.Uint32:
						vm.push(uint32(tmp))
					case reflect.Int32:
						vm.push(int32(tmp))
					default:
						err = true
					}
				}

				if err {
					vm.runtimePanic(op, "%T cannot be cast to %T", arg1, arg2)
				}
			}
		case U64Load:
			vm.push(in.U64())
		case I64Load:
			vm.push(in.I64())
		case U32Load:
			vm.push(in.U32())
		case I32Load:
			vm.push(in.I32())
		case CLoad:
			addr := in.Addr()
			if addr < len(mem.Constants) {
				vm.push(mem.Constants[addr])
			} else {
				indexOutOfRange(op, addr)
			}
		case GLoad:
			addr := in.Addr()
			if addr < len(mem.Globals) {
				vm.push(mem.Globals[addr])
			} else {
				indexOutOfRange(op, addr)
			}
		case GStore:
			if arg := vm.pop1Arg(op); arg != nil {
				addr := in.Addr()
				if addr < len(mem.Globals) {
					mem.Globals[addr] = arg
				} else {
					indexOutOfRange(op, addr)
				}
			}
		case Load:
			addr := in.Addr()
			if addr < len(vm.currFrame.locals) {
				vm.push(vm.currFrame.locals[addr])
			} else {
				indexOutOfRange(op, addr)
			}
		case Store:
			addr := in.Addr()
			if arg := vm.pop1Arg(op); arg != nil {
				if addr < len(vm.currFrame.locals) {
					vm.currFrame.locals[addr] = arg
				} else {
					indexOutOfRange(op, addr)
				}
			}
		case Goto:
			ip2 = in.Addr()
		case IfFalse:
			if arg, ok := vm.pop1I32Arg(op); ok {
				if arg == 0 {
					ip2 = in.Addr()
				}
			}
		case IfTrue:
			if arg, ok := vm.pop1I32Arg(op); ok {
				if arg != 0 {
					ip2 = in.Addr()
				}
			}
		case Call:
			addr := in.Addr()
			if addr < len(mem.Constants) {
				c := mem.Constants[addr]
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
				indexOutOfRange(op, addr)
			}
		default:
			internalPanic(op, "not implemented")
		}
		vm.ip = ip2
	}

	vm.inited = false
}

func (vm *VM) pop2U64Args(op Opcode) (uint64, uint64, bool) {
	arg2 := vm.pop()
	arg1 := vm.pop()
	if arg1 == nil || arg2 == nil {
		internalPanic(op, "2 arguments required")
		return 0, 0, false
	}
	arg1Int, ok1 := arg1.(uint64)
	arg2Int, ok2 := arg2.(uint64)
	if !ok1 || !ok2 {
		internalPanic(op, "2 u64 arguments expected (got %T and %T)", arg1, arg2)
		return 0, 0, false
	}
	return arg1Int, arg2Int, true
}

func (vm *VM) pop2I64Args(op Opcode) (int64, int64, bool) {
	arg2 := vm.pop()
	arg1 := vm.pop()
	if arg1 == nil || arg2 == nil {
		internalPanic(op, "2 arguments required")
		return 0, 0, false
	}
	arg1Int, ok1 := arg1.(int64)
	arg2Int, ok2 := arg2.(int64)
	if !ok1 || !ok2 {
		internalPanic(op, "2 i64 arguments expected (got %T and %T)", arg1, arg2)
		return 0, 0, false
	}
	return arg1Int, arg2Int, true
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

func (vm *VM) pop2Args(op Opcode) (interface{}, interface{}, bool) {
	arg2 := vm.pop()
	arg1 := vm.pop()
	if arg1 == nil || arg2 == nil {
		internalPanic(op, "2 arguments required")
		return nil, nil, false
	}
	return arg1, arg2, true
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
