package codegen

import (
	"fmt"
	"math/big"

	"github.com/jhnl/interpreter/semantics"
	"github.com/jhnl/interpreter/token"
	"github.com/jhnl/interpreter/vm"
)

type function struct {
	constantAddress int
	argCount        int
	localCount      int
	entry           *block // Function entry point
}

type instruction struct {
	in     vm.Instruction
	target *block // Jump target
}

type block struct {
	next         *block
	instructions []instruction
	address      int
	visited      bool
}

type compiler struct {
	globalAddress   int
	constantAddress int
	// TODO: Improve handling of constant literals
	stringLiterals  map[string]int
	float64Literals map[float64]int
	float32Literals map[float32]int
	functions       map[string]*function
	startBlock      *block
	flattenedBlocks []*block

	// Jump targets used when compiling if and while statements.
	// If statement: target0 is branch taken and target1 is not taken.
	// While statement: target0 is loop condition and target1 is block following the loop.
	currBlock    *block
	target0      *block
	target1      *block
	scope        *semantics.Scope
	localAddress int
}

// Compile mod to bytecode.
// 1. Convert AST to CFG, where a node in the CFG is a basic block with bytecode instructions.
// 2. Flatten CFG to linear bytecode and patch jump addresses.
func Compile(mod *semantics.File) (int, vm.CodeMemory, vm.DataMemory) {
	c := &compiler{}
	c.stringLiterals = make(map[string]int)
	c.float64Literals = make(map[float64]int)
	c.float32Literals = make(map[float32]int)
	c.functions = make(map[string]*function)
	c.compileModule(mod)
	return c.createProgram()
}

// AST to CFG
//

func (c *compiler) defineString(literal string) int {
	if addr, ok := c.stringLiterals[literal]; ok {
		return addr
	}
	addr := c.constantAddress
	c.constantAddress++
	c.stringLiterals[literal] = addr
	return addr
}

func (c *compiler) defineFloat64(literal float64) int {
	if addr, ok := c.float64Literals[literal]; ok {
		return addr
	}
	addr := c.constantAddress
	c.constantAddress++
	c.float64Literals[literal] = addr
	return addr
}

func (c *compiler) defineFloat32(literal float32) int {
	if addr, ok := c.float32Literals[literal]; ok {
		return addr
	}
	addr := c.constantAddress
	c.constantAddress++
	c.float32Literals[literal] = addr
	return addr
}

func (c *compiler) defineVar(id string) (int, bool) {
	sym := c.scope.Lookup(id)
	if sym.Global {
		sym.Address = c.globalAddress
		c.globalAddress++
	} else {
		sym.Address = c.localAddress
		c.localAddress++
	}
	return sym.Address, sym.Global
}

func (c *compiler) lookupVar(id string) (int, bool) {
	sym := c.scope.Lookup(id)
	return sym.Address, sym.Global
}

func (c *compiler) lookupFunc(id string) (int, bool) {
	if f, ok := c.functions[id]; ok {
		return f.constantAddress, true
	}
	return -1, false
}

func (c *compiler) setNextBlock(block *block) {
	if c.currBlock != nil {
		c.currBlock.next = block
	}
	c.currBlock = block
}

func (b *block) addJumpInstr(in vm.Instruction, target *block) {
	in2 := instruction{in: in, target: target}
	b.instructions = append(b.instructions, in2)
}

func (b *block) addJumpInstr0(op vm.Opcode, target *block) {
	b.addJumpInstr(vm.NewInstr0(op), target)
}

func (b *block) addInstr0(op vm.Opcode) {
	b.addJumpInstr(vm.NewInstr0(op), nil)
}

func (b *block) addInstr1(op vm.Opcode, arg1 int64) {
	b.addJumpInstr(vm.NewInstr1(op, arg1), nil)
}

func (b *block) addInstrAddr(op vm.Opcode, arg1 int) {
	b.addJumpInstr(vm.NewInstr1(op, int64(arg1)), nil)
}

