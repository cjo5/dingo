package vm

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
)

// Run bytecode.
func Run(code *BytecodeProgram, output *os.File) {
	vm := &vm{code: code, output: output}

	defer func() {
		if r := recover(); r != nil {
			if vm.frame != nil {
				if !vm.frame.end() {
					// TODO: Print stack trace
					in := vm.frame.instr()
					panic(fmt.Sprintf("instruction: %s", in))
				}
			}
			panic(r)
		}
	}()

	vm.runProgram()
}

type vm struct {
	code      *BytecodeProgram
	frame     *frame   // Current frame
	callStack []*frame // Previous frames (current is not included)

	output *os.File
	halt   bool
}

func (vm *vm) pushFrame(frame *frame) {
	vm.callStack = append(vm.callStack, vm.frame)
	vm.frame = frame
}

func (vm *vm) popFrame() {
	idx := len(vm.callStack) - 1
	vm.frame = vm.callStack[idx]
	vm.callStack = vm.callStack[:idx]
}

func floatToString(value float64) string {
	return fmt.Sprintf("%.6g", value)
}

func (vm *vm) runProgram() {
	bootstrapMod := vm.code.Modules[0]
	bootstrapFun := bootstrapMod.Functions[0]
	vm.frame = newFrame(bootstrapMod, bootstrapFun)

	for !vm.halt {
		frame := vm.frame
		if frame.end() {
			if len(vm.callStack) == 0 {
				vm.halt = true
			} else {
				vm.popFrame()
			}
			continue
		}

		in := frame.instr()
		ip2 := frame.ip + 1

		switch op := in.Op; op {
		case Nop:
			// Do nothing
		case Halt:
			vm.halt = true
		case Dup:
			frame.push(frame.peek())
		case Pop:
			frame.pop()
		case Ret:
			vm.popFrame()
			if frame.fun.ReturnValue {
				vm.frame.push(frame.pop())
			}
			frame = vm.frame
			ip2 = frame.ip
		case Print:
			arg := frame.pop()
			str := ""
			switch t := arg.(type) {
			case bool:
				str = strconv.FormatBool(t)
			case string:
				str = t
			case uint64:
				str = strconv.FormatUint(uint64(t), 10)
			case uint32:
				str = strconv.FormatUint(uint64(t), 10)
			case uint16:
				str = strconv.FormatUint(uint64(t), 10)
			case uint8:
				str = strconv.FormatUint(uint64(t), 10)
			case int64:
				str = strconv.FormatInt(int64(t), 10)
			case int32:
				str = strconv.FormatInt(int64(t), 10)
			case int16:
				str = strconv.FormatInt(int64(t), 10)
			case int8:
				str = strconv.FormatInt(int64(t), 10)
			case float64:
				str = floatToString(t)
			case float32:
				str = floatToString(float64(t))
			default:
				panic(fmt.Sprintf("%s: unsupported type %T", op, t))
			}
			vm.output.WriteString(str)
		case Not:
			res := 0
			if frame.popI32() == 0 {
				res = 1
			}
			frame.push(res)
		case U64Add, U64Sub, U64Mul, U64Div, U64Mod, U64Cmp:
			arg2 := frame.popU64()
			arg1 := frame.popU64()
			switch op {
			case U64Add:
				frame.push(arg1 + arg2)
			case U64Sub:
				frame.push(arg1 - arg2)
			case U64Mul:
				frame.push(arg1 * arg2)
			case U64Div:
				frame.push(arg1 / arg2)
			case U64Mod:
				frame.push(arg1 % arg2)
			case U64Cmp:
				res := int32(0)
				if arg1 < arg2 {
					res = -1
				} else if arg1 > arg2 {
					res = 1
				}
				frame.push(res)
			}
		case U32Add, U32Sub, U32Mul, U32Div, U32Mod, U32Cmp:
			arg2 := frame.popU32()
			arg1 := frame.popU32()
			switch op {
			case U32Add:
				frame.push(arg1 + arg2)
			case U32Sub:
				frame.push(arg1 - arg2)
			case U32Mul:
				frame.push(arg1 * arg2)
			case U32Div:
				frame.push(arg1 / arg2)
			case U32Mod:
				frame.push(arg1 % arg2)
			case U32Cmp:
				res := int32(0)
				if arg1 < arg2 {
					res = -1
				} else if arg1 > arg2 {
					res = 1
				}
				frame.push(res)
			}
		case U16Add, U16Sub, U16Mul, U16Div, U16Mod, U16Cmp:
			arg2 := frame.popU16()
			arg1 := frame.popU16()
			switch op {
			case U16Add:
				frame.push(arg1 + arg2)
			case U16Sub:
				frame.push(arg1 - arg2)
			case U16Mul:
				frame.push(arg1 * arg2)
			case U16Div:
				frame.push(arg1 / arg2)
			case U16Mod:
				frame.push(arg1 % arg2)
			case U16Cmp:
				res := int32(0)
				if arg1 < arg2 {
					res = -1
				} else if arg1 > arg2 {
					res = 1
				}
				frame.push(res)
			}
		case U8Add, U8Sub, U8Mul, U8Div, U8Mod, U8Cmp:
			arg2 := frame.popU8()
			arg1 := frame.popU8()
			switch op {
			case U8Add:
				frame.push(arg1 + arg2)
			case U8Sub:
				frame.push(arg1 - arg2)
			case U8Mul:
				frame.push(arg1 * arg2)
			case U8Div:
				frame.push(arg1 / arg2)
			case U8Mod:
				frame.push(arg1 % arg2)
			case U8Cmp:
				res := int32(0)
				if arg1 < arg2 {
					res = -1
				} else if arg1 > arg2 {
					res = 1
				}
				frame.push(res)
			}
		case I64Add, I64Sub, I64Mul, I64Div, I64Mod, I64Cmp:
			arg2 := frame.popI64()
			arg1 := frame.popI64()
			switch op {
			case I64Add:
				frame.push(arg1 + arg2)
			case I64Sub:
				frame.push(arg1 - arg2)
			case I64Mul:
				frame.push(arg1 * arg2)
			case I64Div:
				frame.push(arg1 / arg2)
			case I64Mod:
				frame.push(arg1 % arg2)
			case U64Cmp:
				res := int32(0)
				if arg1 < arg2 {
					res = -1
				} else if arg1 > arg2 {
					res = 1
				}
				frame.push(res)
			}
		case I32Add, I32Sub, I32Mul, I32Div, I32Mod, I32Cmp:
			arg2 := frame.popI32()
			arg1 := frame.popI32()
			switch op {
			case I32Add:
				frame.push(arg1 + arg2)
			case I32Sub:
				frame.push(arg1 - arg2)
			case I32Mul:
				frame.push(arg1 * arg2)
			case I32Div:
				frame.push(arg1 / arg2)
			case I32Mod:
				frame.push(arg1 % arg2)
			case I32Cmp:
				res := int32(0)
				if arg1 < arg2 {
					res = -1
				} else if arg1 > arg2 {
					res = 1
				}
				frame.push(res)
			}
		case I16Add, I16Sub, I16Mul, I16Div, I16Mod, I16Cmp:
			arg2 := frame.popI16()
			arg1 := frame.popI16()
			switch op {
			case I16Add:
				frame.push(arg1 + arg2)
			case I16Sub:
				frame.push(arg1 - arg2)
			case I16Mul:
				frame.push(arg1 * arg2)
			case I16Div:
				frame.push(arg1 / arg2)
			case I16Mod:
				frame.push(arg1 % arg2)
			case I16Cmp:
				res := int32(0)
				if arg1 < arg2 {
					res = -1
				} else if arg1 > arg2 {
					res = 1
				}
				frame.push(res)
			}
		case I8Add, I8Sub, I8Mul, I8Div, I8Mod, I8Cmp:
			arg2 := frame.popI8()
			arg1 := frame.popI8()
			switch op {
			case I8Add:
				frame.push(arg1 + arg2)
			case I8Sub:
				frame.push(arg1 - arg2)
			case I8Mul:
				frame.push(arg1 * arg2)
			case I8Div:
				frame.push(arg1 / arg2)
			case I8Mod:
				frame.push(arg1 % arg2)
			case I8Cmp:
				res := int32(0)
				if arg1 < arg2 {
					res = -1
				} else if arg1 > arg2 {
					res = 1
				}
				frame.push(res)
			}
		case F64Add, F64Sub, F64Mul, F64Div, F64Cmp:
			arg2 := frame.popF64()
			arg1 := frame.popF64()
			switch op {
			case F64Add:
				frame.push(arg1 + arg2)
			case F64Sub:
				frame.push(arg1 - arg2)
			case F64Mul:
				frame.push(arg1 * arg2)
			case F64Div:
				frame.push(arg1 / arg2)
			case F64Cmp:
				res := int32(0)
				if arg1 < arg2 {
					res = -1
				} else if arg1 > arg2 {
					res = 1
				}
				frame.push(res)
			}
		case F32Add, F32Sub, F32Mul, F32Div, F32Cmp:
			arg2 := frame.popF32()
			arg1 := frame.popF32()
			switch op {
			case F32Add:
				frame.push(arg1 + arg2)
			case F32Sub:
				frame.push(arg1 - arg2)
			case F32Mul:
				frame.push(arg1 * arg2)
			case F32Div:
				frame.push(arg1 / arg2)
			case F32Cmp:
				res := int32(0)
				if arg1 < arg2 {
					res = -1
				} else if arg1 > arg2 {
					res = 1
				}
				frame.push(res)
			}
		case CmpEq, CmpNe, CmpGt, CmpGe, CmpLt, CmpLe:
			arg := frame.popI32()
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
			frame.push(res)
		case NumCast:
			arg2 := frame.pop()
			arg1 := frame.pop()

			intVal := int64(0)
			floatVal := float64(0)
			floatNumber := false
			err := false

			switch t1 := arg1.(type) {
			case uint64:
				intVal = int64(t1)
			case uint32:
				intVal = int64(t1)
			case uint16:
				intVal = int64(t1)
			case uint8:
				intVal = int64(t1)
			case int64:
				intVal = int64(t1)
			case int32:
				intVal = int64(t1)
			case int16:
				intVal = int64(t1)
			case int8:
				intVal = int64(t1)
			case float64:
				floatVal = float64(t1)
				floatNumber = true
			case float32:
				floatVal = float64(t1)
				floatNumber = true
			default:
				err = true
			}

			if !err {
				t2 := reflect.TypeOf(arg2).Kind()
				switch t2 {
				case reflect.Uint64:
					if floatNumber {
						frame.push(uint64(floatVal))
					} else {
						frame.push(uint64(intVal))
					}
				case reflect.Uint32:
					if floatNumber {
						frame.push(uint32(floatVal))
					} else {
						frame.push(uint32(intVal))
					}
				case reflect.Uint16:
					if floatNumber {
						frame.push(uint16(floatVal))
					} else {
						frame.push(uint16(intVal))
					}
				case reflect.Uint8:
					if floatNumber {
						frame.push(uint8(floatVal))
					} else {
						frame.push(uint8(intVal))
					}
				case reflect.Int64:
					if floatNumber {
						frame.push(int64(floatVal))
					} else {
						frame.push(int64(intVal))
					}
				case reflect.Int32:
					if floatNumber {
						frame.push(int32(floatVal))
					} else {
						frame.push(int32(intVal))
					}
				case reflect.Int16:
					if floatNumber {
						frame.push(int16(floatVal))
					} else {
						frame.push(int16(intVal))
					}
				case reflect.Int8:
					if floatNumber {
						frame.push(int8(floatVal))
					} else {
						frame.push(int8(intVal))
					}
				case reflect.Float64:
					if floatNumber {
						frame.push(float64(floatVal))
					} else {
						frame.push(float64(intVal))
					}
				case reflect.Float32:
					if floatNumber {
						frame.push(float32(floatVal))
					} else {
						frame.push(float32(intVal))
					}
				default:
					err = true
				}
			}

			if err {
				panic(fmt.Sprintf("%s: %T cannot be cast to %T", op, arg1, arg2))
			}
		case NewStruct:
			desc := frame.popStructDescriptor()
			obj := &StructObject{Name: desc.Name}
			obj.Fields = make([]interface{}, desc.FieldCount)
			for i := desc.FieldCount - 1; i >= 0; i-- {
				obj.Fields[i] = frame.pop()
			}
			frame.push(obj)
		case U64Load:
			frame.push(in.U64())
		case U32Load:
			frame.push(in.U32())
		case U16Load:
			frame.push(in.U16())
		case U8Load:
			frame.push(in.U8())
		case I64Load:
			frame.push(in.I64())
		case I32Load:
			frame.push(in.I32())
		case I16Load:
			frame.push(in.I16())
		case I8Load:
			frame.push(in.I8())
		case ConstLoad:
			addr := in.Addr()
			frame.push(frame.mod.Constants[addr])
		case GlobalLoad:
			addr := in.Addr()
			frame.push(frame.mod.Globals[addr])
		case FieldLoad:
			storage := frame.popStructObject()
			frame.push(storage.Get(in.Addr()))
		case FieldStore:
			storage := frame.popStructObject()
			storage.Put(in.Addr(), frame.pop())
		case GlobalStore:
			addr := in.Addr()
			frame.mod.Globals[addr] = frame.pop()
		case Load:
			addr := in.Addr()
			frame.push(frame.locals[addr])
		case Store:
			addr := in.Addr()
			frame.locals[addr] = frame.pop()
		case SetMod:
			frame.mod = vm.code.Modules[in.Addr()]
		case Goto:
			ip2 = in.Addr()
		case IfFalse:
			arg := frame.popI32()
			if arg == 0 {
				ip2 = in.Addr()
			}
		case IfTrue:
			arg := frame.popI32()
			if arg != 0 {
				ip2 = in.Addr()
			}
		case Call:
			mod := frame.mod
			addr := in.Addr()
			fun := mod.Functions[addr]
			newFrame := newFrame(mod, fun)
			newFrame.locals = make([]interface{}, fun.LocalCount+fun.ArgCount)
			for i := fun.ArgCount - 1; i >= 0; i-- {
				newFrame.locals[i] = frame.pop()
			}
			frame.ip++ // Execute next instruction after return
			frame = newFrame
			vm.pushFrame(newFrame)
			ip2 = 0
		default:
			panic(fmt.Sprintf("%s: not implemented", op))
		}
		frame.ip = ip2
	}
}
