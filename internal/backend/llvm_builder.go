package backend

import (
	"fmt"
	"os"
	"strings"

	"github.com/jhnl/dingo/internal/common"

	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
	"llvm.org/llvm/bindings/go/llvm"
)

type llvmCodeBuilder struct {
	b           llvm.Builder
	errors      *common.ErrorList
	target      *llvmTarget
	config      *common.BuildConfig
	objectfiles []string

	mod        llvm.Module
	values     map[*ir.Symbol]llvm.Value
	types      llvmTypeContext
	signature  bool
	inFunction bool

	level         int
	fun           llvm.Value
	retValue      llvm.Value
	retBlock      llvm.BasicBlock
	loops         []*loopContext
	defers        []*deferContext
	ifMergeBlocks []llvm.BasicBlock
}

type loopContext struct {
	condBlock llvm.BasicBlock
	exitBlock llvm.BasicBlock
	level     int
}

type deferContext struct {
	headerBlock        llvm.BasicBlock
	mainBlock          llvm.BasicBlock
	footerBlock        llvm.BasicBlock
	brBlock            llvm.BasicBlock
	exitBlock          llvm.BasicBlock
	potentialBrTargets []llvm.BasicBlock
	brTarget           llvm.Value
	stmts              []*deferStmt
	level              int
}

type deferStmt struct {
	stmt  ir.Stmt
	block llvm.BasicBlock
	br    bool
}

func (ctx *deferContext) topStmt() *deferStmt {
	nstmts := len(ctx.stmts)
	if nstmts > 0 {
		return ctx.stmts[nstmts-1]
	}
	return nil
}

func (ctx *deferContext) stmtBrBlock() llvm.BasicBlock {
	if stmt := ctx.topStmt(); stmt != nil {
		stmt.br = true
		return stmt.block
	}
	return ctx.footerBlock
}

// Field indexes for slice struct.
const ptrFieldIndex = 0
const lenFieldIndex = 1

// BuildLLVM code.
func BuildLLVM(set *ir.ModuleSet, target ir.Target, config *common.BuildConfig) error {
	llvmTarget := target.(*llvmTarget)

	cb := newBuilder(llvmTarget, config)
	cb.validateModuleSet(set)

	if cb.errors.IsError() {
		return cb.errors
	}

	defer cb.deleteObjects()

	for _, mod := range set.Modules {
		if !cb.buildModule(mod) {
			return cb.errors
		}
	}

	var args []string
	args = append(args, "-o")
	args = append(args, config.Exe)
	args = append(args, cb.objectfiles...)

	cmd := exec.Command("cc", args...)
	if linkOutput, linkErr := cmd.CombinedOutput(); linkErr != nil {
		lines := strings.Split(string(linkOutput), "\n")
		cb.errors.AddContext(token.NoPosition, lines, "link: %s", linkErr)
	}

	return cb.errors
}

func newBuilder(target *llvmTarget, config *common.BuildConfig) *llvmCodeBuilder {
	cb := &llvmCodeBuilder{b: llvm.NewBuilder()}
	cb.b = llvm.NewBuilder()
	cb.errors = &common.ErrorList{}
	cb.target = target
	cb.config = config
	return cb
}

func (cb *llvmCodeBuilder) validateModuleSet(set *ir.ModuleSet) {
	for _, mod := range set.Modules {
		if len(mod.FQN) == 0 {
			cb.errors.Add(mod.Path, "no module name")
		}
	}

	mod := set.FindModule("main")
	if mod != nil {
		cb.validateMainFunc(mod)
	} else {
		cb.errors.AddGeneric1(fmt.Errorf("no main module"))
	}
}

func (cb *llvmCodeBuilder) validateMainFunc(mod *ir.Module) {
	mainFunc := mod.FindFuncSymbol("main")
	if mainFunc != nil {
		tmain := mainFunc.T.(*ir.FuncType)
		pos := mainFunc.DefPos

		if !tmain.C {
			cb.errors.Add(pos, "'main' must be declared as a C function")
		}

		if len(tmain.Params) > 0 {
			param0 := tmain.Params[0]
			if param0.T.ID() != ir.TInt32 {
				cb.errors.Add(pos, "first parameter of 'main' must have type %s (has type %s)", ir.TInt32, param0.T)
			}
		}

		if len(tmain.Params) > 1 {
			param1 := tmain.Params[1]
			texpected := ir.NewPointerType(ir.NewPointerType(ir.NewBasicType(ir.TInt8), true), true)
			if !param1.T.Equals(texpected) {
				cb.errors.Add(pos, "second parameter of 'main' must have type %s (has type %s)", texpected, param1.T)
			}
		}

		if len(tmain.Params) > 2 {
			cb.errors.Add(pos, "'main' can have no more than 2 parameters (got %d)", len(tmain.Params))
		}

		if tmain.Return.ID() != ir.TInt32 {
			cb.errors.Add(pos, "'main' must have return type %s (has type %s)", ir.TInt32, tmain.Return)
		}
	} else {
		cb.errors.Add(mod.Path, "no main function")
	}
}