func (c *compiler) compileModule(mod *semantics.File) {
	var vars []*semantics.VarDecl
	var funcs []*semantics.FuncDecl

	c.scope = mod.Scope

	// Define addresses
	for _, decl := range mod.Decls {
		switch t := decl.(type) {
		case *semantics.VarDecl:
			c.defineVar(t.Name.Literal())
			vars = append(vars, t)
		case *semantics.FuncDecl:
			fun := &function{}
			fun.constantAddress = c.constantAddress
			c.constantAddress++
			c.functions[t.Name.Literal()] = fun
			funcs = append(funcs, t)
		default:
			panic(fmt.Sprintf("Unknown decl %T", t))
		}
	}

	// Ugly hack...
	// We first define globals so they have the correct addresses when the functions are compiled.
	// When the statements are compiled, the globals are defined once again so the globalAddress needs to be reset so they get the same address.
	c.globalAddress = 0

	// Compile functions
	for _, decl := range funcs {
		entry := &block{}
		c.currBlock = entry
		c.compileFuncDecl(decl)
	}

	c.startBlock = &block{}
	c.currBlock = c.startBlock

	// Compile globals
	for _, decl := range vars {
		c.compileVarDecl(decl)
	}

	// Invoke main function
	if addr, found := c.lookupFunc("main"); found {
		c.currBlock.addInstrAddr(vm.Call, addr)
		c.currBlock.addInstr0(vm.Pop)
	}

	// Halt program
	c.currBlock.addInstr0(vm.Halt)
}

func (c *compiler) compileStmtList(stmts []semantics.Stmt) {
	for _, stmt := range stmts {
		c.compileStmt(stmt)
	}
}

func (c *compiler) compileStmt(stmt semantics.Stmt) {
	switch t := stmt.(type) {
	case *semantics.BlockStmt:
		c.compileBlockStmt(t)
	case *semantics.DeclStmt:
		c.compileDeclStmt(t)
	case *semantics.PrintStmt:
		c.compilePrintStmt(t)
	case *semantics.IfStmt:
		c.compileIfStmt(t)
	case *semantics.WhileStmt:
		c.compileWhileStmt(t)
	case *semantics.ReturnStmt:
		c.compileReturnStmt(t)
	case *semantics.BranchStmt:
		c.compileBranchStmt(t)
	case *semantics.AssignStmt:
		c.compileAssignStmt(t)
	case *semantics.ExprStmt:
		c.compileExprStmt(t)
	default:
		panic(fmt.Sprintf("Unable to compile stmt %T", t))
	}
}

func (c *compiler) compileBlockStmt(stmt *semantics.BlockStmt) {
	outer := c.scope
	if stmt.Scope != nil {
		c.scope = stmt.Scope
	}
	c.compileStmtList(stmt.Stmts)
	c.scope = outer
}

func (c *compiler) compileDeclStmt(stmt *semantics.DeclStmt) {
	switch t := stmt.D.(type) {
	case *semantics.VarDecl:
		c.compileVarDecl(t)
	default:
		panic(fmt.Sprintf("Unable to compile decl stmt %T", t))
	}
}

func (c *compiler) compileVarDecl(stmt *semantics.VarDecl) {
	c.compileExpr(stmt.X)
	addr, global := c.defineVar(stmt.Name.Literal())
	if global {
		c.currBlock.addInstrAddr(vm.GStore, addr)
	} else {
		c.currBlock.addInstrAddr(vm.Store, addr)
	}
}

func (c *compiler) compileFuncDecl(stmt *semantics.FuncDecl) {
	fun, ok := c.functions[stmt.Name.Literal()]
	if !ok {
		panic(fmt.Sprintf("Function %s not defined", stmt.Name.Literal()))
	}

	fun.entry = c.currBlock
	fun.argCount = len(stmt.Params)

	outer := c.scope
	c.scope = stmt.Scope
	c.localAddress = 0
	c.compileBlockStmt(stmt.Body)

	fun.localCount = c.localAddress
	c.scope = outer
}

func (c *compiler) compilePrintStmt(stmt *semantics.PrintStmt) {
	c.compileExpr(stmt.X)
	c.currBlock.addInstr0(vm.Print)
}

func (c *compiler) compileIfStmt(stmt *semantics.IfStmt) {
	jump := &block{}
	seq := &block{}

	c.compileExpr(stmt.Cond)
	c.currBlock.addJumpInstr(vm.NewInstr0(vm.IfFalse), jump)
	c.setNextBlock(seq)
	c.compileBlockStmt(stmt.Body)

	if stmt.Else == nil {
		c.setNextBlock(jump)
	} else {
		// A sequence of if / else if / else is compiled recursively.
		// The jump instruction at the end of a conditional block needs
		// to be patched with the block that follows the chain.
		//

		tmp := c.currBlock
		jumpIndex := len(tmp.instructions)
		tmp.addInstr0(vm.Goto)
		c.setNextBlock(jump)
		c.compileStmt(stmt.Else)

		if _, ok := stmt.Else.(*semantics.BlockStmt); ok {
			c.setNextBlock(&block{})
		}

		tmp.instructions[jumpIndex].target = c.currBlock
	}
}

