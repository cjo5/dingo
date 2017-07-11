package semantics

import (
	"fmt"

	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/token"
)

var builtinScope = NewScope(nil)

func addBuiltinType(t *TType) {
	sym := &Symbol{}
	sym.ID = TypeSymbol
	sym.T = t
	sym.Name = t.String()
	sym.Pos = token.NoPosition
	builtinScope.Insert(sym)
}

func init() {
	addBuiltinType(TBuiltinVoid)
	addBuiltinType(TBuiltinBool)
	addBuiltinType(TBuiltinString)
	addBuiltinType(TBuiltinModule)
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
	scope *Scope
	mod   *Module
	file  *FileInfo
	fun   *FuncDecl

	declSym *Symbol
}

func newChecker(prog *Program) *checker {
	c := &checker{prog: prog, scope: builtinScope}
	c.errors = &common.ErrorList{}
	return c
}

func (c *checker) resetWalkState() {
	c.mod = nil
	c.file = nil
	c.fun = nil
	c.declSym = nil
}

func (c *checker) openScope() {
	c.scope = NewScope(c.scope)
}

func (c *checker) closeScope() {
	c.scope = c.scope.Outer
}

func setScope(c *checker, scope *Scope) (*checker, *Scope) {
	curr := c.scope
	c.scope = scope
	return c, curr
}

func (c *checker) error(pos token.Position, format string, args ...interface{}) {
	filename := ""
	if c.file != nil {
		filename = c.file.Path
	}
	c.errors.Add(filename, pos, format, args...)
}

func (c *checker) insert(scope *Scope, id SymbolID, name string, pos token.Position, decl Decl) *Symbol {
	sym := NewSymbol(id, name, pos, c.file, decl, c.isToplevel())
	if existing := scope.Insert(sym); existing != nil {
		msg := fmt.Sprintf("redeclaration of '%s', previously declared at %s", name, existing.Pos)
		c.error(pos, msg)
		return nil
	}
	return sym
}

func (c *checker) lookup(name token.Token) *Symbol {
	if existing := c.scope.Lookup(name.Literal); existing == nil {
		c.error(name.Pos, "'%s' undefined", name.Literal)
	} else {
		return existing
	}
	return nil
}

func (c *checker) typeOf(spec token.Token) *TType {
	sym := c.scope.Lookup(spec.Literal)
	if sym == nil || sym.ID != TypeSymbol {
		c.error(spec.Pos, "%s is not a type", spec.Literal)
		return TBuiltinUntyped
	}
	return sym.T
}

func (c *checker) isToplevel() bool {
	if c.mod != nil {
		if c.scope == c.mod.External || c.scope == c.mod.Internal {
			return true
		}
	}
	if c.file != nil {
		if c.scope == c.file.Scope {
			return true
		}
	}
	return false
}

func (c *checker) sortModules() {
	for _, mod := range c.prog.Modules {
		mod.Color = GraphColorWhite
	}

	var sortedModules []*Module

	for _, mod := range c.prog.Modules {
		if mod.Color == GraphColorWhite {
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
	if mod.Color == GraphColorBlack {
		return true
	} else if mod.Color == GraphColorGray {
		return false
	}

	mod.Color = GraphColorGray
	for _, file := range mod.Files {
		for _, imp := range file.Info.Imports {
			if !sortModuleDependencies(imp.Mod, sortedModules) {
				return false
			}
		}
	}
	mod.Color = GraphColorBlack
	*sortedModules = append(*sortedModules, mod)
	return true
}

func (c *checker) sortDecls() {
	for _, mod := range c.prog.Modules {
		for _, file := range mod.Files {
			for _, decl := range file.Decls {
				sym := decl.Symbol()
				if sym != nil {
					sym.color = GraphColorWhite
				}
			}
		}
	}

	for _, mod := range c.prog.Modules {
		var sortedSymbols []*Symbol
		for _, file := range mod.Files {
			for _, decl := range file.Decls {
				sym := decl.Symbol()
				if sym == nil {
					continue
				}

				var cycleTrace []*Symbol
				if !sortSymbolDependencies(sym, &cycleTrace, &sortedSymbols) {
					// Report most specific cycle
					i, j := 0, len(cycleTrace)-1
					for ; i < len(cycleTrace) && j >= 0; i, j = i+1, j-1 {
						if cycleTrace[i] == cycleTrace[j] {
							break
						}
					}

					if i < j {
						sym = cycleTrace[j]
						cycleTrace = cycleTrace[i:j]
					}

					sym.Flags |= SymFlagDepCycle
					for _, t := range cycleTrace {
						t.Flags |= SymFlagDepCycle
					}

					trace := common.NewTrace(fmt.Sprintf("%s uses:", sym.Name), nil)
					for i := len(cycleTrace) - 1; i >= 0; i-- {
						line := cycleTrace[i].File.Path + ":" + cycleTrace[i].Name
						trace.Lines = append(trace.Lines, line)
					}
					c.errors.AddTrace(sym.File.Path, sym.Pos, trace, "initializer cycle detected")

				}
			}
		}

		if len(sortedSymbols) == 0 {
			continue
		}

		var files []*FileDecls
		currFile := &FileDecls{Info: sortedSymbols[0].File}
		for _, sym := range sortedSymbols {
			if sym.File != currFile.Info {
				files = append(files, currFile)
				currFile = &FileDecls{Info: sym.File}
			}
			currFile.Decls = append(currFile.Decls, sym.Src)
		}
		files = append(files, currFile)
		mod.Files = files
	}
}

// Returns false if cycle
func sortSymbolDependencies(sym *Symbol, trace *[]*Symbol, sortedSymbols *[]*Symbol) bool {
	if sym.color == GraphColorBlack {
		return true
	} else if sym.color == GraphColorGray {
		return false
	}

	sortOK := true
	sym.color = GraphColorGray
	for _, dep := range sym.dependencies {
		if !sortSymbolDependencies(dep, trace, sortedSymbols) {
			*trace = append(*trace, dep)
			sortOK = false
			break
		}
	}
	sym.color = GraphColorBlack
	*sortedSymbols = append(*sortedSymbols, sym)
	return sortOK
}

// Returns false if error
func (c *checker) tryCastLiteral(expr Expr, target *TType) bool {
	if target.IsNumericType() && expr.Type().IsNumericType() {
		lit, _ := expr.(*Literal)
		if lit != nil {
			castResult := typeCastNumericLiteral(lit, target)

			if castResult == numericCastOK {
				return true
			}

			if castResult == numericCastOverflows {
				c.error(lit.Value.Pos, "constant expression %s overflows %s", lit.Value.Literal, target.ID)
			} else if castResult == numericCastTruncated {
				c.error(lit.Value.Pos, "type mismatch: constant float expression %s not compatible with %s", lit.Value.Literal, target.ID)
			} else {
				panic(fmt.Sprintf("Unhandled numeric cast result %d", castResult))
			}

			return false
		}
	}
	return true
}
