package vm

import (
	"fmt"
)

type frame struct {
	mod      *ModuleObject   // Current module
	fun      *FunctionObject // Current function
	ip       int             // Instruction pointer
	locals   MemoryRegion
	operands MemoryRegion
}

func newFrame(mod *ModuleObject, fun *FunctionObject) *frame {
	return &frame{mod: mod, fun: fun}
}

func (f *frame) end() bool {
	return f.ip >= f.instrCount()
}

func (f *frame) instrCount() int {
	return len(f.fun.Code)
}

func (f *frame) instr() Instruction {
	return f.fun.Code[f.ip]
}

func (f *frame) push(arg interface{}) {
	f.operands = append(f.operands, arg)
}

func (f *frame) peek() interface{} {
	idx := len(f.operands) - 1
	return f.operands[idx]
}

func (f *frame) pop() interface{} {
	idx := len(f.operands) - 1
	arg := f.operands[idx]
	f.operands = f.operands[:idx]
	return arg
}

func (f *frame) popU64() uint64 {
	arg := f.pop()
	argInt, ok := arg.(uint64)
	if ok {
		return argInt
	}
	panic(typeErr(argInt, arg))
}

func (f *frame) popU32() uint32 {
	arg := f.pop()
	argInt, ok := arg.(uint32)
	if ok {
		return argInt
	}
	panic(typeErr(argInt, arg))
}

func (f *frame) popU16() uint16 {
	arg := f.pop()
	argInt, ok := arg.(uint16)
	if ok {
		return argInt
	}
	panic(typeErr(argInt, arg))
}

func (f *frame) popU8() uint8 {
	arg := f.pop()
	argInt, ok := arg.(uint8)
	if ok {
		return argInt
	}
	panic(typeErr(argInt, arg))
}

func (f *frame) popI64() int64 {
	arg := f.pop()
	argInt, ok := arg.(int64)
	if ok {
		return argInt
	}
	panic(typeErr(argInt, arg))
}

func (f *frame) popI32() int32 {
	arg := f.pop()
	argInt, ok := arg.(int32)
	if ok {
		return argInt
	}
	panic(typeErr(argInt, arg))
}

func (f *frame) popI16() int16 {
	arg := f.pop()
	argInt, ok := arg.(int16)
	if ok {
		return argInt
	}
	panic(typeErr(argInt, arg))
}

func (f *frame) popI8() int8 {
	arg := f.pop()
	argInt, ok := arg.(int8)
	if ok {
		return argInt
	}
	panic(typeErr(argInt, arg))
}

func (f *frame) popF64() float64 {
	arg := f.pop()
	argFloat, ok := arg.(float64)
	if ok {
		return argFloat
	}
	panic(typeErr(argFloat, arg))
}

func (f *frame) popF32() float32 {
	arg := f.pop()
	argFloat, ok := arg.(float32)
	if ok {
		return argFloat
	}
	panic(typeErr(argFloat, arg))
}

func (f *frame) popBool() bool {
	arg := f.pop()
	argBool, ok := arg.(bool)
	if ok {
		return argBool
	}
	panic(typeErr(argBool, arg))
}

func (f *frame) popStructObject() *StructObject {
	arg := f.pop()
	argStructObject, ok := arg.(*StructObject)
	if ok {
		return argStructObject
	}
	panic(typeErr(argStructObject, arg))
}

func (f *frame) popStructDescriptor() *StructDescriptor {
	arg := f.pop()
	argStructDescriptor, ok := arg.(*StructDescriptor)
	if ok {
		return argStructDescriptor
	}
	panic(typeErr(argStructDescriptor, arg))
}

func typeErr(expected interface{}, got interface{}) string {
	return fmt.Sprintf("internal error: expected type %T but got %T", expected, got)
}