func (c *compiler) compileWhileStmt(stmt *semantics.WhileStmt) {
	loop := &block{}
	cond := &block{}
	join := &block{}
	c.target0 = cond
	c.target1 = join
	c.currBlock.addJumpInstr(vm.NewInstr0(vm.Goto), cond)
	c.setNextBlock(loop)
	c.compileBlockStmt(stmt.Body)
	c.setNextBlock(cond)
	c.compileExpr(stmt.Cond)
	c.currBlock.addJumpInstr(vm.NewInstr0(vm.IfTrue), loop)
	c.setNextBlock(join)
	c.target0 = nil
	c.target1 = nil
}

func (c *compiler) compileReturnStmt(stmt *semantics.ReturnStmt) {
	if stmt.X != nil {
		c.compileExpr(stmt.X)
	}
	c.currBlock.addInstr0(vm.Ret)
	c.setNextBlock(&block{})
}

func (c *compiler) compileBranchStmt(stmt *semantics.BranchStmt) {
	if stmt.Tok.ID == token.Continue {
		c.currBlock.addJumpInstr(vm.NewInstr0(vm.Goto), c.target0)
	} else { // break
		c.currBlock.addJumpInstr(vm.NewInstr0(vm.Goto), c.target1)
	}
}

func (c *compiler) compileAssignStmt(stmt *semantics.AssignStmt) {
	assign := stmt.Assign.ID
	if assign == token.AddAssign || assign == token.SubAssign ||
		assign == token.MulAssign || assign == token.DivAssign ||
		assign == token.ModAssign {
		c.compileIdent(stmt.Name)
	}
	c.compileExpr(stmt.Right)
	t := stmt.Name.Type().ID
	if assign == token.AddAssign {
		c.currBlock.addInstr0(vm.AddOp(t))
	} else if assign == token.SubAssign {
		c.currBlock.addInstr0(vm.SubOp(t))
	} else if assign == token.MulAssign {
		c.currBlock.addInstr0(vm.MulOp(t))
	} else if assign == token.DivAssign {
		c.currBlock.addInstr0(vm.DivOp(t))
	} else if assign == token.ModAssign {
		c.currBlock.addInstr0(vm.ModOp(t))
	}
	addr, global := c.lookupVar(stmt.Name.Literal())
	if global {
		c.currBlock.addInstrAddr(vm.GStore, addr)
	} else {
		c.currBlock.addInstrAddr(vm.Store, addr)
	}
}

func (c *compiler) compileExprStmt(stmt *semantics.ExprStmt) {
	c.compileExpr(stmt.X)

	if call, ok := stmt.X.(*semantics.FuncCall); ok {
		sym := call.Name.Sym
		pop := false
		if sym.ID == semantics.TypeSymbol {
			// Type cast
			pop = true
		} else {
			decl, _ := sym.Src.(*semantics.FuncDecl)
			if decl.Return.Type().ID == semantics.TVoid {
				pop = true
			}
		}
		if pop {
			// Pop unused return value
			c.currBlock.addInstr0(vm.Pop)
		}
	}
}

func (c *compiler) compileExpr(expr semantics.Expr) {
	switch t := expr.(type) {
	case *semantics.BinaryExpr:
		c.compileBinaryExpr(t)
	case *semantics.UnaryExpr:
		c.compileUnaryExpr(t)
	case *semantics.Literal:
		c.compileLiteral(t)
	case *semantics.Ident:
		c.compileIdent(t)
	case *semantics.FuncCall:
		c.compileFuncCall(t)
	default:
		panic(fmt.Sprintf("Unable to compile expr %T", t))
	}
}

