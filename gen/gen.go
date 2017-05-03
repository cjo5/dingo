package gen

import (
	"strconv"

	"fmt"

	"github.com/jhnl/interpreter/ast"
	"github.com/jhnl/interpreter/token"
	"github.com/jhnl/interpreter/vm"
)

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
	startBlock *block
	currBlock  *block

	// Jump targets used when compiling if and while statements.
	// If statement: target0 is branch taken and target1 is not taken.
	// While statement: target0 is loop condition and target1 is block following the loop.
	target0 *block
	target1 *block

	constants map[string]int
	globals   map[string]int

	flattenedBlocks []*block
}

// Compile generates bytecode in two steps.
// 1. Convert AST to CFG, where a node in the CFG is a basic block with bytecode instructions.
// 2. Flatten CFG to linear bytecode and patch jump addresses.
func Compile(mod *ast.Module) (vm.CodeMemory, vm.DataMemory) {
	c := &compiler{}
	c.constants = make(map[string]int, 4)
	c.globals = make(map[string]int, 4)
	c.startBlock = &block{}
	c.setNextBlock(c.startBlock)
	c.compileModule(mod)
	return c.createProgram()
}

// AST to CFG
//

func (c *compiler) defineConstant(literal string) int {
	if addr, ok := c.constants[literal]; ok {
		return addr
	}
	addr := len(c.constants)
	c.constants[literal] = addr
	return addr
}

func (c *compiler) insertGlobal(id string) int {
	if addr, ok := c.globals[id]; ok {
		return addr
	}
	addr := len(c.globals)
	c.globals[id] = addr
	return addr
}

func (c *compiler) lookupGlobal(id string) int {
	if addr, ok := c.globals[id]; ok {
		return addr
	}
	return -1
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

func (c *compiler) compileModule(mod *ast.Module) {
	c.compileStmtList(mod.Stmts)
}

func (c *compiler) compileStmtList(stmts []ast.Stmt) {
	for _, stmt := range stmts {
		c.compileStmt(stmt)
	}
}

func (c *compiler) compileStmt(stmt ast.Stmt) {
	switch t := stmt.(type) {
	case *ast.BlockStmt:
		c.compileBlockStmt(t)
	case *ast.DeclStmt:
		c.compileDeclStmt(t)
	case *ast.PrintStmt:
		c.compilePrintStmt(t)
	case *ast.IfStmt:
		c.compileIfStmt(t)
	case *ast.WhileStmt:
		c.compileWhileStmt(t)
	case *ast.BranchStmt:
		c.compileBranchStmt(t)
	case *ast.AssignStmt:
		c.compileAssignStmt(t)
	}
}

func (c *compiler) compileBlockStmt(stmt *ast.BlockStmt) {
	c.compileStmtList(stmt.Stmts)
}

func (c *compiler) compileDeclStmt(stmt *ast.DeclStmt) {
	c.compileExpr(stmt.X)
	addr := c.insertGlobal(stmt.Name.Name.Literal)
	c.currBlock.addInstr1(vm.Gstore, addr)
}

func (c *compiler) compilePrintStmt(stmt *ast.PrintStmt) {
	c.compileExpr(stmt.X)
	c.currBlock.addInstr0(vm.Print)
}

func (c *compiler) compileIfStmt(stmt *ast.IfStmt) {
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

		if _, ok := stmt.Else.(*ast.BlockStmt); ok {
			c.setNextBlock(&block{})
		}

		tmp.instructions[jumpIndex].target = c.currBlock
	}
}

func (c *compiler) compileWhileStmt(stmt *ast.WhileStmt) {
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

func (c *compiler) compileBranchStmt(stmt *ast.BranchStmt) {
	if stmt.Tok.ID == token.Continue {
		c.currBlock.addJumpInstr(vm.NewInstr0(vm.Goto), c.target0)
	} else { // break
		c.currBlock.addJumpInstr(vm.NewInstr0(vm.Goto), c.target1)
	}
}

func (c *compiler) compileAssignStmt(stmt *ast.AssignStmt) {
	assign := stmt.Assign.ID
	if assign == token.AddAssign || assign == token.SubAssign ||
		assign == token.MulAssign || assign == token.DivAssign ||
		assign == token.ModAssign {
		c.compileIdent(stmt.ID)
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
	addr := c.lookupGlobal(stmt.ID.Name.Literal)
	c.currBlock.addInstr1(vm.Gstore, addr)
}

func (c *compiler) compileExpr(expr ast.Expr) {
	switch t := expr.(type) {
	case *ast.BinaryExpr:
		c.compileBinaryExpr(t)
	case *ast.UnaryExpr:
		c.compileUnaryExpr(t)
	case *ast.Literal:
		c.compileLiteral(t)
	case *ast.Ident:
		c.compileIdent(t)
	}
}

func (c *compiler) compileBinaryExpr(expr *ast.BinaryExpr) {
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

func (c *compiler) compileUnaryExpr(expr *ast.UnaryExpr) {
	if expr.Op.ID == token.Sub {
		c.currBlock.addInstr1(vm.Iload, 0)
	}
	c.compileExpr(expr.X)
	if expr.Op.ID == token.Sub {
		c.currBlock.addInstr0(vm.BinarySub)
	} else {
		// TODO: logical NOT
		panic(fmt.Sprintf("compileUnaryExpr for op %s is not implemented", expr.Op))
	}
}

func (c *compiler) compileLiteral(lit *ast.Literal) {
	if lit.Value.ID == token.String {
		s := c.unescapeString(lit.Value.Literal)
		addr := c.defineConstant(s)
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

func (c *compiler) compileIdent(id *ast.Ident) {
	addr := c.lookupGlobal(id.Name.Literal)
	c.currBlock.addInstr1(vm.Gload, addr)
}

// CFG to linear bytecode
//

func (c *compiler) createProgram() (vm.CodeMemory, vm.DataMemory) {
	c.dfs(c.startBlock)
	c.calculateBlockAddresses()
	code := c.getCodeMemory()
	mem := c.getDataMemory()
	return code, mem
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
	code = append(code, vm.NewInstr0(vm.Halt))

	return code
}

func (c *compiler) getDataMemory() vm.DataMemory {
	var data vm.DataMemory

	data.Constants = make([]interface{}, len(c.constants))
	for k, v := range c.constants {
		data.Constants[v] = k
	}
	data.Globals = make([]interface{}, len(c.globals))
	for _, v := range c.globals {
		data.Globals[v] = 0
	}

	return data
}
