package llvm

import (
	"fmt"
	"strings"

	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
	"llvm.org/llvm/bindings/go/llvm"
)

type codeBuilder struct {
	b              llvm.Builder
	mod            llvm.Module
	inFunction     bool
	signature      bool
	values         map[*ir.Symbol]llvm.Value
	typeDecls      map[*ir.Symbol]llvm.Type
	loopConditions []llvm.BasicBlock
	loopExits      []llvm.BasicBlock
}

const ptrFieldIndex = 0
const lenFieldIndex = 1

// Build LLVM code.
func Build(set *ir.ModuleSet, outfile string) {
	if err := llvm.InitializeNativeTarget(); err != nil {
		panic(err)
	}

	if err := llvm.InitializeNativeAsmPrinter(); err != nil {
		panic(err)
	}

	triple := llvm.DefaultTargetTriple()
	target, err := llvm.GetTargetFromTriple(triple)
	if err != nil {
		panic(err)
	}
	targetMachine := target.CreateTargetMachine(triple, "", "", llvm.CodeGenLevelNone, llvm.RelocDefault, llvm.CodeModelDefault)

	// TODO:
	// - Build multiple modules
	// - Write object/assembly files to specified output directory
	// - Invoke system linker to build executable
	// - Better error handling
	//

	cb := &codeBuilder{b: llvm.NewBuilder()}
	cb.values = make(map[*ir.Symbol]llvm.Value)
	cb.typeDecls = make(map[*ir.Symbol]llvm.Type)

	cb.b = llvm.NewBuilder()
	mod := set.Modules[0]
	cb.buildModule(mod)

	ext := filepath.Ext(mod.Path)
	filename := outfile
	if len(filename) == 0 {
		filename = strings.TrimSuffix(mod.Path, ext)
	}
	objectfile := filename + ".o"
	outputmode := llvm.ObjectFile

	if code, err := targetMachine.EmitToMemoryBuffer(cb.mod, outputmode); err == nil {
		if writeErr := ioutil.WriteFile(objectfile, code.Bytes(), 0644); writeErr != nil {
			panic(writeErr)
		}

		cmd := exec.Command("gcc", objectfile, "-o", filename)
		if linkOutput, linkErr := cmd.CombinedOutput(); linkErr != nil {
			panic(fmt.Sprintf("Err: %s\nOutput: %s", linkErr, linkOutput))
		}
	} else {
		panic(err)
	}
}

func checkEndsWithBranchStmt(stmts []ir.Stmt) bool {
	n := len(stmts)
	if n > 0 {
		stmt := stmts[n-1]
		switch stmt.(type) {
		case *ir.ReturnStmt, *ir.BranchStmt:
			return true
		}
	}
	return false
}

func checkElseEndsWithBranchStmt(stmt *ir.IfStmt) bool {
	body := stmt.Body
	if stmt.Else != nil {
		switch t := stmt.Else.(type) {
		case *ir.IfStmt:
			return checkElseEndsWithBranchStmt(t)
		case *ir.BlockStmt:
			body = t
		default:
			panic(fmt.Sprintf("Unhandled IfStmt else %T", t))
		}
	}
	return checkEndsWithBranchStmt(body.Stmts)
}

func (cb *codeBuilder) buildModule(mod *ir.Module) {
	cb.mod = llvm.NewModule(mod.FQN)
	cb.inFunction = false

	cb.signature = true
	for _, decl := range mod.Decls {
		cb.buildDecl(decl)
	}

	cb.signature = false
	for _, decl := range mod.Decls {
		cb.buildDecl(decl)
	}

	if err := llvm.VerifyModule(cb.mod, llvm.ReturnStatusAction); err != nil {
		panic(err)
	}

	cb.mod.Dump()
}

func (cb *codeBuilder) buildDecl(decl ir.Decl) {
	switch t := decl.(type) {
	case *ir.ValTopDecl:
		cb.buildValTopDecl(t)
	case *ir.ValDecl:
		cb.buildValDecl(t)
	case *ir.FuncDecl:
		cb.buildFuncDecl(t)
	case *ir.StructDecl:
		cb.buildStructDecl(t)
	default:
		panic(fmt.Sprintf("Unhandled decl %T", decl))
	}
}

