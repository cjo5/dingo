package semantics

import (
	"fmt"

	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/token"
)

var builtinScope = NewScope(RootScope, nil)

func addBuiltinType(t Type) {
	sym := &Symbol{}
	sym.ID = TypeSymbol
	sym.T = t
	sym.Flags = SymFlagCastable
	sym.Name = t.ID().String()
	sym.Pos = token.NoPosition
	builtinScope.Insert(sym)
}

func init() {
	addBuiltinType(TBuiltinVoid)
	addBuiltinType(TBuiltinBool)
	addBuiltinType(TBuiltinString)
	addBuiltinType(TBuiltinUInt64)
	addBuiltinType(TBuiltinInt64)
	addBuiltinType(TBuiltinUInt32)
	addBuiltinType(TBuiltinInt32)
	addBuiltinType(TBuiltinUInt16)
	addBuiltinType(TBuiltinInt16)
	addBuiltinType(TBuiltinUInt8)
	addBuiltinType(TBuiltinInt8)
	addBuiltinType(TBuiltinFloat64)
	addBuiltinType(TBuiltinFloat32)
}

// Check program.
// Resolve identifiers, type check and look for cyclic dependencies between identifiers.
//
// Module dependencies are resolved before Check is invoked.
// Module[0] has no dependencies.
// Module[1] has no dependencies, or only depends on Module[0].
// Module[2] has no dependencies, or depends on one of, or both, Module[0] and Module[1].
// And so on...
//
func Check(prog *Program) error {
	c := newChecker(prog)

	c.sortModules()
	symbolWalk(c)
	dependencyWalk(c)
	c.sortDecls()
	typeWalk(c)

	if c.errors.IsFatal() {
		return c.errors
	}

	return nil
}

type checker struct {
	prog   *Program
	errors *common.ErrorList

	// State that changes when visiting nodes
	scope   *Scope
	mod     *Module
	fileCtx *FileContext
	topDecl TopDecl
}

func newChecker(prog *Program) *checker {
	c := &checker{prog: prog, scope: builtinScope}
	c.errors = &common.ErrorList{}
	return c
}

func (c *checker) resetWalkState() {
	c.mod = nil
	c.fileCtx = nil
	c.topDecl = nil
}

func (c *checker) openScope(id ScopeID) {
	c.scope = NewScope(id, c.scope)
}

func (c *checker) closeScope() {
	c.scope = c.scope.Outer
}

func setScope(c *checker, scope *Scope) (*checker, *Scope) {
	curr := c.scope
	c.scope = scope
	return c, curr
}

func (c *checker) visibilityScope(tok token.Token) *Scope {
	var scope *Scope
	if tok.Is(token.Public) {
		scope = c.mod.Public
	} else if tok.Is(token.Internal) {
		scope = c.mod.Internal
	} else if tok.Is(token.Private) {
		scope = c.fileScope()
	} else {
		panic(fmt.Sprintf("Unhandled visibility %s", tok))
	}
	return scope
}

func (c *checker) fileScope() *Scope {
	if c.fileCtx != nil {
		return c.fileCtx.Scope
	}
	return nil
}

func (c *checker) setTopDecl(decl TopDecl) {
	c.topDecl = decl
	c.fileCtx = decl.Context()
	c.scope = c.fileCtx.Scope
}

func (c *checker) error(pos token.Position, format string, args ...interface{}) {
	filename := ""
	if c.fileCtx != nil {
		filename = c.fileCtx.Path
	}
	c.errors.Add(filename, pos, format, args...)
}

func (c *checker) insert(scope *Scope, id SymbolID, name string, pos token.Position, src Decl) *Symbol {
	sym := NewSymbol(id, scope.ID, c.mod.ID, name, pos, src)
	if existing := scope.Insert(sym); existing != nil {
		msg := fmt.Sprintf("redeclaration of '%s', previously declared at %s", name, existing.Pos)
		c.error(pos, msg)
		return nil
	}
	return sym
}

func (c *checker) lookup(name string) *Symbol {
	if existing := c.scope.Lookup(name); existing != nil {
		return existing
	}
	return nil
}

func (c *checker) sortModules() {
	for _, mod := range c.prog.Modules {
		mod.color = NodeColorWhite
	}

	var sortedModules []*Module

	for _, mod := range c.prog.Modules {
		if mod.color == NodeColorWhite {
			if !sortModuleDependencies(mod, &sortedModules) {
				// This shouldn't actually happen since cycles are checked when loading imports
				panic("Cycle detected")
			}
		}
	}

	c.prog.Modules = sortedModules
}

