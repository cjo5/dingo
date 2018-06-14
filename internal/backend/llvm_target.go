package backend

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jhnl/dingo/internal/ir"
	"llvm.org/llvm/bindings/go/llvm"
)

type llvmTarget struct {
	machine llvm.TargetMachine
	data    llvm.TargetData
}

type llvmTypeContext map[*ir.Symbol]llvm.Type

// NewLLVMTarget creates an LLVM backend target.
func NewLLVMTarget() ir.Target {
	if err := llvm.InitializeNativeTarget(); err != nil {
		panic(err)
	}

	if err := llvm.InitializeNativeAsmPrinter(); err != nil {
		panic(err)
	}

	target := &llvmTarget{}
	target.machine = createTargetMachine()
	target.data = target.machine.CreateTargetData()

	return target
}

func createTargetMachine() llvm.TargetMachine {
	triple := llvm.DefaultTargetTriple()
	target, err := llvm.GetTargetFromTriple(triple)
	if err != nil {
		panic(err)
	}
	return target.CreateTargetMachine(triple, "", "", llvm.CodeGenLevelNone, llvm.RelocDefault, llvm.CodeModelDefault)
}

func (target *llvmTarget) Sizeof(t ir.Type) int {
	llvmType := llvmType(t, nil)
	return int(target.data.TypeAllocSize(llvmType))
}

func (target *llvmTarget) compareBitSize(t1 llvm.Type, t2 llvm.Type) int {
	return int(target.data.TypeSizeInBits(t1)) - int(target.data.TypeSizeInBits(t2))
}

func llvmBasicType(t *ir.BasicType) llvm.Type {
	switch t.ID() {
	case ir.TVoid:
		return llvm.VoidType()
	case ir.TBool:
		return llvm.IntType(1)
	case ir.TUInt64, ir.TInt64:
		return llvm.IntType(64)
	case ir.TUInt32, ir.TInt32:
		return llvm.IntType(32)
	case ir.TUInt16, ir.TInt16:
		return llvm.IntType(16)
	case ir.TUInt8, ir.TInt8:
		return llvm.IntType(8)
	case ir.TFloat64:
		return llvm.DoubleType()
	case ir.TFloat32:
		return llvm.FloatType()
	default:
		panic(fmt.Sprintf("Unhandled basic type %s", t))
	}
}

func llvmStructType(t *ir.StructType, ctx *llvmTypeContext) llvm.Type {
	if ctx != nil {
		if res, ok := (*ctx)[t.Sym]; ok {
			return res
		}
		panic(fmt.Sprintf("Failed to find named type %s", t))
	}

	var fieldTypes []llvm.Type
	for _, field := range t.Fields {
		fieldTypes = append(fieldTypes, llvmType(field.T, ctx))
	}

	return llvm.StructType(fieldTypes, false)
}

func llvmArrayType(t *ir.ArrayType, ctx *llvmTypeContext) llvm.Type {
	telem := llvmType(t.Elem, ctx)
	return llvm.ArrayType(telem, t.Size)
}

func llvmSliceType(t *ir.SliceType, ctx *llvmTypeContext) llvm.Type {
	telem := llvmType(t.Elem, ctx)
	tptr := llvm.PointerType(telem, 0)
	tsize := llvmType(ir.TBuiltinInt32, ctx)
	return llvm.StructType([]llvm.Type{tptr, tsize}, false)
}

func llvmPointerType(t *ir.PointerType, ctx *llvmTypeContext) llvm.Type {
	var tunderlying llvm.Type
	if t.Underlying.ID() == ir.TUntyped || t.Underlying.ID() == ir.TVoid {
		tunderlying = llvm.Int8Type()
	} else {
		tunderlying = llvmType(t.Underlying, ctx)
	}
	return llvm.PointerType(tunderlying, 0)
}

func llvmFuncType(t *ir.FuncType, ctx *llvmTypeContext) llvm.Type {
	var params []llvm.Type
	for _, param := range t.Params {
		params = append(params, llvmType(param.T, ctx))
	}
	ret := llvmType(t.Return, ctx)
	return llvm.PointerType(llvm.FunctionType(ret, params, false), 0)
}

func llvmType(t1 ir.Type, ctx *llvmTypeContext) llvm.Type {
	switch t2 := t1.(type) {
	case *ir.BasicType:
		return llvmBasicType(t2)
	case *ir.StructType:
		return llvmStructType(t2, ctx)
	case *ir.ArrayType:
		return llvmArrayType(t2, ctx)
	case *ir.SliceType:
		return llvmSliceType(t2, ctx)
	case *ir.PointerType:
		return llvmPointerType(t2, ctx)
	case *ir.FuncType:
		return llvmFuncType(t2, ctx)
	default:
		panic(fmt.Sprintf("Unhandled type %s", t2))
	}
}

func mangle(sym *ir.Symbol) string {
	if sym.ID == ir.FuncSymbol {
		tfun := sym.T.(*ir.FuncType)
		if tfun.C {
			return sym.Name
		}
	}

	var b bytes.Buffer
	b.WriteString("_ZN")
	b.WriteString(mangleFQN(sym.ModFQN()))
	b.WriteString(fmt.Sprintf("%d", len(sym.Name)))
	b.WriteString(sym.Name)
	b.WriteString("E")
	return b.String()
}

func mangleFQN(fqn string) string {
	split := strings.Split(fqn, ".")
	var b bytes.Buffer
	for _, part := range split {
		b.WriteString(fmt.Sprintf("%d", len(part)))
		b.WriteString(part)
	}
	return b.String()
}