func (cb *codeBuilder) buildValTopDecl(decl *ir.ValTopDecl) {
	sym := decl.Sym
	if cb.signature {
		loc := llvm.AddGlobal(cb.mod, cb.toLLVMType(sym.T), sym.Name)

		switch decl.Visibility.ID {
		case token.Public:
			loc.SetLinkage(llvm.ExternalLinkage)
		case token.Private:
			loc.SetLinkage(llvm.InternalLinkage)
		}

		cb.values[sym] = loc
		return
	}

	init := cb.buildExprVal(decl.Initializer)
	loc := cb.values[sym]
	loc.SetInitializer(init)
}

func (cb *codeBuilder) buildValDecl(decl *ir.ValDecl) {
	sym := decl.Sym
	loc := cb.b.CreateAlloca(cb.toLLVMType(sym.T), sym.Name)
	cb.values[decl.Sym] = loc

	init := cb.buildExprVal(decl.Initializer)
	cb.b.CreateStore(init, loc)
}

func (cb *codeBuilder) buildFuncDecl(decl *ir.FuncDecl) {
	fun := cb.mod.NamedFunction(decl.Name.Literal)

	if cb.signature {
		if !fun.IsNil() {
			return
		}

		var paramTypes []llvm.Type
		for _, p := range decl.Params {
			paramTypes = append(paramTypes, cb.toLLVMType(p.Type.Type()))
		}
		retType := cb.toLLVMType(decl.TReturn.Type())

		funType := llvm.FunctionType(retType, paramTypes, false)
		fun := llvm.AddFunction(cb.mod, decl.Name.Literal, funType)

		switch decl.Visibility.ID {
		case token.Public:
			fun.SetLinkage(llvm.ExternalLinkage)
		case token.Private:
			fun.SetLinkage(llvm.InternalLinkage)
		}

		return
	} else if decl.SignatureOnly() {
		return
	}

	block := llvm.AddBasicBlock(fun, ".entry")
	cb.b.SetInsertPointAtEnd(block)

	for i, p := range fun.Params() {
		sym := decl.Params[i].Sym
		loc := cb.b.CreateAlloca(p.Type(), sym.Name)
		cb.b.CreateStore(p, loc)
		cb.values[sym] = loc
	}

	cb.inFunction = true
	cb.buildBlockStmt(decl.Body)
	if checkEndsWithBranchStmt(decl.Body.Stmts) {
		cb.b.GetInsertBlock().EraseFromParent() // Remove empty basic block created by last return statement
	}
	cb.inFunction = false
}

func (cb *codeBuilder) buildStructDecl(decl *ir.StructDecl) {
	if cb.signature {
		structt := cb.mod.Context().StructCreateNamed(decl.Name.Literal)
		cb.typeDecls[decl.Sym] = structt
		return
	}

	structt := cb.typeDecls[decl.Sym]

	var types []llvm.Type
	for _, field := range decl.Fields {
		types = append(types, cb.toLLVMType(field.Sym.T))
	}

	structt.StructSetBody(types, false)
}

func (cb *codeBuilder) buildStmtList(stmts []ir.Stmt) {
	for _, stmt := range stmts {
		cb.buildStmt(stmt)
	}
}

func (cb *codeBuilder) buildStmt(stmt ir.Stmt) {
	switch t := stmt.(type) {
	case *ir.BlockStmt:
		cb.buildBlockStmt(t)
	case *ir.DeclStmt:
		cb.buildDeclStmt(t)
	case *ir.IfStmt:
		cb.buildIfStmt(t)
	case *ir.ForStmt:
		cb.buildForStmt(t)
	case *ir.ReturnStmt:
		cb.buildReturnStmt(t)
	case *ir.BranchStmt:
		cb.buildBranchStmt(t)
	case *ir.AssignStmt:
		cb.buildAssignStmt(t)
	case *ir.ExprStmt:
		cb.buildExprStmt(t)
	default:
		panic(fmt.Sprintf("Unhandled stmt %T", t))
	}
}

func (cb *codeBuilder) buildBlockStmt(stmt *ir.BlockStmt) {
	cb.buildStmtList(stmt.Stmts)
}