// Returns false if cycle
func sortModuleDependencies(mod *Module, sortedModules *[]*Module) bool {
	if mod.color == NodeColorBlack {
		return true
	} else if mod.color == NodeColorGray {
		return false
	}
	mod.color = NodeColorGray
	for _, file := range mod.Files {
		for _, imp := range file.Imports {
			if !sortModuleDependencies(imp.Mod, sortedModules) {
				return false
			}
		}
	}
	mod.color = NodeColorBlack
	*sortedModules = append(*sortedModules, mod)
	return true
}

func (c *checker) sortDecls() {
	for _, mod := range c.prog.Modules {
		for _, decl := range mod.Decls {
			decl.setNodeColor(NodeColorWhite)
		}
	}

	for _, mod := range c.prog.Modules {
		var sortedDecls []TopDecl
		for _, decl := range mod.Decls {
			sym := decl.Symbol()
			if sym == nil {
				continue
			}

			var cycleTrace []TopDecl
			if !sortDeclDependencies(decl, &cycleTrace, &sortedDecls) {
				// Report most specific cycle
				i, j := 0, len(cycleTrace)-1
				for ; i < len(cycleTrace) && j >= 0; i, j = i+1, j-1 {
					if cycleTrace[i] == cycleTrace[j] {
						break
					}
				}

				if i < j {
					decl = cycleTrace[j]
					cycleTrace = cycleTrace[i:j]
				}

				sym.Flags |= SymFlagDepCycle

				trace := common.NewTrace(fmt.Sprintf("%s uses:", sym.Name), nil)
				for i := len(cycleTrace) - 1; i >= 0; i-- {
					s := cycleTrace[i].Symbol()
					s.Flags |= SymFlagDepCycle
					line := cycleTrace[i].Context().Path + ":" + s.Name
					trace.Lines = append(trace.Lines, line)
				}
				c.errors.AddTrace(decl.Context().Path, sym.Pos, trace, "initializer cycle detected")
			}
		}
		mod.Decls = sortedDecls
	}
}

// Returns false if cycle
func sortDeclDependencies(decl TopDecl, trace *[]TopDecl, sortedDecls *[]TopDecl) bool {
	color := decl.nodeColor()
	if color == NodeColorBlack {
		return true
	} else if color == NodeColorGray {
		return false
	}

	sortOK := true
	decl.setNodeColor(NodeColorGray)
	for _, dep := range decl.dependencies() {
		if !sortDeclDependencies(dep, trace, sortedDecls) {
			*trace = append(*trace, dep)
			sortOK = false
			break
		}
	}
	decl.setNodeColor(NodeColorBlack)
	*sortedDecls = append(*sortedDecls, decl)
	return sortOK
}

// Returns false if error
func (c *checker) tryCastLiteral(expr Expr, target Type) bool {
	if IsNumericType(expr.Type()) && IsNumericType(target) {
		lit, _ := expr.(*BasicLit)
		if lit != nil {
			castResult := typeCastNumericLiteral(lit, target)

			if castResult == numericCastOK {
				return true
			}

			if castResult == numericCastOverflows {
				c.error(lit.Value.Pos, "constant expression %s overflows %s", lit.Value.Literal, target)
			} else if castResult == numericCastTruncated {
				c.error(lit.Value.Pos, "type mismatch: constant float expression %s not compatible with %s", lit.Value.Literal, target)
			} else {
				panic(fmt.Sprintf("Unhandled numeric cast result %d", castResult))
			}

			return false
		}
	}
	return true
}

// Returns false if error
func (c *checker) tryCoerceBigNumber(expr Expr) bool {
	t := expr.Type()
	if t.ID() == TBigInt {
		return c.tryCastLiteral(expr, TBuiltinInt32)
	} else if t.ID() == TBigFloat {
		return c.tryCastLiteral(expr, TBuiltinFloat64)
	}
	return true
}

// Returns Ident that was declared as const. Nil otherwise.
func (c *checker) checkConstant(expr Expr) *Ident {
	return checkConstantRecursively(expr)
}

func checkConstantRecursively(expr Expr) *Ident {
	switch t := expr.(type) {
	case *Ident:
		if t.Sym != nil && t.Sym.Constant() {
			return t
		}
	case *DotIdent:
		if t.Name.Sym != nil {
			if t.Name.Sym.Constant() {
				return t.Name
			}
		} else {
			return nil
		}
		return checkConstantRecursively(t.X)
	}
	return nil
}
