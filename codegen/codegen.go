package codegen

import (
	"strconv"

	"fmt"

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
	stringLiterals  map[string]int
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
func Compile(mod *semantics.Module) (int, vm.CodeMemory, vm.DataMemory) {
	c := &compiler{}
	c.stringLiterals = make(map[string]int)
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

func (c *compiler) defineVar(id string) (int, bool) {
	sym, global := c.scope.Lookup(id)
	if global {
		sym.Address = c.globalAddress
		c.globalAddress++
	} else {
		sym.Address = c.localAddress
		c.localAddress++
	}
	return sym.Address, global
}

func (c *compiler) lookupVar(id string) (int, bool) {
	sym, global := c.scope.Lookup(id)
	return sym.Address, global
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

func (b *block) addInstr1(op vm.Opcode, arg1 int) {
	b.addJumpInstr(vm.NewInstr1(op, arg1), nil)
}

func (c *compiler) compileModule(mod *semantics.Module) {
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
		c.currBlock.addInstr1(vm.Call, addr)
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
		c.currBlock.addInstr1(vm.Gstore, addr)
	} else {
		c.currBlock.addInstr1(vm.Store, addr)
	}
}

func (c *compiler) compileFuncDecl(stmt *semantics.FuncDecl) {
	fun, ok := c.functions[stmt.Name.Literal()]
	if !ok {
		panic(fmt.Sprintf("Function %s not defined", stmt.Name.Literal()))
	}

	fun.entry = c.currBlock
	fun.argCount = len(stmt.Fields)

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
	c.compileExpr(stmt.X)
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
	if assign == token.AddAssign {
		c.currBlock.addInstr0(vm.BinaryAdd)
	} else if assign == token.SubAssign {
		c.currBlock.addInstr0(vm.BinarySub)
	} else if assign == token.MulAssign {
		c.currBlock.addInstr0(vm.BinaryMul)
	} else if assign == token.DivAssign {
		c.currBlock.addInstr0(vm.BinaryDiv)
	} else if assign == token.ModAssign {
		c.currBlock.addInstr0(vm.BinaryMod)
	}
	addr, global := c.lookupVar(stmt.Name.Literal())
	if global {
		c.currBlock.addInstr1(vm.Gstore, addr)
	} else {
		c.currBlock.addInstr1(vm.Store, addr)
	}
}

func (c *compiler) compileExprStmt(stmt *semantics.ExprStmt) {
	c.compileExpr(stmt.X)

	if _, ok := stmt.X.(*semantics.CallExpr); ok {
		// Pop unused return value
		c.currBlock.addInstr0(vm.Pop)
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
	case *semantics.CallExpr:
		c.compileCallExpr(t)
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
		target0.addInstr1(vm.Iload, 0)
		target0.addJumpInstr0(vm.Goto, join)
		target0.next = target1
		target1.addInstr1(vm.Iload, 1)
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

		switch expr.Op.ID {
		case token.Add:
			c.currBlock.addInstr0(vm.BinaryAdd)
		case token.Sub:
			c.currBlock.addInstr0(vm.BinarySub)
		case token.Mul:
			c.currBlock.addInstr0(vm.BinaryMul)
		case token.Div:
			c.currBlock.addInstr0(vm.BinaryDiv)
		case token.Mod:
			c.currBlock.addInstr0(vm.BinaryMod)
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

func (c *compiler) compileUnaryExpr(expr *semantics.UnaryExpr) {
	c.compileExpr(expr.X)
	if expr.Op.ID == token.Sub {
		c.currBlock.addInstr0(vm.Neg)
	} else {
		c.currBlock.addInstr0(vm.Not)
	}
}

func (c *compiler) compileLiteral(lit *semantics.Literal) {
	if lit.Value.ID == token.String {
		s := c.unescapeString(lit.Value.Literal)
		addr := c.defineString(s)
		c.currBlock.addInstr1(vm.Cload, addr)
	} else if lit.Value.ID == token.Int {
		val, err := strconv.Atoi(lit.Value.Literal)
		if err != nil {
			panic(err)
		}
		c.currBlock.addInstr1(vm.Iload, val)
	} else if lit.Value.ID == token.True {
		c.currBlock.addInstr1(vm.Iload, 1)
	} else if lit.Value.ID == token.False {
		c.currBlock.addInstr1(vm.Iload, 0)
	}
}

func (c *compiler) unescapeString(literal string) string {
	// TODO:
	// - Should this be done some place else?
	// - Improve rune handling

	escaped := []rune(literal)
	var unescaped []rune

	start := 0
	n := len(escaped)

	// Remove quotes
	if n >= 2 {
		start++
		n--
	}

	for i := start; i < n; i++ {
		ch1 := escaped[i]
		if ch1 == '\\' && (i+1) < len(escaped) {
			i++
			ch2 := escaped[i]

			if ch2 == 'a' {
				ch1 = 0x07
			} else if ch2 == 'b' {
				ch1 = 0x08
			} else if ch2 == 'f' {
				ch1 = 0x0c
			} else if ch2 == 'n' {
				ch1 = 0x0a
			} else if ch2 == 'r' {
				ch1 = 0x0d
			} else if ch2 == 't' {
				ch1 = 0x09
			} else if ch2 == 'v' {
				ch1 = 0x0b
			} else {
				ch1 = ch2
			}
		}
		unescaped = append(unescaped, ch1)
	}

	return string(unescaped)
}

func (c *compiler) compileIdent(id *semantics.Ident) {
	addr, global := c.lookupVar(id.Literal())
	if global {
		c.currBlock.addInstr1(vm.Gload, addr)
	} else {
		c.currBlock.addInstr1(vm.Load, addr)
	}
}

func (c *compiler) compileCallExpr(expr *semantics.CallExpr) {
	for _, arg := range expr.Args {
		c.compileExpr(arg)
	}
	if addr, ok := c.lookupFunc(expr.Name.Literal()); ok {
		c.currBlock.addInstr1(vm.Call, addr)
		c.setNextBlock(&block{})
	} else {
		panic(fmt.Sprintf("Failed to find %s", expr.Name.Name))
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
				in.in.Arg1 = in.target.address
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