func (cb *codeBuilder) buildDeclStmt(stmt *ir.DeclStmt) {
	cb.buildDecl(stmt.D)
}

func (cb *codeBuilder) buildIfStmt(stmt *ir.IfStmt) {
	cond := cb.buildExprVal(stmt.Cond)

	fun := cb.b.GetInsertBlock().Parent()
	iftrue := llvm.AddBasicBlock(fun, ".if_true")
	join := llvm.AddBasicBlock(fun, ".join")

	var iffalse llvm.BasicBlock
	if stmt.Else != nil {
		iffalse = llvm.AddBasicBlock(fun, ".if_false")
		cb.b.CreateCondBr(cond, iftrue, iffalse)
	} else {
		cb.b.CreateCondBr(cond, iftrue, join)
	}

	cb.b.SetInsertPointAtEnd(iftrue)
	cb.buildBlockStmt(stmt.Body)
	if checkEndsWithBranchStmt(stmt.Body.Stmts) {
		cb.b.GetInsertBlock().EraseFromParent() // Remove empty basic block created by branch statement
	} else {
		cb.b.CreateBr(join)
	}

	var last llvm.BasicBlock

	if stmt.Else != nil {
		last = fun.LastBasicBlock()
		iffalse.MoveAfter(last)
		cb.b.SetInsertPointAtEnd(iffalse)
		cb.buildStmt(stmt.Else)

		if checkElseEndsWithBranchStmt(stmt) {
			cb.b.GetInsertBlock().EraseFromParent() // Remove empty basic block created by branch statement
		} else {
			cb.b.CreateBr(join)
		}
	}

	last = fun.LastBasicBlock()
	join.MoveAfter(last)
	cb.b.SetInsertPointAtEnd(join)
}

func (cb *codeBuilder) buildForStmt(stmt *ir.ForStmt) {
	if stmt.Init != nil {
		cb.buildValDecl(stmt.Init)
	}

	fun := cb.b.GetInsertBlock().Parent()
	loop := llvm.AddBasicBlock(fun, ".loop")
	exit := llvm.AddBasicBlock(fun, ".loop_exit")

	var inc llvm.BasicBlock
	if stmt.Inc != nil {
		inc = llvm.AddBasicBlock(fun, ".loop_inc")
	}

	var cond llvm.BasicBlock
	if stmt.Cond != nil {
		cond = llvm.AddBasicBlock(fun, ".loop_cond")
	}

	if stmt.Inc != nil {
		cb.loopConditions = append(cb.loopConditions, inc)
	} else if stmt.Cond != nil {
		cb.loopConditions = append(cb.loopConditions, cond)
	} else {
		cb.loopConditions = append(cb.loopConditions, loop)
	}

	cb.loopExits = append(cb.loopExits, exit)

	if stmt.Cond != nil {
		cb.b.CreateBr(cond)
		cb.b.SetInsertPointAtEnd(cond)
		loopCond := cb.buildExprVal(stmt.Cond)
		cb.b.CreateCondBr(loopCond, loop, exit)
	} else {
		cb.b.CreateBr(loop)
	}

	last := fun.LastBasicBlock()
	loop.MoveAfter(last)
	cb.b.SetInsertPointAtEnd(loop)
	cb.buildBlockStmt(stmt.Body)

	if stmt.Inc != nil {
		cb.b.CreateBr(inc)
		last = fun.LastBasicBlock()
		inc.MoveAfter(last)
		cb.b.SetInsertPointAtEnd(inc)
		cb.buildStmt(stmt.Inc)
	}

	if stmt.Cond != nil {
		cb.b.CreateBr(cond)
	} else {
		cb.b.CreateBr(loop)
	}

	last = fun.LastBasicBlock()
	exit.MoveAfter(last)
	cb.b.SetInsertPointAtEnd(exit)

	cb.loopConditions = cb.loopConditions[:len(cb.loopConditions)-1]
	cb.loopExits = cb.loopExits[:len(cb.loopExits)-1]
}

func (cb *codeBuilder) buildReturnStmt(stmt *ir.ReturnStmt) {
	if stmt.X != nil {
		val := cb.buildExprVal(stmt.X)
		cb.b.CreateRet(val)
	} else {
		cb.b.CreateRetVoid()
	}
	fun := cb.b.GetInsertBlock().Parent()
	block := llvm.AddBasicBlock(fun, "")
	cb.b.SetInsertPointAtEnd(block)
}