func (c *compiler) compileBinaryExpr(expr *semantics.BinaryExpr) {
	if expr.Op.ID == token.Lor || expr.Op.ID == token.Land {
		// TODO: Make it more efficient if expression is part of control flow (if, while etc)

		jump2 := &block{}
		target0 := &block{} // false
		target1 := &block{} // true
		join := &block{}
		jump2.next = target0
		target0.addInstr1(vm.I32Load, 0)
		target0.addJumpInstr0(vm.Goto, join)
		target0.next = target1
		target1.addInstr1(vm.I32Load, 1)
		target1.next = join

		c.compileExpr(expr.Left)
		if expr.Op.ID == token.Lor {
			c.currBlock.addJumpInstr0(vm.IfTrue, target1)
			c.setNextBlock(jump2)
			c.compileExpr(expr.Right)
			c.currBlock.addJumpInstr0(vm.IfTrue, target1)
		} else { // Land
			c.currBlock.addJumpInstr0(vm.IfFalse, target0)
			c.setNextBlock(jump2)
			c.compileExpr(expr.Right)
			c.currBlock.addJumpInstr0(vm.IfTrue, target1)
		}
		c.currBlock = join
	} else {
		c.compileExpr(expr.Left)
		c.compileExpr(expr.Right)

		t := expr.Type().ID

		switch expr.Op.ID {
		case token.Add:
			c.currBlock.addInstr0(vm.AddOp(t))
		case token.Sub:
			c.currBlock.addInstr0(vm.SubOp(t))
		case token.Mul:
			c.currBlock.addInstr0(vm.MulOp(t))
		case token.Div:
			c.currBlock.addInstr0(vm.DivOp(t))
		case token.Mod:
			c.currBlock.addInstr0(vm.ModOp(t))
		case token.Eq, token.Neq, token.Gt, token.GtEq, token.Lt, token.LtEq:
			c.currBlock.addInstr0(vm.CmpOp(expr.Left.Type().ID))
			switch expr.Op.ID {
			case token.Eq:
				c.currBlock.addInstr0(vm.CmpEq)
			case token.Neq:
				c.currBlock.addInstr0(vm.CmpNe)
			case token.Gt:
				c.currBlock.addInstr0(vm.CmpGt)
			case token.GtEq:
				c.currBlock.addInstr0(vm.CmpGe)
			case token.Lt:
				c.currBlock.addInstr0(vm.CmpLt)
			case token.LtEq:
				c.currBlock.addInstr0(vm.CmpLe)
			}
		}
	}
}

func (c *compiler) compileUnaryExpr(expr *semantics.UnaryExpr) {
	binop := vm.Nop
	if expr.Op.ID == token.Sub {
		switch expr.T.ID {
		case semantics.TUInt32:
			c.currBlock.addInstr1(vm.U32Load, 0)
			binop = vm.U32Sub
		case semantics.TInt32:
			c.currBlock.addInstr1(vm.I32Load, 0)
			binop = vm.I32Sub
		}
	}

	c.compileExpr(expr.X)
	if expr.Op.ID == token.Sub {
		c.currBlock.addInstr0(binop)
	} else {
		c.currBlock.addInstr0(vm.Not)
	}
}

func (c *compiler) compileLiteral(lit *semantics.Literal) {
	if lit.Value.ID == token.String {
		if str, ok := lit.Raw.(string); ok {
			addr := c.defineString(str)
			c.currBlock.addInstrAddr(vm.CLoad, addr)
		} else {
			panic(fmt.Sprintf("Failed to convert raw literal %s with type %T", lit.Value, lit.Raw))
		}
	} else if lit.Value.ID == token.Integer || lit.Value.ID == token.Float {
		if bigInt, ok := lit.Raw.(*big.Int); ok {
			switch lit.T.ID {
			case semantics.TUInt64:
				c.currBlock.addInstr1(vm.U64Load, int64(bigInt.Uint64()))
			case semantics.TUInt32:
				c.currBlock.addInstr1(vm.U32Load, int64(bigInt.Uint64()))
			case semantics.TUInt16:
				c.currBlock.addInstr1(vm.U16Load, int64(bigInt.Uint64()))
			case semantics.TUInt8:
				c.currBlock.addInstr1(vm.U8Load, int64(bigInt.Uint64()))
			case semantics.TInt64:
				c.currBlock.addInstr1(vm.I64Load, int64(bigInt.Int64()))
			case semantics.TInt32:
				c.currBlock.addInstr1(vm.I32Load, int64(bigInt.Int64()))
			case semantics.TInt16:
				c.currBlock.addInstr1(vm.I16Load, int64(bigInt.Int64()))
			case semantics.TInt8:
				c.currBlock.addInstr1(vm.I8Load, int64(bigInt.Int64()))
			default:
				panic(fmt.Sprintf("Unhandled literal %s with type %s", lit.Value, lit.T.ID))
			}
		} else if bigFloat, ok := lit.Raw.(*big.Float); ok {
			addr := 0
			switch lit.T.ID {
			case semantics.TFloat64:
				val, _ := bigFloat.Float64()
				addr = c.defineFloat64(val)
			case semantics.TFloat32:
				val, _ := bigFloat.Float32()
				addr = c.defineFloat32(val)
			default:
				panic(fmt.Sprintf("Unhandled literal %s with type %s", lit.Value, lit.T.ID))
			}
			c.currBlock.addInstrAddr(vm.CLoad, addr)
		} else {
			panic(fmt.Sprintf("Failed to convert raw literal %s with type %T", lit.Value, lit.Raw))
		}
	} else if lit.Value.ID == token.True {
		c.currBlock.addInstr1(vm.I32Load, 1)
	} else if lit.Value.ID == token.False {
		c.currBlock.addInstr1(vm.I32Load, 0)
	} else {
		panic(fmt.Sprintf("Unhandled literal %s", lit.Value))
	}
}

