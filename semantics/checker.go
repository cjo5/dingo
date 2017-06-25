package semantics

import (
	"fmt"

	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/token"
)

// Check will resolve identifiers and do type checking.
func Check(mod *Module) error {
	var c checker

	c.checkModule(mod)
	if len(c.errors) > 0 {
		return c.errors
	}

	return nil
}

type checker struct {
	scope      *Scope
	currGlobal *Symbol
	errors     common.ErrorList
}

func (c *checker) error(tok token.Token, format string, args ...interface{}) {
	c.errors.Add(tok.Pos, format, args...)
}

func (c *checker) openScope() {
	c.scope = NewScope(c.scope)
}

func (c *checker) closeScope() {
	c.scope = c.scope.Outer
}

func (c *checker) declare(id SymbolID, name token.Token, node Node) *Symbol {
	sym := NewSymbol(id, name, node, c.isGlobalScope())
	if existing := c.scope.Insert(sym); existing != nil {
		msg := fmt.Sprintf("redeclaration of '%s', previously declared at %s", name.Literal, existing.Pos())
		c.error(name, msg)
	}
	return sym
}

func (c *checker) resolve(name token.Token) *Symbol {
	if existing, _ := c.scope.Lookup(name.Literal); existing == nil {
		c.error(name, "'%s' undefined", name.Literal)
	} else {
		return existing
	}
	return nil
}

func (c *checker) isGlobalScope() bool {
	return c.scope.Outer == nil
}

func hasDependency(sym *Symbol, dep *Symbol) bool {
	if sym == dep {
		return true
	}
	for _, v := range sym.Dependencies {
		if hasDependency(v, dep) {
			return true
		}
	}
	return false
}

func (c *checker) checkModule(mod *Module) {
	c.openScope()

	for _, decl := range mod.Decls {
		switch t := decl.(type) {
		case *VarDecl:
			c.declare(VarSymbol, t.Name.Name, t)
		case *FuncDecl:
			c.declare(FuncSymbol, t.Name.Name, t)
		}
	}

	for _, decl := range mod.Decls {
		c.checkDecl(decl)
	}

	if len(c.errors) > 0 {
		return
	}

	// Fix dependency order

	var globals []*Symbol
	var decls []Decl

	for _, decl := range mod.Decls {
		switch t := decl.(type) {
		case *VarDecl:
			sym := c.resolve(t.Name.Name)
			globals = append(globals, sym)
		case *FuncDecl:
			decls = append(decls, decl)
		}
	}

	var sortedGlobals []*Symbol
	for _, g1 := range globals {
		sortedGlobals = sortDependencies(g1, sortedGlobals)
	}

	for _, sym := range sortedGlobals {
		switch t := sym.Decl.(type) {
		case *VarDecl:
			decls = append(decls, t)
		}
	}

	mod.Decls = decls
	mod.Scope = c.scope
	c.closeScope()
}

func contains(sym *Symbol, globals []*Symbol) bool {
	for _, g := range globals {
		if sym == g {
			return true
		}
	}
	return false
}

func sortDependencies(sym *Symbol, globals []*Symbol) []*Symbol {
	for _, dep := range sym.Dependencies {
		globals = sortDependencies(dep, globals)
	}
	if !contains(sym, globals) {
		globals = append(globals, sym)
	}
	return globals
}

func (c *checker) checkDecl(decl Decl) {
	switch t := decl.(type) {
	case *VarDecl:
		c.checkVarDecl(t)
	case *FuncDecl:
		c.checkFuncDecl(t)
	}
}

func (c *checker) checkVarDecl(decl *VarDecl) {
	if c.isGlobalScope() {
		c.currGlobal = c.resolve(decl.Name.Name)
	} else {
		c.declare(VarSymbol, decl.Name.Name, decl)
	}

	if decl.X != nil {
		c.checkExpr(decl.X)
	}
	c.currGlobal = nil
}

