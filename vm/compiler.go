package vm

import (
	"fmt"
	"math/big"

	"github.com/jhnl/interpreter/semantics"
	"github.com/jhnl/interpreter/token"
)

// Compile to bytecode.
// 1. Convert AST to CFG, where a node in the CFG is a basic block with bytecode instructions.
// 2. Flatten CFG to linear bytecode and patch jump addresses.
func Compile(prog *semantics.Program) *BytecodeProgram {
	c := &compiler{}

	bootstrapFun := newFunctionUnit("<init>")
	bootstrapFun.entry = &basicBlock{}
	bootstrapMod := newModuleUnit(0, 0, "<bootstrap>", "")
	bootstrapMod.functions = append(bootstrapMod.functions, bootstrapFun)
	c.compiledModules = append(c.compiledModules, bootstrapMod)

	semantics.StartProgramWalk(c, prog)

	mainModAddr := -1
	for i := 1; i < len(c.compiledModules); i++ {
		bootstrapFun.entry.addInstrAddr(SetMod, i)
		bootstrapFun.entry.addInstrAddr(Call, 0) // Module init function is defined at address 0

		mod := c.compiledModules[i]
		if mod.main != nil {
			mainModAddr = i
		}
	}

	if mainModAddr > -1 {
		main := c.compiledModules[mainModAddr].main
		bootstrapFun.entry.addInstrAddr(SetMod, mainModAddr)
		bootstrapFun.entry.addInstrAddr(Call, main.Address)
	}

	return c.createBytecodeProgram()
}

type moduleUnit struct {
	id      int
	obj     *ModuleObject
	address int
	main    *semantics.Symbol

	globalAddress int
	localAddress  int

	constants []interface{}
	functions []*functionUnit
}

type functionUnit struct {
	obj          *FunctionObject
	entry        *basicBlock // Function entry point
	sortedBlocks []*basicBlock
}

type compiler struct {
	semantics.BaseVisitor
	module          *moduleUnit
	compiledModules []*moduleUnit

	// Jump targets used when compiling if and while statements.
	// If statement: target0 is branch taken and target1 is not taken.
	// While statement: target0 is loop condition and target1 is block following the loop.
	currBlock *basicBlock
	target0   *basicBlock
	target1   *basicBlock

	scope *semantics.Scope
}

type basicBlock struct {
	next         *basicBlock
	edges        []edge
	instructions []Instruction
	address      int
	visited      bool
}

type edge struct {
	// Index to instructions array in basicBlock where edge is stored (not target).
	// The instruction needs to be patched with the correct target address.
	instrIndex int
	target     *basicBlock
}

func newModuleUnit(id int, address int, name string, path string) *moduleUnit {
	mod := &moduleUnit{id: id, address: address}
	mod.obj = &ModuleObject{Name: name, Path: path}
	return mod
}

func newFunctionUnit(name string) *functionUnit {
	fun := &functionUnit{}
	fun.obj = &FunctionObject{Name: name}
	fun.entry = &basicBlock{}
	return fun
}

func (m *moduleUnit) defineConstant(constant interface{}) int {
	switch t1 := constant.(type) {
	case string:
		for idx, v := range m.constants {
			if t2, ok := v.(string); ok {
				if t1 == t2 {
					return idx
				}
			}
		}
	case float64:
		for idx, v := range m.constants {
			if t2, ok := v.(float64); ok {
				if t1 == t2 {
					return idx
				}
			}
		}
	case float32:
		for idx, v := range m.constants {
			if t2, ok := v.(float32); ok {
				if t1 == t2 {
					return idx
				}
			}
		}
	}

	addr := len(m.constants)
	m.constants = append(m.constants, constant)
	return addr
}

func (m *moduleUnit) reserveConstantAddr() int {
	addr := len(m.constants)
	m.constants = append(m.constants, nil)
	return addr
}

func (m *moduleUnit) reserveFunctionAddr() int {
	addr := len(m.functions)
	m.functions = append(m.functions, nil)
	return addr
}