func (c *compiler) compileIdent(id *semantics.Ident) {
	addr, global := c.lookupVar(id.Literal())
	if global {
		c.currBlock.addInstrAddr(vm.GLoad, addr)
	} else {
		c.currBlock.addInstrAddr(vm.Load, addr)
	}
}

func (c *compiler) compileFuncCall(expr *semantics.FuncCall) {
	for _, arg := range expr.Args {
		c.compileExpr(arg)
	}

	sym := expr.Name.Sym
	if sym.ID == semantics.TypeSymbol {
		dstType := sym.T

		switch dstType.ID {
		case semantics.TUInt64:
			c.currBlock.addInstr1(vm.U64Load, 0)
		case semantics.TUInt32:
			c.currBlock.addInstr1(vm.U32Load, 0)
		case semantics.TUInt16:
			c.currBlock.addInstr1(vm.U16Load, 0)
		case semantics.TUInt8:
			c.currBlock.addInstr1(vm.U8Load, 0)
		case semantics.TInt64:
			c.currBlock.addInstr1(vm.I64Load, 0)
		case semantics.TInt32:
			c.currBlock.addInstr1(vm.I32Load, 0)
		case semantics.TInt16:
			c.currBlock.addInstr1(vm.I16Load, 0)
		case semantics.TInt8:
			c.currBlock.addInstr1(vm.I8Load, 0)
		case semantics.TFloat64:
			addr := c.defineFloat64(0)
			c.currBlock.addInstrAddr(vm.CLoad, addr)
		case semantics.TFloat32:
			addr := c.defineFloat32(0)
			c.currBlock.addInstrAddr(vm.CLoad, addr)
		default:
			panic(fmt.Sprintf("Unhandled dest type %T", dstType.ID))
		}

		c.currBlock.addInstr0(vm.NumCast)
	} else {
		if addr, ok := c.lookupFunc(expr.Name.Literal()); ok {
			c.currBlock.addInstrAddr(vm.Call, addr)
			c.setNextBlock(&block{})
		} else {
			panic(fmt.Sprintf("Failed to find %s", expr.Name.Name))
		}
	}
}

// CFG to linear bytecode
//

func (c *compiler) createProgram() (int, vm.CodeMemory, vm.DataMemory) {
	for _, fun := range c.functions {
		c.dfs(fun.entry)
	}
	c.dfs(c.startBlock)
	c.calculateBlockAddresses()
	entry := c.startBlock.address
	code := c.getCodeMemory()
	mem := c.getDataMemory()
	return entry, code, mem
}

// Flatten the CFG by doing a postordering depth-first search of the graph.
func (c *compiler) dfs(block *block) {
	if block.visited {
		return
	}
	block.visited = true
	if block.next != nil {
		c.dfs(block.next)
	}
	for _, in := range block.instructions {
		if in.target != nil {
			c.dfs(in.target)
		}
	}
	c.flattenedBlocks = append(c.flattenedBlocks, block)
}

func (c *compiler) calculateBlockAddresses() {
	addr := 0
	for i := len(c.flattenedBlocks) - 1; i >= 0; i-- {
		b := c.flattenedBlocks[i]
		b.address = addr
		addr += len(b.instructions)
	}
}

func (c *compiler) getCodeMemory() vm.CodeMemory {
	var code vm.CodeMemory

	for i := len(c.flattenedBlocks) - 1; i >= 0; i-- {
		b := c.flattenedBlocks[i]
		for _, in := range b.instructions {
			// Patch jump address
			if in.target != nil {
				in.in.Arg1 = int64(in.target.address)
			}
			code = append(code, in.in)
		}
	}

	return code
}

func (c *compiler) getDataMemory() vm.DataMemory {
	var data vm.DataMemory

	data.Constants = make([]interface{}, c.constantAddress)
	for k, v := range c.stringLiterals {
		data.Constants[v] = k
	}

	for k, v := range c.float64Literals {
		data.Constants[v] = k
	}

	for k, v := range c.float32Literals {
		data.Constants[v] = k
	}

	for k, f := range c.functions {
		desc := &vm.FunctionDescriptor{Name: k, Address: f.entry.address, ArgCount: f.argCount, LocalCount: f.localCount}
		data.Constants[f.constantAddress] = desc
	}

	data.Globals = make([]interface{}, c.globalAddress)
	for i := range data.Globals {
		data.Globals[i] = 0
	}

	return data
}
