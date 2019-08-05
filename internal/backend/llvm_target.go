package backend

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/cjo5/dingo/internal/ir"
	"github.com/cjo5/dingo/internal/token"
	"llvm.org/llvm/bindings/go/llvm"
)

type llvmTarget struct {
	machine llvm.TargetMachine
	data    llvm.TargetData
}

type llvmTypeMap map[ir.SymbolKey]llvm.Type

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

func llvmBasicType(kind ir.TypeKind) llvm.Type {
	switch kind {
	case ir.TVoid:
		return llvm.VoidType()
	case ir.TBool:
		return llvm.Int1Type()
	case ir.TNull:
		return llvm.PointerType(llvm.Int8Type(), 0)
	case ir.TUInt8, ir.TInt8:
		return llvm.Int8Type()
	case ir.TUInt16, ir.TInt16:
		return llvm.Int16Type()
	case ir.TUInt32, ir.TInt32:
		return llvm.Int32Type()
	case ir.TUInt64, ir.TInt64:
		return llvm.Int64Type()
	case ir.TUSize:
		return llvm.Int64Type()
	case ir.TFloat32:
		return llvm.FloatType()
	case ir.TFloat64:
		return llvm.DoubleType()
	case ir.TConstInt:
		return llvm.Int32Type()
	case ir.TConstFloat:
		return llvm.DoubleType()
	default:
		panic(fmt.Sprintf("Unhandled basic type %s", kind))
	}
}

func llvmSizeType() llvm.Type {
	return llvmBasicType(ir.TUSize)
}

func llvmStructType(t *ir.StructType, ctx *llvmTypeMap) llvm.Type {
	if ctx != nil {
		if res, ok := (*ctx)[t.Sym.Key]; ok {
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

func llvmArrayType(t *ir.ArrayType, ctx *llvmTypeMap) llvm.Type {
	telem := llvmType(t.Elem, ctx)
	return llvm.ArrayType(telem, t.Size)
}

func llvmSliceType(t *ir.SliceType, ctx *llvmTypeMap) llvm.Type {
	telem := llvmType(t.Elem, ctx)
	tptr := llvm.PointerType(telem, 0)
	tsize := llvmSizeType()
	return llvm.StructType([]llvm.Type{tptr, tsize}, false)
}

func llvmPointerType(t *ir.PointerType, ctx *llvmTypeMap) llvm.Type {
	var telem llvm.Type
	if t.Elem.Kind() == ir.TVoid {
		telem = llvm.Int8Type()
	} else {
		telem = llvmType(t.Elem, ctx)
	}
	return llvm.PointerType(telem, 0)
}

func llvmFuncType(t *ir.FuncType, ctx *llvmTypeMap) llvm.Type {
	var params []llvm.Type
	for _, param := range t.Params {
		params = append(params, llvmType(param.T, ctx))
	}
	ret := llvmType(t.Return, ctx)
	return llvm.PointerType(llvm.FunctionType(ret, params, false), 0)
}

func llvmType(t1 ir.Type, ctx *llvmTypeMap) llvm.Type {
	switch t2 := t1.(type) {
	case *ir.AliasType:
		return llvmType(t2.T, ctx)
	case *ir.BasicType:
		return llvmBasicType(t2.Kind())
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

func isExternalLLVMLinkage(sym *ir.Symbol) bool {
	if sym.Public || !sym.IsDefined() || sym.ABI != ir.DGABI {
		return true
	}
	return false
}

func llvmLinkage(sym *ir.Symbol) llvm.Linkage {
	if isExternalLLVMLinkage(sym) {
		return llvm.ExternalLinkage
	}
	return llvm.InternalLinkage
}

func mangle(sym *ir.Symbol) string {
	if sym.ABI == ir.CABI {
		return sym.Name
	}
	var b bytes.Buffer
	b.WriteString("_ZN")
	b.WriteString(mangleFQN(sym.ModFQN))
	b.WriteString(fmt.Sprintf("%d", len(sym.Name)))
	b.WriteString(sym.Name)
	b.WriteString("E")
	return b.String()
}

func mangleFQN(fqn string) string {
	split := strings.Split(fqn, token.ScopeSep.String())
	var b bytes.Buffer
	for _, part := range split {
		b.WriteString(fmt.Sprintf("%d", len(part)))
		b.WriteString(part)
	}
	return b.String()
}