func (cb *codeBuilder) buildBranchStmt(stmt *ir.BranchStmt) {
	if stmt.Tok.ID == token.Continue {
		block := cb.loopConditions[len(cb.loopConditions)-1]
		cb.b.CreateBr(block)
	} else if stmt.Tok.ID == token.Break {
		block := cb.loopExits[len(cb.loopExits)-1]
		cb.b.CreateBr(block)
	} else {
		panic(fmt.Sprintf("Unhandled BranchStmt %s", stmt.Tok))
	}
	fun := cb.b.GetInsertBlock().Parent()
	block := llvm.AddBasicBlock(fun, "")
	cb.b.SetInsertPointAtEnd(block)
}

func (cb *codeBuilder) buildAssignStmt(stmt *ir.AssignStmt) {
	loc := cb.buildExprPtr(stmt.Left)
	val := cb.buildExprVal(stmt.Right)

	switch stmt.Assign.ID {
	case token.AddAssign, token.SubAssign, token.MulAssign, token.DivAssign, token.ModAssign:
		left := cb.b.CreateLoad(loc, "")
		val = cb.createArithmeticOp(stmt.Assign.ID, stmt.Left.Type(), left, val)
	}

	cb.b.CreateStore(val, loc)
}

func (cb *codeBuilder) buildExprStmt(stmt *ir.ExprStmt) {
	cb.buildExprVal(stmt.X)
}

func (cb *codeBuilder) llvmEnumAttribute(name string, val uint64) llvm.Attribute {
	kind := llvm.AttributeKindID(name)
	ctx := llvm.GlobalContext()
	attr := ctx.CreateEnumAttribute(kind, val)
	return attr
}

func (cb *codeBuilder) toLLVMType(t ir.Type) llvm.Type {
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
	case ir.TStruct:
		tstruct := t.(*ir.StructType)
		if res, ok := cb.typeDecls[tstruct.Sym]; ok {
			return res
		}
		panic(fmt.Sprintf("Failed to find named type %s", t))
	case ir.TArray:
		tarray := t.(*ir.ArrayType)
		telem := cb.toLLVMType(tarray.Elem)
		return llvm.ArrayType(telem, tarray.Size)
	case ir.TSlice:
		tslice := t.(*ir.SliceType)
		telem := cb.toLLVMType(tslice.Elem)
		tptr := llvm.PointerType(telem, 0)
		tsize := cb.toLLVMType(ir.TBuiltinInt32)
		return llvm.StructType([]llvm.Type{tptr, tsize}, false)
	case ir.TPointer:
		tptr := t.(*ir.PointerType)
		tunderlying := cb.toLLVMType(tptr.Underlying)
		return llvm.PointerType(tunderlying, 0)
	case ir.TFunc:
		tfun := t.(*ir.FuncType)
		var params []llvm.Type
		for _, param := range tfun.Params {
			params = append(params, cb.toLLVMType(param))
		}
		ret := cb.toLLVMType(tfun.Return)
		return llvm.PointerType(llvm.FunctionType(ret, params, false), 0)
	default:
		panic(fmt.Sprintf("Unhandled type %s", t.ID()))
	}
}

func (cb *codeBuilder) buildExprVal(expr ir.Expr) llvm.Value {
	return cb.buildExpr(expr, true)
}

func (cb *codeBuilder) buildExprPtr(expr ir.Expr) llvm.Value {
	return cb.buildExpr(expr, false)
}

func (cb *codeBuilder) buildExpr(expr ir.Expr, load bool) llvm.Value {
	switch t := expr.(type) {
	case *ir.BinaryExpr:
		return cb.buildBinaryExpr(t)
	case *ir.UnaryExpr:
		return cb.buildUnaryExpr(t, load)
	case *ir.BasicLit:
		return cb.buildBasicLit(t)
	case *ir.StructLit:
		return cb.buildStructLit(t)
	case *ir.ArrayLit:
		return cb.buildArrayLit(t)
	case *ir.Ident:
		return cb.buildIdent(t, load)
	case *ir.DotExpr:
		return cb.buildDotExpr(t, load)
	case *ir.CastExpr:
		return cb.buildCastExpr(t)
	case *ir.LenExpr:
		return cb.buildLenExpr(t)
	case *ir.FuncCall:
		return cb.buildFuncCall(t)
	case *ir.AddressExpr:
		return cb.buildAddressExpr(t, load)
	case *ir.IndexExpr:
		return cb.buildIndexExpr(t, load)
	case *ir.SliceExpr:
		return cb.buildSliceExpr(t)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", t))
	}
}

