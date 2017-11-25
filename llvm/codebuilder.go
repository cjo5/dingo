package llvm

import (
	"fmt"
	"strings"

	"github.com/jhnl/dingo/common"

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
	cb.mod = llvm.NewModule(mod.Name.Literal)
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

	switch t := decl.Initializer.(type) {
	case *ir.BasicLit:
	case *ir.StructLit:
	default:
		panic(fmt.Sprintf("%T cannot be used as global initializer in LLVM", t))
	}

	init := cb.buildExpr(decl.Initializer)
	loc := cb.values[sym]
	loc.SetInitializer(init)
}

func (cb *codeBuilder) buildValDecl(decl *ir.ValDecl) {
	sym := decl.Sym
	loc := cb.b.CreateAlloca(cb.toLLVMType(sym.T), sym.Name)
	cb.values[decl.Sym] = loc

	init := cb.buildExpr(decl.Initializer)
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

	block := llvm.AddBasicBlock(fun, "entry")
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
	if !cb.signature {
		return
	}

	structt := cb.mod.Context().StructCreateNamed(decl.Name.Literal)
	cb.typeDecls[decl.Sym] = structt

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
	case *ir.WhileStmt:
		cb.buildWhileStmt(t)
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
	cond := cb.buildExpr(stmt.Cond)

	fun := cb.b.GetInsertBlock().Parent()
	iftrue := llvm.AddBasicBlock(fun, "if_true")
	join := llvm.AddBasicBlock(fun, "join")

	var iffalse llvm.BasicBlock
	if stmt.Else != nil {
		iffalse = llvm.AddBasicBlock(fun, "if_false")
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

func (cb *codeBuilder) buildWhileStmt(stmt *ir.WhileStmt) {
	fun := cb.b.GetInsertBlock().Parent()
	loop := llvm.AddBasicBlock(fun, "loop")
	cond := llvm.AddBasicBlock(fun, "loop_cond")
	exit := llvm.AddBasicBlock(fun, "loop_exit")
	var last llvm.BasicBlock

	cb.loopConditions = append(cb.loopConditions, cond)
	cb.loopExits = append(cb.loopExits, exit)

	cb.b.CreateBr(cond)

	cb.b.SetInsertPointAtEnd(cond)
	loopCond := cb.buildExpr(stmt.Cond)
	cb.b.CreateCondBr(loopCond, loop, exit)

	last = fun.LastBasicBlock()
	loop.MoveAfter(last)
	cb.b.SetInsertPointAtEnd(loop)
	cb.buildBlockStmt(stmt.Body)
	cb.b.CreateBr(cond)

	last = fun.LastBasicBlock()
	exit.MoveAfter(last)
	cb.b.SetInsertPointAtEnd(exit)

	cb.loopConditions = cb.loopConditions[:len(cb.loopConditions)-1]
	cb.loopExits = cb.loopExits[:len(cb.loopExits)-1]
}

func (cb *codeBuilder) buildReturnStmt(stmt *ir.ReturnStmt) {
	if stmt.X != nil {
		val := cb.buildExpr(stmt.X)
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
	var loc llvm.Value

	switch left := stmt.Left.(type) {
	case *ir.StarExpr:
		loc = cb.buildExpr(left.X)
	case *ir.DotExpr:
		loc = cb.buildDotExpr(left, false)
	case *ir.Ident:
		loc = cb.values[left.Sym]
	default:
		panic(fmt.Sprintf("Unhandled left side %T of assignment", left))
	}

	val := cb.buildExpr(stmt.Right)

	switch stmt.Assign.ID {
	case token.AddAssign, token.SubAssign, token.MulAssign, token.DivAssign, token.ModAssign:
		left := cb.b.CreateLoad(loc, "")
		val = cb.createArithmeticOp(stmt.Assign.ID, stmt.Left.Type(), left, val)
	}

	cb.b.CreateStore(val, loc)
}

func (cb *codeBuilder) buildExprStmt(stmt *ir.ExprStmt) {
	cb.buildExpr(stmt.X)
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
	case ir.TString:
		return llvm.PointerType(llvm.Int8Type(), 0)
	case ir.TStruct:
		structt := t.(*ir.StructType)
		if res, ok := cb.typeDecls[structt.Sym]; ok {
			return res
		}
		panic(fmt.Sprintf("Failed to find named type %s", t))
	case ir.TPointer:
		pointer := t.(*ir.PointerType)
		underlying := cb.toLLVMType(pointer.Underlying)
		return llvm.PointerType(underlying, 0)
	default:
		panic(fmt.Sprintf("Unhandled type %s", t.ID()))
	}
}

func (cb *codeBuilder) buildExpr(expr ir.Expr) llvm.Value {
	switch t := expr.(type) {
	case *ir.BinaryExpr:
		return cb.buildBinaryExpr(t)
	case *ir.UnaryExpr:
		return cb.buildUnaryExpr(t)
	case *ir.StarExpr:
		return cb.buildStarExpr(t)
	case *ir.BasicLit:
		return cb.buildBasicLit(t)
	case *ir.StructLit:
		return cb.buildStructLit(t)
	case *ir.Ident:
		return cb.buildIdent(t)
	case *ir.DotExpr:
		return cb.buildDotExpr(t, true)
	case *ir.Cast:
		return cb.buildCast(t)
	case *ir.FuncCall:
		return cb.buildFuncCall(t)
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
	case token.Star, token.MulAssign:
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
	left := cb.buildExpr(expr.Left)

	switch expr.Op.ID {
	case token.Add, token.Sub, token.Star, token.Div, token.Mod:
		right := cb.buildExpr(expr.Right)
		return cb.createArithmeticOp(expr.Op.ID, expr.T, left, right)
	case token.Eq, token.Neq, token.Gt, token.GtEq, token.Lt, token.LtEq:
		right := cb.buildExpr(expr.Right)
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
		right := cb.buildExpr(expr.Right)
		cb.b.CreateBr(join)

		cb.b.SetInsertPointAtEnd(join)
		phi := cb.b.CreatePHI(cb.toLLVMType(expr.T), "")
		phi.AddIncoming([]llvm.Value{case1, right}, []llvm.BasicBlock{leftBlock, rightBlock})
		return phi
	}

	panic(fmt.Sprintf("Unhandled binary op %s", expr.Op.ID))
}

func (cb *codeBuilder) buildUnaryExpr(expr *ir.UnaryExpr) llvm.Value {
	if expr.Op.ID == token.Sub {
		val := cb.buildExpr(expr.X)
		if ir.IsFloatingType(expr.T) {
			return cb.b.CreateFNeg(val, "")
		}
		return cb.b.CreateNeg(val, "")
	} else if expr.Op.ID == token.Lnot {
		val := cb.buildExpr(expr.X)
		return cb.b.CreateNot(val, "")
	} else if expr.Op.ID == token.And {
		if dot, ok := expr.X.(*ir.DotExpr); ok {
			return cb.buildDotExpr(dot, false)
		}
		ident := ir.ExprToIdent(expr.X)
		val, ok := cb.values[ident.Sym]
		common.Assert(ok, "nil value")
		return val
	}

	panic(fmt.Sprintf("Unhandled unary op %s", expr.Op.ID))
}

func (cb *codeBuilder) buildStarExpr(expr *ir.StarExpr) llvm.Value {
	val := cb.buildExpr(expr.X)
	return cb.b.CreateLoad(val, val.Name())
}

func (cb *codeBuilder) buildBasicLit(expr *ir.BasicLit) llvm.Value {
	if expr.Value.ID == token.String {
		if cb.inFunction {
			return cb.b.CreateGlobalStringPtr(expr.AsString(), "str")
		}

		typ := llvm.ArrayType(llvm.Int8Type(), len(expr.AsString())+1) // +1 is for null-terminator
		arr := llvm.AddGlobal(cb.mod, typ, "str")
		arr.SetLinkage(llvm.PrivateLinkage)
		arr.SetUnnamedAddr(true)
		arr.SetGlobalConstant(true)
		arr.SetInitializer(llvm.ConstString(expr.AsString(), true))

		return llvm.ConstBitCast(arr, llvm.PointerType(llvm.Int8Type(), 0))
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
		return llvm.ConstPointerNull(llvmType)
	}

	panic(fmt.Sprintf("Unhandled basic lit %s, type %s", expr.Value.ID, expr.T))
}

func (cb *codeBuilder) buildStructLit(expr *ir.StructLit) llvm.Value {
	structt := expr.T.(*ir.StructType)
	llvmType := cb.typeDecls[structt.Sym]
	structLit := llvm.Undef(llvmType)

	for _, field := range expr.Initializers {
		index := structt.FieldIndex(field.Key.Literal)
		init := cb.buildExpr(field.Value)
		structLit = cb.b.CreateInsertValue(structLit, init, index, "")
	}

	return structLit
}

func (cb *codeBuilder) buildIdent(expr *ir.Ident) llvm.Value {
	if val, ok := cb.values[expr.Sym]; ok {
		return cb.b.CreateLoad(val, expr.Sym.Name)
	}
	panic(fmt.Sprintf("%s not found", expr.Sym))
}

func (cb *codeBuilder) buildDotExpr(expr *ir.DotExpr, load bool) llvm.Value {
	var val llvm.Value

	switch t := expr.X.(type) {
	case *ir.Ident:
		val = cb.values[t.Sym]
	case *ir.DotExpr:
		val = cb.buildDotExpr(t, false)
	case *ir.StarExpr:
		val = cb.buildExpr(t.X)
	default:
		panic(fmt.Sprintf("Unhandled DotExpr %T", t))
	}

	structt := expr.X.Type().(*ir.StructType)
	index := structt.FieldIndex(expr.Name.Literal())

	gep := cb.b.CreateStructGEP(val, index, "")
	if load {
		return cb.b.CreateLoad(gep, "")
	}
	return gep
}

func (cb *codeBuilder) buildCast(expr *ir.Cast) llvm.Value {
	val := cb.buildExpr(expr.X)

	to := expr.ToTyp.Type()
	from := expr.X.Type()
	if from.IsEqual(to) {
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

func (cb *codeBuilder) buildFuncCall(expr *ir.FuncCall) llvm.Value {
	var args []llvm.Value
	for _, arg := range expr.Args {
		args = append(args, cb.buildExpr(arg))
	}
	sym := ir.ExprSymbol(expr.X)
	fun := cb.mod.NamedFunction(sym.Name)
	return cb.b.CreateCall(fun, args, "")
}