func dfsSortBlocks(block *basicBlock, sortedBlocks *[]*basicBlock) {
	if block.visited {
		return
	}
	block.visited = true
	if block.next != nil {
		dfsSortBlocks(block.next, sortedBlocks)
	}
	for _, edge := range block.edges {
		dfsSortBlocks(edge.target, sortedBlocks)
	}
	*sortedBlocks = append(*sortedBlocks, block)
}

func (f *functionUnit) sortBlocks(block *basicBlock) {
	var sortedBlocks []*basicBlock
	dfsSortBlocks(f.entry, &sortedBlocks)
	f.sortedBlocks = nil
	for i := len(sortedBlocks) - 1; i >= 0; i-- {
		f.sortedBlocks = append(f.sortedBlocks, sortedBlocks[i])
	}
}

func (f *functionUnit) patchJumpAddresses() {
	// Calculate addresses
	addr := 0
	for _, b := range f.sortedBlocks {
		b.address = addr
		addr += len(b.instructions)
	}
	// Patch jumps
	for _, b := range f.sortedBlocks {
		for _, edge := range b.edges {
			b.instructions[edge.instrIndex].Arg1 = int64(edge.target.address)
		}
	}
}

func (f *functionUnit) getFunctionObject() *FunctionObject {
	f.obj.Code = nil
	for _, b := range f.sortedBlocks {
		f.obj.Code = append(f.obj.Code, b.instructions...)
	}
	return f.obj
}

func (c *compiler) createBytecodeProgram() *BytecodeProgram {
	code := &BytecodeProgram{}
	for _, mod := range c.compiledModules {
		obj := mod.obj
		obj.Constants = mod.constants
		obj.Globals = make([]interface{}, mod.globalAddress)
		for _, fun := range mod.functions {
			fun.sortBlocks(fun.entry)
			fun.patchJumpAddresses()
			obj.Functions = append(obj.Functions, fun.getFunctionObject())
		}
		code.Modules = append(code.Modules, mod.obj)
	}
	return code
}

func (c *compiler) setNextBlock(block *basicBlock) {
	if c.currBlock != nil {
		c.currBlock.next = block
	}
	c.currBlock = block
}

func (c *compiler) moduleAddress(moduleID int) int {
	for _, mod := range c.compiledModules {
		if mod.id == moduleID {
			return mod.address
		}
	}
	panic(fmt.Sprintf("Failed to find module with ID %d", moduleID))
}

func (b *basicBlock) addJumpInstr(in Instruction, target *basicBlock) {
	idx := len(b.instructions)
	edge := edge{instrIndex: idx, target: target}
	b.instructions = append(b.instructions, in)
	b.edges = append(b.edges, edge)
}

func (b *basicBlock) addJumpInstr0(op Opcode, target *basicBlock) {
	b.addJumpInstr(NewInstr0(op), target)
}

func (b *basicBlock) addInstr0(op Opcode) {
	b.instructions = append(b.instructions, NewInstr0(op))
}

func (b *basicBlock) addInstr1(op Opcode, arg1 int64) {
	b.instructions = append(b.instructions, NewInstr1(op, arg1))
}

func (b *basicBlock) addInstrAddr(op Opcode, arg1 int) {
	b.instructions = append(b.instructions, NewInstr1(op, int64(arg1)))
}

func (c *compiler) Module(mod *semantics.Module) {
	c.module = newModuleUnit(mod.ID, len(c.compiledModules), mod.Name.Literal, mod.Path)
	c.compiledModules = append(c.compiledModules, c.module)

	if mod.Main() {
		if main := mod.Internal.Lookup("main"); main != nil {
			if main.ID == semantics.FuncSymbol {
				c.module.main = main
			}
		}
	}

	// Generate addresses for top-level decls

	init := newFunctionUnit("<init>")
	c.module.functions = append(c.module.functions, init)

	var valDecls []*semantics.ValTopDecl
	var decls []semantics.TopDecl
	for _, decl := range mod.Decls {
		switch t := decl.(type) {
		case *semantics.ValTopDecl:
			valDecls = append(valDecls, t)
		case *semantics.FuncDecl:
			t.Sym.Address = c.module.reserveFunctionAddr()
			decls = append(decls, t)
		case *semantics.StructDecl:
			t.Sym.Address = c.module.reserveConstantAddr()
			decls = append(decls, t)
		default:
			panic(fmt.Sprintf("Unhandled TopDecl %T", t))
		}
	}

	// Compile top-level decls

	c.scope = mod.Internal

	// Init val decls in init function
	for _, decl := range valDecls {
		c.currBlock = init.entry
		c.VisitValTopDecl(decl)
	}

	for _, decl := range decls {
		c.currBlock = nil
		semantics.VisitDecl(c, decl)
	}
}