func (cb *codeBuilder) createArithmeticOp(op token.ID, t ir.Type, left llvm.Value, right llvm.Value) llvm.Value {
	switch op {
	case token.Add, token.AddAssign:
		if ir.IsFloatingType(t) {
			return cb.b.CreateFAdd(left, right, "")
		}
		return cb.b.CreateAdd(left, right, "")
	case token.Sub, token.SubAssign:
		if ir.IsFloatingType(t) {
			return cb.b.CreateFSub(left, right, "")
		}
		return cb.b.CreateSub(left, right, "")
	case token.Mul, token.MulAssign:
		if ir.IsFloatingType(t) {
			return cb.b.CreateFMul(left, right, "")
		}
		return cb.b.CreateMul(left, right, "")
	case token.Div:
		if ir.IsFloatingType(t) {
			return cb.b.CreateFDiv(left, right, "")
		} else if ir.IsUnsignedType(t) {
			return cb.b.CreateUDiv(left, right, "")
		}
		return cb.b.CreateSDiv(left, right, "")
	case token.Mod:
		if ir.IsFloatingType(t) {
			return cb.b.CreateFRem(left, right, "")
		} else if ir.IsUnsignedType(t) {
			return cb.b.CreateURem(left, right, "")
		}
		return cb.b.CreateSRem(left, right, "")
	}
	panic(fmt.Sprintf("Unhandled arithmetic op %s", op))
}

func floatPredicate(op token.ID) llvm.FloatPredicate {
	switch op {
	case token.Eq:
		return llvm.FloatUEQ
	case token.Neq:
		return llvm.FloatUNE
	case token.Gt:
		return llvm.FloatUGT
	case token.GtEq:
		return llvm.FloatUGE
	case token.Lt:
		return llvm.FloatULT
	case token.LtEq:
		return llvm.FloatULE
	}
	panic(fmt.Sprintf("Unhandled float predicate %s", op))
}

func intPredicate(op token.ID, t ir.Type) llvm.IntPredicate {
	switch op {
	case token.Eq:
		return llvm.IntEQ
	case token.Neq:
		return llvm.IntNE
	case token.Gt:
		if ir.IsSignedType(t) {
			return llvm.IntSGT
		}
		return llvm.IntUGT
	case token.GtEq:
		if ir.IsSignedType(t) {
			return llvm.IntSGE
		}
		return llvm.IntUGE
	case token.Lt:
		if ir.IsSignedType(t) {
			return llvm.IntSLT
		}
		return llvm.IntULT
	case token.LtEq:
		if ir.IsSignedType(t) {
			return llvm.IntSLT
		}
		return llvm.IntULT
	}
	panic(fmt.Sprintf("Unhandled int predicate %s", op))
}