func (c *checker) checkFuncDecl(decl *FuncDecl) {
	if c.isGlobalScope() {
		c.currGlobal = c.resolve(decl.Name.Name)
	} else {
		c.declare(FuncSymbol, decl.Name.Name, decl)
	}

	c.openScope()

	for _, field := range decl.Fields {
		c.declare(VarSymbol, field.Name, field)
	}
	c.checkBlockStmt(false, decl.Body)

	decl.Scope = c.scope
	c.closeScope()
	c.currGlobal = nil
}

func (c *checker) checkStmt(stmt Stmt) {
	switch t := stmt.(type) {
	case *BlockStmt:
		c.checkBlockStmt(true, t)
	case *DeclStmt:
		c.checkDecl(t.D)
	case *PrintStmt:
		c.checkExpr(t.X)
	case *AssignStmt:
		c.checkAssignStmt(t)
	case *ExprStmt:
		c.checkExpr(t.X)
	case *IfStmt:
		c.checkIfStmt(t)
	case *WhileStmt:
		c.checkWhileStmt(t)
	case *ReturnStmt:
		if t.X != nil {
			c.checkExpr(t.X)
		}
	}
}

func (c *checker) checkBlockStmt(newScope bool, stmt *BlockStmt) {
	if newScope {
		c.openScope()
	}
	for _, stmt := range stmt.Stmts {
		c.checkStmt(stmt)
	}
	stmt.Scope = c.scope
	if newScope {
		c.closeScope()
	}
}

func (c *checker) checkAssignStmt(stmt *AssignStmt) {
	c.checkIdent(stmt.Name)
	c.checkExpr(stmt.Right)
}

func (c *checker) checkIfStmt(stmt *IfStmt) {
	c.checkExpr(stmt.Cond)
	c.checkBlockStmt(true, stmt.Body)
	if stmt.Else != nil {
		c.checkStmt(stmt.Else)
	}
}

func (c *checker) checkWhileStmt(stmt *WhileStmt) {
	c.checkExpr(stmt.Cond)
	c.checkBlockStmt(true, stmt.Body)
}

func (c *checker) checkExpr(expr Expr) {
	switch t := expr.(type) {
	case *BinaryExpr:
		c.checkBinaryExpr(t)
	case *UnaryExpr:
		c.checkUnaryExpr(t)
	case *Literal:
		c.checkLiteral(t)
	case *Ident:
		c.checkIdent(t)
	case *CallExpr:
		c.checkCallExpr(t)
	}
}

func (c *checker) checkBinaryExpr(expr *BinaryExpr) {
	c.checkExpr(expr.Left)
	c.checkExpr(expr.Right)
}

func (c *checker) checkUnaryExpr(expr *UnaryExpr) {
	c.checkExpr(expr.X)
}

func (c *checker) checkLiteral(lit *Literal) {

}

func (c *checker) checkIdent(id *Ident) {
	sym := c.resolve(id.Name)
	if sym != nil && c.currGlobal != nil {
		if sym.Global && sym.ID == VarSymbol && hasDependency(sym, c.currGlobal) {
			c.error(id.Name, "recursive declaration: '%s' previously declared at %s has an implicit or explicit dependency on '%s'",
				sym.Name.Literal, sym.Name.Pos, c.currGlobal.Name.Literal)
		} else if c.currGlobal.ID == VarSymbol {
			c.currGlobal.Dependencies = append(c.currGlobal.Dependencies, sym)
		}
	}
}

func (c *checker) checkCallExpr(call *CallExpr) {
	sym, _ := c.scope.Lookup(call.Name.Literal())
	if sym == nil {
		c.error(call.Name.Name, "'%s' undefined", call.Name.Literal())
	} else if sym.ID != FuncSymbol {
		c.error(call.Name.Name, "'%s' is not a function", sym.Name.Literal)
	}

	if sym != nil {
		decl, _ := sym.Decl.(*FuncDecl)
		if len(decl.Fields) != len(call.Args) {
			c.error(call.Name.Name, "'%s' takes %d argument(s), but called with %d", sym.Name.Literal, len(decl.Fields), len(call.Args))
		}
	}

	c.checkIdent(call.Name)

	for _, arg := range call.Args {
		c.checkExpr(arg)
	}
}