func (c *compiler) VisitValTopDecl(decl *semantics.ValTopDecl) {
	semantics.VisitExpr(c, decl.Initializer)

	addr := c.module.globalAddress
	c.module.globalAddress++

	decl.Sym.Address = addr
	c.currBlock.addInstrAddr(GlobalStore, addr)
}

func (c *compiler) VisitValDecl(decl *semantics.ValDecl) {
	semantics.VisitExpr(c, decl.Initializer)

	addr := c.module.localAddress
	c.module.localAddress++

	decl.Sym.Address = addr
	c.currBlock.addInstrAddr(Store, addr)
}

func (c *compiler) VisitFuncDecl(decl *semantics.FuncDecl) {
	fun := newFunctionUnit(decl.Name.Literal)
	fun.obj.ArgCount = len(decl.Params)
	if decl.TReturn.Type().ID() != semantics.TVoid {
		fun.obj.ReturnValue = true
	}

	outer := c.scope
	c.scope = decl.Scope

	for i, p := range decl.Params {
		p.Sym.Address = i
	}
	c.module.localAddress = len(decl.Params)

	c.currBlock = fun.entry
	c.VisitBlockStmt(decl.Body)
	fun.obj.LocalCount = c.module.localAddress

	c.scope = outer
	addr := decl.Sym.Address
	c.module.functions[addr] = fun
}

func (c *compiler) VisitStructDecl(decl *semantics.StructDecl) {
	desc := &StructDescriptor{Name: decl.Name.Literal, FieldCount: len(decl.Fields)}
	c.module.constants[decl.Sym.Address] = desc
	for i, f := range decl.Fields {
		f.Sym.Address = i
	}
}

func (c *compiler) VisitBlockStmt(stmt *semantics.BlockStmt) {
	outer := c.scope
	c.scope = stmt.Scope
	semantics.VisitStmtList(c, stmt.Stmts)
	c.scope = outer
}

func (c *compiler) VisitDeclStmt(stmt *semantics.DeclStmt) {
	switch t := stmt.D.(type) {
	case *semantics.ValDecl:
		c.VisitValDecl(t)
	default:
		panic(fmt.Sprintf("Unhandled DeclStmt %T", t))
	}
}

func (c *compiler) VisitPrintStmt(stmt *semantics.PrintStmt) {
	for _, x := range stmt.Xs {
		semantics.VisitExpr(c, x)
	}
	c.currBlock.addInstr1(Print, int64(len(stmt.Xs)))
}

func (c *compiler) VisitIfStmt(stmt *semantics.IfStmt) {
	jump := &basicBlock{}
	seq := &basicBlock{}

	semantics.VisitExpr(c, stmt.Cond)
	c.currBlock.addJumpInstr(NewInstr0(IfFalse), jump)
	c.setNextBlock(seq)
	c.VisitBlockStmt(stmt.Body)

	if stmt.Else == nil {
		c.setNextBlock(jump)
	} else {
		// A sequence of if / else if / else is compiled recursively.
		// The jump instruction at the end of a conditional block needs
		// to be patched with the block that follows the chain.
		//

		tmp := c.currBlock
		jumpIndex := len(tmp.instructions)
		tmp.addInstr0(Goto)
		c.setNextBlock(jump)
		semantics.VisitStmt(c, stmt.Else)

		if _, ok := stmt.Else.(*semantics.BlockStmt); ok {
			c.setNextBlock(&basicBlock{})
		}

		edge := edge{instrIndex: jumpIndex, target: c.currBlock}
		tmp.edges = append(tmp.edges, edge)
	}
}