func (cb *codeBuilder) buildBinaryExpr(expr *ir.BinaryExpr) llvm.Value {
	left := cb.buildExprVal(expr.Left)

	switch expr.Op.ID {
	case token.Add, token.Sub, token.Mul, token.Div, token.Mod:
		right := cb.buildExprVal(expr.Right)
		return cb.createArithmeticOp(expr.Op.ID, expr.T, left, right)
	case token.Eq, token.Neq, token.Gt, token.GtEq, token.Lt, token.LtEq:
		right := cb.buildExprVal(expr.Right)
		if ir.IsFloatingType(expr.T) {
			return cb.b.CreateFCmp(floatPredicate(expr.Op.ID), left, right, "")
		}
		return cb.b.CreateICmp(intPredicate(expr.Op.ID, expr.Left.Type()), left, right, "")
	case token.Land, token.Lor:
		fun := cb.b.GetInsertBlock().Parent()
		leftBlock := cb.b.GetInsertBlock()
		rightBlock := llvm.AddBasicBlock(fun, "")
		join := llvm.AddBasicBlock(fun, "")

		var case1 llvm.Value
		if expr.Op.ID == token.Land {
			cb.b.CreateCondBr(left, rightBlock, join)
			case1 = llvm.ConstInt(llvm.Int1Type(), 0, false)
		} else { // Lor
			cb.b.CreateCondBr(left, join, rightBlock)
			case1 = llvm.ConstInt(llvm.Int1Type(), 1, false)
		}

		cb.b.SetInsertPointAtEnd(rightBlock)
		right := cb.buildExprVal(expr.Right)
		cb.b.CreateBr(join)

		cb.b.SetInsertPointAtEnd(join)
		phi := cb.b.CreatePHI(cb.toLLVMType(expr.T), "")
		phi.AddIncoming([]llvm.Value{case1, right}, []llvm.BasicBlock{leftBlock, rightBlock})
		return phi
	}

	panic(fmt.Sprintf("Unhandled binary op %s", expr.Op.ID))
}

func (cb *codeBuilder) buildUnaryExpr(expr *ir.UnaryExpr, load bool) llvm.Value {
	switch expr.Op.ID {
	case token.Sub:
		val := cb.buildExprVal(expr.X)
		if ir.IsFloatingType(expr.T) {
			return cb.b.CreateFNeg(val, "")
		}
		return cb.b.CreateNeg(val, "")
	case token.Lnot:
		val := cb.buildExprVal(expr.X)
		return cb.b.CreateNot(val, "")
	case token.Mul:
		val := cb.buildExprVal(expr.X)

		if load {
			return cb.b.CreateLoad(val, val.Name())
		}
		return val
	default:
		panic(fmt.Sprintf("Unhandled unary op %s", expr.Op.ID))
	}
}

func (cb *codeBuilder) buildBasicLit(expr *ir.BasicLit) llvm.Value {
	if expr.Value.ID == token.String {
		raw := expr.AsString()
		strLen := len(raw)
		var ptr llvm.Value

		if cb.inFunction {
			ptr = cb.b.CreateGlobalStringPtr(expr.AsString(), ".str")
		} else {
			typ := llvm.ArrayType(llvm.Int8Type(), strLen+1) // +1 is for null-terminator
			arr := llvm.AddGlobal(cb.mod, typ, ".str")
			arr.SetLinkage(llvm.PrivateLinkage)
			arr.SetUnnamedAddr(true)
			arr.SetGlobalConstant(true)
			arr.SetInitializer(llvm.ConstString(raw, true))

			ptr = llvm.ConstBitCast(arr, llvm.PointerType(llvm.Int8Type(), 0))
		}

		if expr.T.ID() == ir.TSlice {
			size := cb.createSliceSize(strLen)
			slice := cb.createSliceStruct(ptr, size, expr.Type())
			return slice
		} else if expr.T.ID() == ir.TPointer {
			return ptr
		}

		panic(fmt.Sprintf("Unhandled string literal type %T", expr.T))
	}

	llvmType := cb.toLLVMType(expr.T)
	if ir.IsIntegerType(expr.T) {
		val := llvm.ConstInt(llvmType, expr.AsU64(), false)
		if expr.NegatigeInteger() {
			return cb.b.CreateNeg(val, "")
		}
		return val
	} else if ir.IsFloatingType(expr.T) {
		return llvm.ConstFloat(llvmType, expr.AsF64())
	} else if expr.Value.ID == token.True {
		return llvm.ConstInt(llvmType, 1, false)
	} else if expr.Value.ID == token.False {
		return llvm.ConstInt(llvmType, 0, false)
	} else if expr.Value.ID == token.Null {
		switch t := expr.T.(type) {
		case *ir.SliceType:
			tptr := llvm.PointerType(cb.toLLVMType(t.Elem), 0)
			ptr := llvm.ConstPointerNull(tptr)
			size := cb.createSliceSize(0)
			return cb.createSliceStruct(ptr, size, expr.T)
		case *ir.PointerType, *ir.FuncType:
			return llvm.ConstPointerNull(llvmType)
		}
	}

	panic(fmt.Sprintf("Unhandled basic lit %s, type %s", expr.Value.ID, expr.T))
}

