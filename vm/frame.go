package vm

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
	argInt, _ := arg.(uint64)
	return argInt
}

func (f *frame) popU32() uint32 {
	arg := f.pop()
	argInt, _ := arg.(uint32)
	return argInt
}

func (f *frame) popU16() uint16 {
	arg := f.pop()
	argInt, _ := arg.(uint16)
	return argInt
}

func (f *frame) popU8() uint8 {
	arg := f.pop()
	argInt, _ := arg.(uint8)
	return argInt
}

func (f *frame) popI64() int64 {
	arg := f.pop()
	argInt, _ := arg.(int64)
	return argInt
}

func (f *frame) popI32() int32 {
	arg := f.pop()
	argInt, _ := arg.(int32)
	return argInt
}

func (f *frame) popI16() int16 {
	arg := f.pop()
	argInt, _ := arg.(int16)
	return argInt
}

func (f *frame) popI8() int8 {
	arg := f.pop()
	argInt, _ := arg.(int8)
	return argInt
}

func (f *frame) popF64() float64 {
	arg := f.pop()
	argFloat, _ := arg.(float64)
	return argFloat
}

func (f *frame) popF32() float32 {
	arg := f.pop()
	argFloat, _ := arg.(float32)
	return argFloat
}

func (f *frame) popFieldStorage() FieldStorage {
	arg := f.pop()
	argFieldStorage, _ := arg.(FieldStorage)
	return argFieldStorage
}

func (f *frame) popModuleObject() *ModuleObject {
	arg := f.pop()
	argModuleObject, _ := arg.(*ModuleObject)
	return argModuleObject
}