func (c *compiler) VisitWhileStmt(stmt *semantics.WhileStmt) {
	loop := &basicBlock{}
	cond := &basicBlock{}
	join := &basicBlock{}
	c.target0 = cond
	c.target1 = join
	c.currBlock.addJumpInstr(NewInstr0(Goto), cond)
	c.setNextBlock(loop)
	c.VisitBlockStmt(stmt.Body)
	c.setNextBlock(cond)
	semantics.VisitExpr(c, stmt.Cond)
	c.currBlock.addJumpInstr(NewInstr0(IfTrue), loop)
	c.setNextBlock(join)
	c.target0 = nil
	c.target1 = nil
}

func (c *compiler) VisitReturnStmt(stmt *semantics.ReturnStmt) {
	if stmt.X != nil {
		semantics.VisitExpr(c, stmt.X)
	}
	c.currBlock.addInstr0(Ret)
	c.setNextBlock(&basicBlock{})
}

func (c *compiler) VisitBranchStmt(stmt *semantics.BranchStmt) {
	if stmt.Tok.ID == token.Continue {
		c.currBlock.addJumpInstr(NewInstr0(Goto), c.target0)
	} else if stmt.Tok.ID == token.Break {
		c.currBlock.addJumpInstr(NewInstr0(Goto), c.target1)
	} else {
		panic(fmt.Sprintf("Unhandled BranchStmt %s", stmt.Tok))
	}
}

func (c *compiler) VisitAssignStmt(stmt *semantics.AssignStmt) {
	assign := stmt.Assign.ID
	if assign == token.AddAssign || assign == token.SubAssign ||
		assign == token.MulAssign || assign == token.DivAssign ||
		assign == token.ModAssign {
		semantics.VisitExpr(c, stmt.Left)
	}
	semantics.VisitExpr(c, stmt.Right)
	t := stmt.Left.Type().ID()
	if assign == token.AddAssign {
		c.currBlock.addInstr0(AddOp(t))
	} else if assign == token.SubAssign {
		c.currBlock.addInstr0(SubOp(t))
	} else if assign == token.MulAssign {
		c.currBlock.addInstr0(MulOp(t))
	} else if assign == token.DivAssign {
		c.currBlock.addInstr0(DivOp(t))
	} else if assign == token.ModAssign {
		c.currBlock.addInstr0(ModOp(t))
	}

	var sym *semantics.Symbol
	switch t := stmt.Left.(type) {
	case *semantics.Ident:
		sym = t.Sym
	case *semantics.DotIdent:
		semantics.VisitExpr(c, t.X)
		sym = t.Name.Sym
	default:
		panic(fmt.Sprintf("Unhandled assign expr %T", t))
	}

	switch sym.ScopeID {
	case semantics.TopScope:
		c.currBlock.addInstrAddr(GlobalStore, sym.Address)
	case semantics.FieldScope:
		c.currBlock.addInstrAddr(FieldStore, sym.Address)
	case semantics.LocalScope:
		c.currBlock.addInstrAddr(Store, sym.Address)
	default:
		panic(fmt.Sprintf("Unhandled scope ID %d", sym.ScopeID))
	}

	if sym.ModuleID != c.module.id {
		c.currBlock.addInstrAddr(SetMod, c.module.address)
	}
}

func (c *compiler) VisitExprStmt(stmt *semantics.ExprStmt) {
	semantics.VisitExpr(c, stmt.X)

	typ := stmt.X.Type()
	if typ.ID() != semantics.TVoid {
		// Pop unused return value
		c.currBlock.addInstr0(Pop)
	}
}