func (cb *codeBuilder) buildStructLit(expr *ir.StructLit) llvm.Value {
	tstruct := expr.T.(*ir.StructType)
	llvmType := cb.typeDecls[tstruct.Sym]
	structLit := llvm.Undef(llvmType)

	for _, field := range expr.Initializers {
		index := tstruct.FieldIndex(field.Key.Literal)
		init := cb.buildExprVal(field.Value)
		structLit = cb.b.CreateInsertValue(structLit, init, index, "")
	}

	return structLit
}

func (cb *codeBuilder) buildArrayLit(expr *ir.ArrayLit) llvm.Value {
	llvmType := cb.toLLVMType(expr.T)
	arrayLit := llvm.Undef(llvmType)

	for index, elem := range expr.Initializers {
		init := cb.buildExprVal(elem)
		arrayLit = cb.b.CreateInsertValue(arrayLit, init, index, "")
	}

	return arrayLit
}

func (cb *codeBuilder) buildIdent(expr *ir.Ident, load bool) llvm.Value {
	if expr.Sym.ID == ir.FuncSymbol {
		return cb.mod.NamedFunction(expr.Sym.Name)
	}

	if val, ok := cb.values[expr.Sym]; ok {

		if load {
			return cb.b.CreateLoad(val, expr.Sym.Name)
		}
		return val
	}

	panic(fmt.Sprintf("%s not found", expr.Sym))
}

func (cb *codeBuilder) createTempStorage(val llvm.Value) llvm.Value {
	// TODO: See if there's a better way to handle intermediary results
	loc := cb.b.CreateAlloca(val.Type(), ".tmp")
	cb.b.CreateStore(val, loc)
	return loc
}

func (cb *codeBuilder) createSliceStruct(ptr llvm.Value, size llvm.Value, t ir.Type) llvm.Value {
	llvmType := cb.toLLVMType(t)
	sliceStruct := llvm.Undef(llvmType)
	sliceStruct = cb.b.CreateInsertValue(sliceStruct, ptr, ptrFieldIndex, "")
	sliceStruct = cb.b.CreateInsertValue(sliceStruct, size, lenFieldIndex, "")
	return sliceStruct
}

func (cb *codeBuilder) createSliceSize(size int) llvm.Value {
	return llvm.ConstInt(llvm.IntType(32), uint64(size), false)
}

func (cb *codeBuilder) buildDotExpr(expr *ir.DotExpr, load bool) llvm.Value {
	switch t := expr.X.Type().(type) {
	case *ir.StructType:
		val := cb.buildExprPtr(expr.X)
		if val.Type().TypeKind() != llvm.PointerTypeKind {
			val = cb.createTempStorage(val)
		}

		index := t.FieldIndex(expr.Name.Literal())
		gep := cb.b.CreateStructGEP(val, index, "")
		if load {
			return cb.b.CreateLoad(gep, "")
		}
		return gep
	default:
		panic(fmt.Sprintf("%T not handled", t))
	}
}

func (cb *codeBuilder) buildCastExpr(expr *ir.CastExpr) llvm.Value {
	val := cb.buildExprVal(expr.X)

	to := expr.Type()
	from := expr.X.Type()
	if from.Equals(to) || (from.ID() == ir.TPointer && to.ID() == ir.TPointer) {
		return val
	}

	cmpBitSize := ir.CompareBitSize(to, from)
	toLLVM := cb.toLLVMType(to)

	var res llvm.Value
	unhandled := false

	switch {
	case ir.IsFloatingType(from):
		switch {
		case ir.IsFloatingType(to):
			if cmpBitSize > 0 {
				res = cb.b.CreateFPExt(val, toLLVM, "")
			} else {
				res = cb.b.CreateFPTrunc(val, toLLVM, "")
			}
		case ir.IsIntegerType(to):
			if ir.IsUnsignedType(to) {
				res = cb.b.CreateFPToUI(val, toLLVM, "")
			} else {
				res = cb.b.CreateFPToSI(val, toLLVM, "")
			}
		default:
			unhandled = true
		}
	case ir.IsIntegerType(from):
		switch {
		case ir.IsFloatingType(to):
			if ir.IsUnsignedType(from) {
				res = cb.b.CreateUIToFP(val, toLLVM, "")
			} else {
				res = cb.b.CreateSIToFP(val, toLLVM, "")
			}
		case ir.IsIntegerType(to):
			if cmpBitSize > 0 {
				res = cb.b.CreateSExt(val, toLLVM, "")
			} else {
				res = cb.b.CreateTrunc(val, toLLVM, "")
			}
		default:
			unhandled = true
		}
	default:
		unhandled = true
	}

	if unhandled {
		panic(fmt.Sprintf("Failed to cast from %s to %s", from, to))
	}

	return res
}