func (cb *llvmCodeBuilder) deleteObjects() {
	for _, object := range cb.objectfiles {
		os.Remove(object)
	}
}

func (cb *llvmCodeBuilder) buildModule(mod *ir.Module) bool {
	cb.mod = llvm.NewModule(mod.FQN)
	cb.values = make(map[*ir.Symbol]llvm.Value)
	cb.types = make(llvmTypeContext)
	cb.inFunction = false

	cb.signature = true
	for _, decl := range mod.Decls {
		cb.buildDecl(decl)
	}

	cb.signature = false
	for _, decl := range mod.Decls {
		sym := decl.Symbol()
		if sym.IsType() || sym.ModFQN() == mod.FQN {
			cb.buildDecl(decl)
		}
	}

	return cb.finalizeModule(mod)
}

func (cb *llvmCodeBuilder) finalizeModule(mod *ir.Module) bool {
	if cb.errors.IsError() {
		return false
	}

	if cb.config.LLVMIR {
		cb.mod.Dump()
	}

	if err := llvm.VerifyModule(cb.mod, llvm.ReturnStatusAction); err != nil {
		panic(err)
	}

	_, filename := filepath.Split(mod.Path.Filename)
	filename = strings.TrimSuffix(filename, filepath.Ext(mod.Path.Filename))

	objectfile := filename + ".o"
	outputmode := llvm.ObjectFile

	if code, err := cb.target.machine.EmitToMemoryBuffer(cb.mod, outputmode); err == nil {
		file, err := ioutil.TempFile("", objectfile)
		if err != nil {
			panic(err)
		}

		cb.objectfiles = append(cb.objectfiles, file.Name())

		if _, err := file.Write(code.Bytes()); err != nil {
			panic(err)
		}
	} else {
		panic(err)
	}

	return true
}

func (cb *llvmCodeBuilder) llvmType(t ir.Type) llvm.Type {
	return llvmType(t, &cb.types)
}