func (c *compiler) VisitBinaryExpr(expr *semantics.BinaryExpr) semantics.Expr {
	if expr.Op.ID == token.Lor || expr.Op.ID == token.Land {
		// TODO: Make it more efficient if expression is part of control flow (if, while etc)

		jump2 := &basicBlock{}
		target0 := &basicBlock{} // false
		target1 := &basicBlock{} // true
		join := &basicBlock{}
		jump2.next = target0
		target0.addInstr1(BoolLoad, 0)
		target0.addJumpInstr0(Goto, join)
		target0.next = target1
		target1.addInstr1(BoolLoad, 1)
		target1.next = join

		semantics.VisitExpr(c, expr.Left)
		if expr.Op.ID == token.Lor {
			c.currBlock.addJumpInstr0(IfTrue, target1)
			c.setNextBlock(jump2)
			semantics.VisitExpr(c, expr.Right)
			c.currBlock.addJumpInstr0(IfTrue, target1)
		} else { // Land
			c.currBlock.addJumpInstr0(IfFalse, target0)
			c.setNextBlock(jump2)
			semantics.VisitExpr(c, expr.Right)
			c.currBlock.addJumpInstr0(IfTrue, target1)
		}
		c.currBlock = join
	} else {
		semantics.VisitExpr(c, expr.Left)
		semantics.VisitExpr(c, expr.Right)

		t := expr.Type().ID()

		switch expr.Op.ID {
		case token.Add:
			c.currBlock.addInstr0(AddOp(t))
		case token.Sub:
			c.currBlock.addInstr0(SubOp(t))
		case token.Mul:
			c.currBlock.addInstr0(MulOp(t))
		case token.Div:
			c.currBlock.addInstr0(DivOp(t))
		case token.Mod:
			c.currBlock.addInstr0(ModOp(t))
		case token.Eq, token.Neq, token.Gt, token.GtEq, token.Lt, token.LtEq:
			c.currBlock.addInstr0(CmpOp(expr.Left.Type().ID()))
			switch expr.Op.ID {
			case token.Eq:
				c.currBlock.addInstr0(CmpEq)
			case token.Neq:
				c.currBlock.addInstr0(CmpNe)
			case token.Gt:
				c.currBlock.addInstr0(CmpGt)
			case token.GtEq:
				c.currBlock.addInstr0(CmpGe)
			case token.Lt:
				c.currBlock.addInstr0(CmpLt)
			case token.LtEq:
				c.currBlock.addInstr0(CmpLe)
			}
		}
	}
	return expr
}

func (c *compiler) VisitUnaryExpr(expr *semantics.UnaryExpr) semantics.Expr {
	binop := Nop
	if expr.Op.ID == token.Sub {
		loadop := LoadOp(expr.T.ID())
		arg := 0
		if expr.T.ID() == semantics.TFloat64 {
			arg = c.module.defineConstant(float64(0))
		} else if expr.T.ID() == semantics.TFloat32 {
			arg = c.module.defineConstant(float32(0))
		}
		c.currBlock.addInstr1(loadop, int64(arg))
	}

	semantics.VisitExpr(c, expr.X)
	if expr.Op.ID == token.Sub {
		c.currBlock.addInstr0(binop)
	} else {
		c.currBlock.addInstr0(Not)
	}
	return expr
}

func (c *compiler) VisitBasicLit(expr *semantics.BasicLit) semantics.Expr {
	if expr.Value.ID == token.String {
		if str, ok := expr.Raw.(string); ok {
			addr := c.module.defineConstant(str)
			c.currBlock.addInstrAddr(ConstLoad, addr)
		} else {
			panic(fmt.Sprintf("Failed to convert raw expreral %s with type %T", expr.Value, expr.Raw))
		}
	} else if expr.Value.ID == token.Integer || expr.Value.ID == token.Float {
		if bigInt, ok := expr.Raw.(*big.Int); ok {
			switch expr.T.ID() {
			case semantics.TUInt64:
				c.currBlock.addInstr1(U64Load, int64(bigInt.Uint64()))
			case semantics.TUInt32:
				c.currBlock.addInstr1(U32Load, int64(bigInt.Uint64()))
			case semantics.TUInt16:
				c.currBlock.addInstr1(U16Load, int64(bigInt.Uint64()))
			case semantics.TUInt8:
				c.currBlock.addInstr1(U8Load, int64(bigInt.Uint64()))
			case semantics.TInt64:
				c.currBlock.addInstr1(I64Load, int64(bigInt.Int64()))
			case semantics.TInt32:
				c.currBlock.addInstr1(I32Load, int64(bigInt.Int64()))
			case semantics.TInt16:
				c.currBlock.addInstr1(I16Load, int64(bigInt.Int64()))
			case semantics.TInt8:
				c.currBlock.addInstr1(I8Load, int64(bigInt.Int64()))
			default:
				panic(fmt.Sprintf("Unhandled Expr %s with type %s", expr.Value, expr.T))
			}
		} else if bigFloat, ok := expr.Raw.(*big.Float); ok {
			addr := 0
			switch expr.T.ID() {
			case semantics.TFloat64:
				val, _ := bigFloat.Float64()
				addr = c.module.defineConstant(val)
			case semantics.TFloat32:
				val, _ := bigFloat.Float32()
				addr = c.module.defineConstant(val)
			default:
				panic(fmt.Sprintf("Unhandled Literal %s with type %s", expr.Value, expr.T))
			}
			c.currBlock.addInstrAddr(ConstLoad, addr)
		} else {
			panic(fmt.Sprintf("Failed to convert raw Literal %s with type %T", expr.Value, expr.Raw))
		}
	} else if expr.Value.ID == token.True {
		c.currBlock.addInstr1(BoolLoad, 1)
	} else if expr.Value.ID == token.False {
		c.currBlock.addInstr1(BoolLoad, 0)
	} else {
		panic(fmt.Sprintf("Unhandled Literal %s", expr.Value))
	}
	return expr
}