func (cb *codeBuilder) buildLenExpr(expr *ir.LenExpr) llvm.Value {
	switch t := expr.X.Type().(type) {
	case *ir.ArrayType:
		return llvm.ConstInt(llvm.Int32Type(), uint64(t.Size), false)
	case *ir.SliceType:
		val := cb.buildExprPtr(expr.X)
		if val.Type().TypeKind() != llvm.PointerTypeKind {
			val = cb.createTempStorage(val)
		}
		gep := cb.b.CreateStructGEP(val, lenFieldIndex, "")
		return cb.b.CreateLoad(gep, "")
	default:
		panic(fmt.Sprintf("%T not handled", t))
	}
}

func (cb *codeBuilder) buildFuncCall(expr *ir.FuncCall) llvm.Value {
	fun := cb.buildExprVal(expr.X)

	var args []llvm.Value
	for _, arg := range expr.Args {
		args = append(args, cb.buildExprVal(arg))
	}

	return cb.b.CreateCall(fun, args, "")
}

func (cb *codeBuilder) buildAddressExpr(expr *ir.AddressExpr, load bool) llvm.Value {
	return cb.buildExprPtr(expr.X)
}

func (cb *codeBuilder) buildIndexExpr(expr *ir.IndexExpr, load bool) llvm.Value {
	val := cb.buildExprPtr(expr.X)
	if val.Type().TypeKind() != llvm.PointerTypeKind {
		val = cb.createTempStorage(val)
	}

	index := cb.buildExprVal(expr.Index)

	var gep llvm.Value

	if expr.X.Type().ID() == ir.TSlice {
		slicePtr := cb.b.CreateStructGEP(val, ptrFieldIndex, "")
		slicePtr = cb.b.CreateLoad(slicePtr, "")
		gep = cb.b.CreateInBoundsGEP(slicePtr, []llvm.Value{index}, "")
	} else {
		gep = cb.b.CreateInBoundsGEP(val, []llvm.Value{llvm.ConstInt(llvm.Int64Type(), 0, false), index}, "")
	}

	if load {
		return cb.b.CreateLoad(gep, "")
	}

	return gep
}

func (cb *codeBuilder) buildSliceExpr(expr *ir.SliceExpr) llvm.Value {
	val := cb.buildExprPtr(expr.X)
	start := cb.buildExprVal(expr.Start)
	end := cb.buildExprVal(expr.End)

	var gep llvm.Value
	var tptr llvm.Type

	switch t := expr.X.Type().(type) {
	case *ir.ArrayType:
		gep = cb.b.CreateInBoundsGEP(val, []llvm.Value{llvm.ConstInt(llvm.Int64Type(), 0, false), start}, "")
		tptr = llvm.PointerType(cb.toLLVMType(t.Elem), 0)
	case *ir.SliceType:
		slicePtr := cb.b.CreateStructGEP(val, ptrFieldIndex, "")
		slicePtr = cb.b.CreateLoad(slicePtr, "")
		gep = cb.b.CreateInBoundsGEP(slicePtr, []llvm.Value{start}, "")
		tptr = llvm.PointerType(cb.toLLVMType(t.Elem), 0)
	default:
		panic(fmt.Sprintf("Unhandled slice type %T", t))
	}

	ptr := cb.b.CreateBitCast(gep, tptr, "")

	var size llvm.Value
	if lit, ok := expr.Start.(*ir.BasicLit); !ok || !lit.Zero() {
		size = cb.b.CreateSub(end, start, "")
	} else {
		size = end
	}

	return cb.createSliceStruct(ptr, size, expr.Type())
}