func (cb *llvmCodeBuilder) buildDecl(decl ir.Decl) {
	switch t := decl.(type) {
	case *ir.TypeTopDecl:
	case *ir.TypeDecl:
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

func (cb *llvmCodeBuilder) buildValTopDecl(decl *ir.ValTopDecl) {
	sym := decl.Sym
	if cb.signature {
		loc := llvm.AddGlobal(cb.mod, cb.llvmType(sym.T), mangle(sym))

		switch decl.Visibility() {
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

func (cb *llvmCodeBuilder) buildValDecl(decl *ir.ValDecl) {
	sym := decl.Sym
	loc := cb.b.CreateAlloca(cb.llvmType(sym.T), sym.Name)
	cb.values[decl.Sym] = loc

	init := cb.buildExprVal(decl.Initializer)
	cb.b.CreateStore(init, loc)
}

func (cb *llvmCodeBuilder) buildFuncDecl(decl *ir.FuncDecl) {
	name := mangle(decl.Sym)
	fun := cb.mod.NamedFunction(name)

	tfun := ir.ToBaseType(decl.Sym.T).(*ir.FuncType)

	if cb.signature {
		if !fun.IsNil() {
			return
		}

		var paramTypes []llvm.Type
		for _, p := range tfun.Params {
			paramTypes = append(paramTypes, cb.llvmType(p.T))
		}
		retType := cb.llvmType(tfun.Return)

		funType := llvm.FunctionType(retType, paramTypes, false)
		fun = llvm.AddFunction(cb.mod, name, funType)

		switch decl.Visibility() {
		case token.Public:
			fun.SetLinkage(llvm.ExternalLinkage)
		case token.Private:
			fun.SetLinkage(llvm.InternalLinkage)
		}

		return
	} else if decl.SignatureOnly() {
		return
	}

	entryBlock := llvm.AddBasicBlock(fun, ".entry")
	cb.b.SetInsertPointAtEnd(entryBlock)

	if tfun.Return.ID() != ir.TVoid {
		cb.retValue = cb.b.CreateAlloca(cb.llvmType(tfun.Return), ".retval")
	}

	for i, p := range fun.Params() {
		sym := decl.Params[i].Sym
		loc := cb.b.CreateAlloca(p.Type(), sym.Name)
		cb.b.CreateStore(p, loc)
		cb.values[sym] = loc
	}

	cb.level = 0
	cb.fun = fun
	cb.retBlock = llvm.AddBasicBlock(fun, ".ret")
	retBlock := cb.retBlock

	cb.inFunction = true
	terminated := cb.buildBlockStmt(decl.Body)
	cb.inFunction = false

	if !terminated {
		cb.b.CreateBr(retBlock)
	}

	retBlock.MoveAfter(cb.b.GetInsertBlock())
	cb.b.SetInsertPointAtEnd(retBlock)

	if tfun.Return.ID() == ir.TVoid {
		cb.b.CreateRetVoid()
	} else {
		res := cb.b.CreateLoad(cb.retValue, "")
		cb.b.CreateRet(res)

		if !terminated {
			cb.errors.Add(decl.Body.EndPos(), "missing return")
		}
	}
}

func (cb *llvmCodeBuilder) buildStructDecl(decl *ir.StructDecl) {
	if cb.signature {
		structt := cb.mod.Context().StructCreateNamed(mangle(decl.Sym))
		cb.types[decl.Sym] = structt
		return
	}

	if decl.Opaque {
		return
	}

	structt := cb.types[decl.Sym]

	var types []llvm.Type
	for _, field := range decl.Fields {
		types = append(types, cb.llvmType(field.Sym.T))
	}

	structt.StructSetBody(types, false)
}

func (cb *llvmCodeBuilder) buildStmt(stmt ir.Stmt) bool {
	terminate := false
	switch stmt2 := stmt.(type) {
	case *ir.BlockStmt:
		terminate = cb.buildBlockStmt(stmt2)
	case *ir.DeclStmt:
		cb.buildDecl(stmt2.D)
	case *ir.IfStmt:
		terminate = cb.buildIfStmt(stmt2)
	case *ir.ForStmt:
		cb.buildForStmt(stmt2)
	case *ir.AssignStmt:
		cb.buildAssignStmt(stmt2)
	case *ir.ExprStmt:
		cb.buildExprVal(stmt2.X)
	default:
		panic(fmt.Sprintf("Unhandled stmt %T at %s", stmt2, stmt2.Pos()))
	}
	return terminate
}

func (cb *llvmCodeBuilder) buildDeferBrTargets(branchTok token.Token) {
	if len(cb.defers) == 0 {
		return
	}

	level := cb.branchTargetLevel(branchTok)
	index := len(cb.defers) - 1

	if cb.defers[index].level > level {
		for ; index > 0 && cb.defers[index-1].level > level; index-- {
			block := cb.defers[index-1].stmtBrBlock()
			addr := llvm.BlockAddress(cb.fun, block)
			cb.b.CreateStore(addr, cb.defers[index].brTarget)
			cb.defers[index].potentialBrTargets = append(cb.defers[index].potentialBrTargets, block)
		}

		target := cb.branchTarget(branchTok)
		addr := llvm.BlockAddress(cb.fun, target)

		cb.b.CreateStore(addr, cb.defers[index].brTarget)
		cb.defers[index].potentialBrTargets = append(cb.defers[index].potentialBrTargets, target)
	}
}

func (cb *llvmCodeBuilder) branchTargetLevel(branchTok token.Token) int {
	switch branchTok {
	case token.Return:
		return 0
	case token.Continue, token.Break:
		return cb.loops[len(cb.loops)-1].level
	default:
		panic(fmt.Sprintf("Unhandled branch token %s", branchTok))
	}
}

func (cb *llvmCodeBuilder) branchTarget(branchTok token.Token) llvm.BasicBlock {
	switch branchTok {
	case token.Return:
		return cb.retBlock
	case token.Continue:
		return cb.loops[len(cb.loops)-1].condBlock
	case token.Break:
		return cb.loops[len(cb.loops)-1].exitBlock
	default:
		panic(fmt.Sprintf("Unhandled branch token %s", branchTok))
	}
}

func (cb *llvmCodeBuilder) deferOrBranchTarget(branchTok token.Token) llvm.BasicBlock {
	ndefers := len(cb.defers)
	if ndefers > 0 {
		level := cb.branchTargetLevel(branchTok)
		if cb.defers[ndefers-1].level > level {
			return cb.defers[ndefers-1].stmtBrBlock()
		}
	}
	return cb.branchTarget(branchTok)
}

func (cb *llvmCodeBuilder) buildBlockStmt(stmt *ir.BlockStmt) bool {
	initialBlock := cb.b.GetInsertBlock()

	if stmt.Scope.Defer {
		deferCtx := &deferContext{}
		deferCtx.level = cb.level + 1
		deferCtx.headerBlock = llvm.AddBasicBlock(cb.fun, formatLabel("defer.header", stmt.Pos()))
		deferCtx.mainBlock = llvm.AddBasicBlock(cb.fun, formatLabel("defer.block", stmt.Pos()))
		deferCtx.footerBlock = llvm.AddBasicBlock(cb.fun, formatLabel("defer.footer", stmt.Pos()))
		deferCtx.brBlock = llvm.AddBasicBlock(cb.fun, formatLabel("defer.br", stmt.Pos()))
		deferCtx.exitBlock = llvm.AddBasicBlock(cb.fun, formatLabel("defer.exit", stmt.Pos()))

		deferCtx.headerBlock.MoveAfter(cb.b.GetInsertBlock())
		cb.b.SetInsertPointAtEnd(deferCtx.headerBlock)
		deferCtx.brTarget = cb.b.CreateAlloca(llvm.PointerType(llvm.Int8Type(), 0), formatLabel("defer.braddr", stmt.Pos()))
		cb.b.CreateStore(llvm.ConstNull(llvm.PointerType(llvm.Int8Type(), 0)), deferCtx.brTarget)

		cb.b.CreateBr(deferCtx.mainBlock)
		cb.b.SetInsertPointAtEnd(deferCtx.mainBlock)

		cb.defers = append(cb.defers, deferCtx)
	}

	branchTok := token.Invalid
	terminate := false
	cb.level++

	for index, item := range stmt.Stmts {
		switch stmt2 := item.(type) {
		case *ir.DeferStmt:
			deferCtx := cb.defers[len(cb.defers)-1]
			block := llvm.AddBasicBlock(cb.fun, formatLabel("defer.stmt", stmt2.Pos()))
			deferCtx.stmts = append(deferCtx.stmts, &deferStmt{stmt: stmt2.S, block: block})
		case *ir.ReturnStmt:
			if stmt2.X != nil {
				val := cb.buildExprVal(stmt2.X)
				cb.b.CreateStore(val, cb.retValue)
			}
			branchTok = token.Return
			terminate = true
		case *ir.BranchStmt:
			branchTok = stmt2.Tok
			terminate = true
		default:
			terminate = cb.buildStmt(stmt2)
		}
		if terminate {
			if (index + 1) < len(stmt.Stmts) {
				cb.errors.AddWarning(stmt.Stmts[index+1].Pos(), "unreachable code")
			}
			break
		}
	}

	cb.level--

	if branchTok != token.Invalid {
		cb.buildDeferBrTargets(branchTok)
	}

	if stmt.Scope.Defer {
		deferCtx := cb.defers[len(cb.defers)-1]
		cb.defers = cb.defers[:len(cb.defers)-1]

		if branchTok != token.Invalid || !terminate {
			cb.b.CreateBr(deferCtx.stmtBrBlock())
		}

		if len(deferCtx.stmts) > 0 {
			topStmt := deferCtx.topStmt()
			topStmt.block.MoveAfter(cb.b.GetInsertBlock())
			cb.b.SetInsertPointAtEnd(topStmt.block)
			cb.buildStmt(topStmt.stmt)

			for i := len(deferCtx.stmts) - 2; i >= 0; i-- {
				stmt := deferCtx.stmts[i]
				if stmt.br {
					cb.b.CreateBr(stmt.block)
					stmt.block.MoveAfter(cb.b.GetInsertBlock())
					cb.b.SetInsertPointAtEnd(stmt.block)
				} else {
					stmt.block.EraseFromParent()
				}
				cb.buildStmt(stmt.stmt)
			}

			cb.b.CreateBr(deferCtx.footerBlock)
		}

		deferCtx.footerBlock.MoveAfter(cb.b.GetInsertBlock())
		cb.b.SetInsertPointAtEnd(initialBlock)

		if len(deferCtx.potentialBrTargets) == 0 {
			cb.b.CreateBr(deferCtx.mainBlock)

			deferCtx.mainBlock.MoveAfter(initialBlock)
			deferCtx.headerBlock.EraseFromParent()
			deferCtx.brBlock.EraseFromParent()
			deferCtx.exitBlock.EraseFromParent()

			cb.b.SetInsertPointAtEnd(deferCtx.footerBlock)
		} else {
			cb.b.CreateBr(deferCtx.headerBlock)
			deferCtx.mainBlock.MoveAfter(deferCtx.headerBlock)

			cb.b.SetInsertPointAtEnd(deferCtx.footerBlock)

			target := cb.b.CreateLoad(deferCtx.brTarget, "")
			if terminate {
				indirect := cb.b.CreateIndirectBr(target, len(deferCtx.potentialBrTargets))
				for _, t := range deferCtx.potentialBrTargets {
					indirect.AddDest(t)
				}
				deferCtx.brBlock.EraseFromParent()
				deferCtx.exitBlock.EraseFromParent()
			} else {
				cmp := cb.b.CreateICmp(llvm.IntNE, target, llvm.ConstNull(llvm.PointerType(llvm.Int8Type(), 0)), "")
				cb.b.CreateCondBr(cmp, deferCtx.brBlock, deferCtx.exitBlock)

				deferCtx.brBlock.MoveAfter(cb.b.GetInsertBlock())
				cb.b.SetInsertPointAtEnd(deferCtx.brBlock)

				indirect := cb.b.CreateIndirectBr(target, len(deferCtx.potentialBrTargets))
				for _, t := range deferCtx.potentialBrTargets {
					indirect.AddDest(t)
				}

				deferCtx.exitBlock.MoveAfter(cb.b.GetInsertBlock())
				cb.b.SetInsertPointAtEnd(deferCtx.exitBlock)
			}
		}
	} else if branchTok != token.Invalid {
		cb.b.CreateBr(cb.deferOrBranchTarget(branchTok))
	}

	return terminate
}

func formatTokLabel(tok token.Token, name string, pos token.Position) string {
	return formatLabel(fmt.Sprintf("%s.%s", tok, name), pos)
}

func formatLabel(label string, pos token.Position) string {
	return fmt.Sprintf(".%s_%d", label, pos.Line)
}

func (cb *llvmCodeBuilder) buildIfStmt(stmt *ir.IfStmt) bool {
	var mergeBlock llvm.BasicBlock
	var elseBlock llvm.BasicBlock
	var ifBlock llvm.BasicBlock
	ifBranch := false
	elseBranch := false

	if stmt.Tok.Is(token.If) {
		mergeBlock = llvm.AddBasicBlock(cb.fun, formatTokLabel(stmt.Tok, "merge", stmt.EndPos()))
		cb.ifMergeBlocks = append(cb.ifMergeBlocks, mergeBlock)
	} else {
		mergeBlock = cb.ifMergeBlocks[len(cb.ifMergeBlocks)-1]
	}

	cond := cb.buildExprVal(stmt.Cond)

	ifBlock = llvm.AddBasicBlock(cb.fun, formatTokLabel(stmt.Tok, "true", stmt.Body.Pos()))
	if stmt.Else != nil {
		elseBlock = llvm.AddBasicBlock(cb.fun, formatTokLabel(stmt.Tok, "false", stmt.Else.Pos()))
		cb.b.CreateCondBr(cond, ifBlock, elseBlock)
	} else {
		cb.b.CreateCondBr(cond, ifBlock, mergeBlock)
	}

	ifBlock.MoveAfter(cb.b.GetInsertBlock())
	cb.b.SetInsertPointAtEnd(ifBlock)
	ifBranch = cb.buildBlockStmt(stmt.Body)
	if !ifBranch {
		cb.b.CreateBr(mergeBlock)
	}

	if stmt.Else != nil {
		elseBlock.MoveAfter(cb.b.GetInsertBlock())
		cb.b.SetInsertPointAtEnd(elseBlock)
		elseBranch = cb.buildStmt(stmt.Else)
		if !elseBranch {
			// Check if it's an else statement
			if _, ok := stmt.Else.(*ir.BlockStmt); ok {
				cb.b.CreateBr(mergeBlock)
			}
		}
	}

	mergeBlock.MoveAfter(cb.b.GetInsertBlock())
	terminate := true

	if !ifBranch || !elseBranch {
		cb.b.SetInsertPointAtEnd(mergeBlock)
		terminate = false
	} else {
		if stmt.Tok.Is(token.If) {
			mergeBlock.EraseFromParent()
		}

		if stmt.Else != nil {
			cb.b.SetInsertPointAtEnd(elseBlock)
		} else {
			cb.b.SetInsertPointAtEnd(ifBlock)
		}
	}

	if stmt.Tok.Is(token.If) {
		cb.ifMergeBlocks = cb.ifMergeBlocks[:len(cb.ifMergeBlocks)-1]
	}

	return terminate
}

func (cb *llvmCodeBuilder) buildForStmt(stmt *ir.ForStmt) {
	if stmt.Init != nil {
		cb.buildValDecl(stmt.Init)
	}

	loopBlock := llvm.AddBasicBlock(cb.fun, formatTokLabel(stmt.Tok, "body", stmt.Pos()))
	exitBlock := llvm.AddBasicBlock(cb.fun, formatTokLabel(stmt.Tok, "exit", stmt.EndPos()))

	var incBlock llvm.BasicBlock
	if stmt.Inc != nil {
		incBlock = llvm.AddBasicBlock(cb.fun, formatTokLabel(stmt.Tok, "inc", stmt.Pos()))
	}

	var condBlock llvm.BasicBlock
	if stmt.Cond != nil {
		condBlock = llvm.AddBasicBlock(cb.fun, formatTokLabel(stmt.Tok, "cond", stmt.Pos()))
	}

	loopCtx := &loopContext{}
	loopCtx.level = cb.level

	if stmt.Inc != nil {
		loopCtx.condBlock = incBlock
	} else if stmt.Cond != nil {
		loopCtx.condBlock = condBlock
	} else {
		loopCtx.condBlock = loopBlock
	}

	loopCtx.exitBlock = exitBlock
	cb.loops = append(cb.loops, loopCtx)

	if stmt.Cond != nil {
		cb.b.CreateBr(condBlock)
		cb.b.SetInsertPointAtEnd(condBlock)
		cond := cb.buildExprVal(stmt.Cond)
		cb.b.CreateCondBr(cond, loopBlock, exitBlock)
	} else {
		cb.b.CreateBr(loopBlock)
	}

	last := cb.b.GetInsertBlock()
	loopBlock.MoveAfter(last)
	cb.b.SetInsertPointAtEnd(loopBlock)
	cb.buildBlockStmt(stmt.Body)

	if stmt.Inc != nil {
		cb.b.CreateBr(incBlock)
		last = cb.b.GetInsertBlock()
		incBlock.MoveAfter(last)
		cb.b.SetInsertPointAtEnd(incBlock)
		cb.buildStmt(stmt.Inc)
	}

	if stmt.Cond != nil {
		cb.b.CreateBr(condBlock)
	} else {
		cb.b.CreateBr(loopBlock)
	}

	last = cb.b.GetInsertBlock()
	exitBlock.MoveAfter(last)
	cb.b.SetInsertPointAtEnd(exitBlock)

	cb.loops = cb.loops[:len(cb.loops)-1]
}

func (cb *llvmCodeBuilder) buildAssignStmt(stmt *ir.AssignStmt) {
	loc := cb.buildExprPtr(stmt.Left)
	val := cb.buildExprVal(stmt.Right)

	switch stmt.Assign {
	case token.AddAssign, token.SubAssign, token.MulAssign, token.DivAssign, token.ModAssign:
		left := cb.b.CreateLoad(loc, "")
		val = cb.createMathOp(stmt.Assign, stmt.Left.Type(), left, val)
	}

	cb.b.CreateStore(val, loc)
}

func (cb *llvmCodeBuilder) llvmEnumAttribute(name string, val uint64) llvm.Attribute {
	kind := llvm.AttributeKindID(name)
	ctx := llvm.GlobalContext()
	attr := ctx.CreateEnumAttribute(kind, val)
	return attr
}

func (cb *llvmCodeBuilder) buildExprVal(expr ir.Expr) llvm.Value {
	val := cb.buildExpr(expr, true)
	return val
}

func (cb *llvmCodeBuilder) buildExprPtr(expr ir.Expr) llvm.Value {
	val := cb.buildExpr(expr, false)
	return val
}

func (cb *llvmCodeBuilder) buildExpr(expr ir.Expr, load bool) llvm.Value {
	switch expr2 := expr.(type) {
	case *ir.Ident:
		return cb.buildIdent(expr2, load)
	case *ir.BasicLit:
		return cb.buildBasicLit(expr2)
	case *ir.StructLit:
		return cb.buildStructLit(expr2)
	case *ir.ArrayLit:
		return cb.buildArrayLit(expr2)
	case *ir.BinaryExpr:
		return cb.buildBinaryExpr(expr2)
	case *ir.UnaryExpr:
		return cb.buildUnaryExpr(expr2, load)
	case *ir.DotExpr:
		return cb.buildDotExpr(expr2, load)
	case *ir.IndexExpr:
		return cb.buildIndexExpr(expr2, load)
	case *ir.SliceExpr:
		return cb.buildSliceExpr(expr2)
	case *ir.FuncCall:
		return cb.buildFuncCall(expr2)
	case *ir.CastExpr:
		return cb.buildCastExpr(expr2)
	case *ir.LenExpr:
		return cb.buildLenExpr(expr2)
	case *ir.ConstExpr:
		return cb.buildExpr(expr2.X, load)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", expr2))
	}
}

func (cb *llvmCodeBuilder) buildIdent(expr *ir.Ident, load bool) llvm.Value {
	if expr.Sym.ID == ir.FuncSymbol {
		return cb.mod.NamedFunction(mangle(expr.Sym))
	}

	if val, ok := cb.values[expr.Sym]; ok {
		if load {
			return cb.b.CreateLoad(val, expr.Sym.Name)
		}
		return val
	}

	panic(fmt.Sprintf("%s not found", expr.Sym))
}

func (cb *llvmCodeBuilder) buildBasicLit(expr *ir.BasicLit) llvm.Value {
	if expr.Tok == token.String {
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

	llvmType := cb.llvmType(expr.T)
	if ir.IsIntegerType(expr.T) {
		val := llvm.ConstInt(llvmType, expr.AsU64(), false)
		if expr.NegatigeInteger() {
			return cb.b.CreateNeg(val, "")
		}
		return val
	} else if ir.IsFloatType(expr.T) {
		return llvm.ConstFloat(llvmType, expr.AsF64())
	} else if expr.Tok == token.True {
		return llvm.ConstInt(llvmType, 1, false)
	} else if expr.Tok == token.False {
		return llvm.ConstInt(llvmType, 0, false)
	} else if expr.Tok == token.Null {
		switch t := ir.ToBaseType(expr.T).(type) {
		case *ir.SliceType:
			tptr := llvm.PointerType(cb.llvmType(t.Elem), 0)
			ptr := llvm.ConstPointerNull(tptr)
			size := cb.createSliceSize(0)
			return cb.createSliceStruct(ptr, size, t)
		case *ir.PointerType, *ir.FuncType:
			return llvm.ConstPointerNull(llvmType)
		}
	}

	panic(fmt.Sprintf("Unhandled basic lit %s, type %s", expr.Tok, expr.T))
}

func (cb *llvmCodeBuilder) createSliceStruct(ptr llvm.Value, size llvm.Value, t ir.Type) llvm.Value {
	llvmType := cb.llvmType(t)
	sliceStruct := llvm.Undef(llvmType)
	sliceStruct = cb.b.CreateInsertValue(sliceStruct, ptr, ptrFieldIndex, "")
	sliceStruct = cb.b.CreateInsertValue(sliceStruct, size, lenFieldIndex, "")
	return sliceStruct
}

func (cb *llvmCodeBuilder) createSliceSize(size int) llvm.Value {
	return llvm.ConstInt(llvm.IntType(32), uint64(size), false)
}

func (cb *llvmCodeBuilder) buildStructLit(expr *ir.StructLit) llvm.Value {
	tstruct := ir.ToBaseType(expr.T).(*ir.StructType)
	llvmType := cb.types[tstruct.Sym]
	structLit := llvm.Undef(llvmType)

	for argIndex, arg := range expr.Args {
		init := cb.buildExprVal(arg.Value)
		structLit = cb.b.CreateInsertValue(structLit, init, argIndex, "")
	}

	return structLit
}

func (cb *llvmCodeBuilder) buildArrayLit(expr *ir.ArrayLit) llvm.Value {
	llvmType := cb.llvmType(expr.T)
	arrayLit := llvm.Undef(llvmType)

	for index, elem := range expr.Initializers {
		init := cb.buildExprVal(elem)
		arrayLit = cb.b.CreateInsertValue(arrayLit, init, index, "")
	}

	return arrayLit
}

func (cb *llvmCodeBuilder) createMathOp(op token.Token, t ir.Type, left llvm.Value, right llvm.Value) llvm.Value {
	switch op {
	case token.Add, token.AddAssign:
		if ir.IsFloatType(t) {
			return cb.b.CreateFAdd(left, right, "")
		}
		return cb.b.CreateAdd(left, right, "")
	case token.Sub, token.SubAssign:
		if ir.IsFloatType(t) {
			return cb.b.CreateFSub(left, right, "")
		}
		return cb.b.CreateSub(left, right, "")
	case token.Mul, token.MulAssign:
		if ir.IsFloatType(t) {
			return cb.b.CreateFMul(left, right, "")
		}
		return cb.b.CreateMul(left, right, "")
	case token.Div:
		if ir.IsFloatType(t) {
			return cb.b.CreateFDiv(left, right, "")
		} else if ir.IsUnsignedType(t) {
			return cb.b.CreateUDiv(left, right, "")
		}
		return cb.b.CreateSDiv(left, right, "")
	case token.Mod:
		if ir.IsFloatType(t) {
			return cb.b.CreateFRem(left, right, "")
		} else if ir.IsUnsignedType(t) {
			return cb.b.CreateURem(left, right, "")
		}
		return cb.b.CreateSRem(left, right, "")
	}
	panic(fmt.Sprintf("Unhandled arithmetic op %s", op))
}

func floatPredicate(op token.Token) llvm.FloatPredicate {
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

func intPredicate(op token.Token, t ir.Type) llvm.IntPredicate {
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
			return llvm.IntSLE
		}
		return llvm.IntULE
	}
	panic(fmt.Sprintf("Unhandled int predicate %s", op))
}

func (cb *llvmCodeBuilder) buildBinaryExpr(expr *ir.BinaryExpr) llvm.Value {
	left := cb.buildExprVal(expr.Left)

	switch expr.Op {
	case token.Add, token.Sub, token.Mul, token.Div, token.Mod:
		right := cb.buildExprVal(expr.Right)
		return cb.createMathOp(expr.Op, expr.T, left, right)
	case token.Eq, token.Neq, token.Gt, token.GtEq, token.Lt, token.LtEq:
		right := cb.buildExprVal(expr.Right)
		if ir.IsFloatType(expr.T) {
			return cb.b.CreateFCmp(floatPredicate(expr.Op), left, right, "")
		}
		return cb.b.CreateICmp(intPredicate(expr.Op, expr.Left.Type()), left, right, "")
	case token.Land, token.Lor:
		fun := cb.b.GetInsertBlock().Parent()
		leftBlock := cb.b.GetInsertBlock()
		rightBlock := llvm.AddBasicBlock(fun, "")
		join := llvm.AddBasicBlock(fun, "")

		var case1 llvm.Value
		if expr.Op == token.Land {
			cb.b.CreateCondBr(left, rightBlock, join)
			case1 = llvm.ConstInt(llvm.Int1Type(), 0, false)
		} else { // Lor
			cb.b.CreateCondBr(left, join, rightBlock)
			case1 = llvm.ConstInt(llvm.Int1Type(), 1, false)
		}

		last := fun.LastBasicBlock()
		rightBlock.MoveAfter(last)
		cb.b.SetInsertPointAtEnd(rightBlock)

		right := cb.buildExprVal(expr.Right)
		rightBlock = cb.b.GetInsertBlock()
		cb.b.CreateBr(join)

		last = fun.LastBasicBlock()
		join.MoveAfter(last)
		cb.b.SetInsertPointAtEnd(join)

		phi := cb.b.CreatePHI(cb.llvmType(expr.T), "")
		phi.AddIncoming([]llvm.Value{case1, right}, []llvm.BasicBlock{leftBlock, rightBlock})
		return phi
	}

	panic(fmt.Sprintf("Unhandled binary op %s", expr.Op))
}

func (cb *llvmCodeBuilder) buildUnaryExpr(expr *ir.UnaryExpr, load bool) llvm.Value {
	switch expr.Op {
	case token.Sub:
		val := cb.buildExprVal(expr.X)
		if ir.IsFloatType(expr.T) {
			return cb.b.CreateFNeg(val, "")
		}
		return cb.b.CreateNeg(val, "")
	case token.Lnot:
		val := cb.buildExprVal(expr.X)
		return cb.b.CreateNot(val, "")
	case token.Deref:
		val := cb.buildExprVal(expr.X)
		if load {
			return cb.b.CreateLoad(val, val.Name())
		}
		return val
	case token.Addr:
		val := cb.buildExprPtr(expr.X)
		return val
	default:
		panic(fmt.Sprintf("Unhandled unary op %s", expr.Op))
	}
}

func (cb *llvmCodeBuilder) createTempStorage(val llvm.Value) llvm.Value {
	// TODO: See if there's a better way to handle intermediary results
	loc := cb.b.CreateAlloca(val.Type(), ".tmp")
	cb.b.CreateStore(val, loc)
	return loc
}

func (cb *llvmCodeBuilder) buildDotExpr(expr *ir.DotExpr, load bool) llvm.Value {
	switch t := ir.ToBaseType(expr.X.Type()).(type) {
	case *ir.ModuleType:
		return cb.buildIdent(expr.Name, load)
	case *ir.StructType:
		val := cb.buildExprPtr(expr.X)
		if val.Type().TypeKind() != llvm.PointerTypeKind {
			val = cb.createTempStorage(val)
		}

		index := t.FieldIndex(expr.Name.Literal)
		gep := cb.b.CreateStructGEP(val, index, "")
		if load {
			return cb.b.CreateLoad(gep, "")
		}
		return gep
	default:
		panic(fmt.Sprintf("%T not handled", t))
	}
}

func (cb *llvmCodeBuilder) buildIndexExpr(expr *ir.IndexExpr, load bool) llvm.Value {
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

func (cb *llvmCodeBuilder) buildSliceExpr(expr *ir.SliceExpr) llvm.Value {
	val := cb.buildExprPtr(expr.X)
	start := cb.buildExprVal(expr.Start)
	end := cb.buildExprVal(expr.End)

	var gep llvm.Value
	var tptr llvm.Type

	switch t := ir.ToBaseType(expr.X.Type()).(type) {
	case *ir.ArrayType:
		gep = cb.b.CreateInBoundsGEP(val, []llvm.Value{llvm.ConstInt(llvm.Int64Type(), 0, false), start}, "")
		tptr = llvm.PointerType(cb.llvmType(t.Elem), 0)
	case *ir.SliceType:
		slicePtr := cb.b.CreateStructGEP(val, ptrFieldIndex, "")
		slicePtr = cb.b.CreateLoad(slicePtr, "")
		gep = cb.b.CreateInBoundsGEP(slicePtr, []llvm.Value{start}, "")
		tptr = llvm.PointerType(cb.llvmType(t.Elem), 0)
	case *ir.PointerType:
		tmp := cb.b.CreateLoad(val, "")
		gep = cb.b.CreateInBoundsGEP(tmp, []llvm.Value{start}, "")
		tptr = llvm.PointerType(cb.llvmType(t.Elem), 0)
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

func (cb *llvmCodeBuilder) buildFuncCall(expr *ir.FuncCall) llvm.Value {
	fun := cb.buildExprVal(expr.X)
	var args []llvm.Value
	for _, arg := range expr.Args {
		args = append(args, cb.buildExprVal(arg.Value))
	}
	return cb.b.CreateCall(fun, args, "")
}

func (cb *llvmCodeBuilder) buildCastExpr(expr *ir.CastExpr) llvm.Value {
	val := cb.buildExprVal(expr.X)

	to := expr.Type()
	from := expr.X.Type()
	if from.Equals(to) {
		return val
	}

	fromLLVM := cb.llvmType(from)
	toLLVM := cb.llvmType(to)

	cmpBitSize := cb.target.compareBitSize(toLLVM, fromLLVM)

	var res llvm.Value
	unhandled := false

	switch {
	case ir.IsFloatType(from):
		switch {
		case ir.IsFloatType(to):
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
		case ir.IsFloatType(to):
			if ir.IsUnsignedType(from) {
				res = cb.b.CreateUIToFP(val, toLLVM, "")
			} else {
				res = cb.b.CreateSIToFP(val, toLLVM, "")
			}
		case ir.IsIntegerType(to):
			if cmpBitSize > 0 {
				if ir.IsSignedType(to) {
					res = cb.b.CreateSExt(val, toLLVM, "")
				} else {
					res = cb.b.CreateZExt(val, toLLVM, "")
				}
			} else {
				res = cb.b.CreateTrunc(val, toLLVM, "")
			}
		default:
			unhandled = true
		}
	default:
		if from.ID() == ir.TPointer && to.ID() == ir.TPointer {
			res = cb.b.CreateBitCast(val, cb.llvmType(to), "")
		} else if from.ID() == ir.TSlice && to.ID() == ir.TSlice {
			slice1 := ir.ToBaseType(from).(*ir.SliceType)
			slice2 := ir.ToBaseType(to).(*ir.SliceType)
			if slice1.Elem.Equals(slice2.Elem) {
				res = val
			} else {
				unhandled = true
			}
		} else {
			unhandled = true
		}
	}

	if unhandled {
		panic(fmt.Sprintf("Failed to cast from %s to %s", from, to))
	}

	return res
}

func (cb *llvmCodeBuilder) buildLenExpr(expr *ir.LenExpr) llvm.Value {
	switch t := ir.ToBaseType(expr.X.Type()).(type) {
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