func (c *compiler) VisitStructLit(expr *semantics.StructLit) semantics.Expr {
	for _, init := range expr.Initializers {
		semantics.VisitExpr(c, init.Value)
	}
	var sym *semantics.Symbol
	switch id := expr.Name.(type) {
	case *semantics.Ident:
		sym = id.Sym
	case *semantics.DotIdent:
		semantics.VisitExpr(c, id.X)
		sym = id.Name.Sym
	default:
		panic(fmt.Sprintf("Unhandled struct literal expr %T", id))
	}
	c.currBlock.addInstrAddr(ConstLoad, sym.Address)
	c.currBlock.addInstr0(NewStruct)

	if sym.ModuleID != c.module.id {
		c.currBlock.addInstrAddr(SetMod, c.module.address)
	}
	return expr
}

func (c *compiler) VisitIdent(expr *semantics.Ident) semantics.Expr {
	sym := expr.Sym
	switch sym.ScopeID {
	case semantics.TopScope:
		if sym.ID == semantics.ModuleSymbol {
			modt, _ := sym.T.(*semantics.ModuleType)
			c.currBlock.addInstrAddr(SetMod, c.moduleAddress(modt.ModuleID))
		} else {
			c.currBlock.addInstrAddr(GlobalLoad, sym.Address)
		}
	case semantics.FieldScope:
		c.currBlock.addInstrAddr(FieldLoad, sym.Address)
	case semantics.LocalScope:
		c.currBlock.addInstrAddr(Load, sym.Address)
	default:
		panic(fmt.Sprintf("Unhandled scope ID %d", sym.ScopeID))
	}
	return expr
}

func (c *compiler) VisitDotIdent(expr *semantics.DotIdent) semantics.Expr {
	semantics.VisitExpr(c, expr.X)
	c.VisitIdent(expr.Name)
	return expr
}

func (c *compiler) VisitFuncCall(expr *semantics.FuncCall) semantics.Expr {
	semantics.VisitExprList(c, expr.Args)

	var sym *semantics.Symbol
	switch id := expr.X.(type) {
	case *semantics.Ident:
		sym = id.Sym
	case *semantics.DotIdent:
		semantics.VisitExpr(c, id.X)
		sym = id.Name.Sym
	default:
		panic(fmt.Sprintf("Unhandled function expr %T", id))
	}

	if sym.ID == semantics.FuncSymbol {
		c.currBlock.addInstrAddr(Call, sym.Address)
		c.setNextBlock(&basicBlock{})
	} else {
		dstType := expr.X.Type()

		arg := 0
		op := LoadOp(dstType.ID())
		if dstType.ID() == semantics.TFloat64 {
			arg = c.module.defineConstant(float64(0))
		} else if dstType.ID() == semantics.TFloat32 {
			arg = c.module.defineConstant(float32(0))
		}

		c.currBlock.addInstr1(op, int64(arg))
		c.currBlock.addInstr0(NumCast)
	}

	if sym.ModuleID != c.module.id {
		c.currBlock.addInstrAddr(SetMod, c.module.address)
	}

	return expr
}
