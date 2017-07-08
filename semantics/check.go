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
	sym.Name = token.Synthetic(token.Ident, t.String())
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
	c := newTypeChecker(prog)

	buildSymbolTableTree(c)
	if c.errors.IsFatal() {
		return c.errors
	}

	typeCheck(c)
	if c.errors.IsFatal() {
		return c.errors
	}

	return nil
}

type typeChecker struct {
	prog   *Program
	errors common.ErrorList

	// State that changes when visiting nodes
	scope *Scope
	mod   *Module
	file  *File
	fun   *FuncDecl
}

func newTypeChecker(prog *Program) *typeChecker {
	return &typeChecker{prog: prog, scope: builtinScope}
}

func (c *typeChecker) openScope() {
	c.scope = NewScope(c.scope)
}

func (c *typeChecker) closeScope() {
	c.scope = c.scope.Outer
}

func (c *typeChecker) error(tok token.Token, format string, args ...interface{}) {
	filename := ""
	if c.file != nil {
		filename = c.file.Path
	}
	c.errors.Add(filename, tok.Pos, format, args...)
}

func (c *typeChecker) errorPos(pos token.Position, format string, args ...interface{}) {
	filename := ""
	if c.file != nil {
		filename = c.file.Path
	}
	c.errors.Add(filename, pos, format, args...)
}

func (c *typeChecker) insert(scope *Scope, id SymbolID, name token.Token, node Node) *Symbol {
	sym := NewSymbol(id, name, node, false)
	if existing := scope.Insert(sym); existing != nil {
		msg := fmt.Sprintf("redeclaration of '%s', previously declared at %s", name.Literal, existing.Pos())
		c.error(name, msg)
	}
	return sym
}

func (c *typeChecker) lookup(name token.Token) *Symbol {
	if existing := c.scope.Lookup(name.Literal); existing == nil {
		c.error(name, "'%s' undefined", name.Literal)
	} else {
		return existing
	}
	return nil
}

func (c *typeChecker) typeOf(spec token.Token) *TType {
	sym := c.scope.Lookup(spec.Literal)
	if sym == nil || sym.ID != TypeSymbol {
		c.error(spec, "%s is not a type", spec.Literal)
		return TBuiltinUntyped
	}
	return sym.T
}

func (c *typeChecker) isProgramScope() bool {
	if c.mod == nil {
		return false
	}
	return c.scope == c.mod.External
}

func (c *typeChecker) isModuleScope() bool {
	if c.mod == nil {
		return false
	}
	return c.scope == c.mod.Internal
}

func (c *typeChecker) isFileScope() bool {
	if c.file == nil {
		return false
	}
	return c.scope == c.file.Scope
}

// Returns false if error
func (c *typeChecker) tryCastLiteral(expr Expr, target *TType) bool {
	if target.IsNumericType() && expr.Type().IsNumericType() {
		lit, _ := expr.(*Literal)
		if lit != nil {
			castResult := typeCastNumericLiteral(lit, target)

			if castResult == numericCastOK {
				return true
			}

			if castResult == numericCastOverflows {
				c.error(lit.Value, "constant expression %s overflows %s", lit.Value.Literal, target.ID)
			} else if castResult == numericCastTruncated {
				c.error(lit.Value, "type mismatch: constant float expression %s not compatible with %s", lit.Value.Literal, target.ID)
			} else {
				panic(fmt.Sprintf("Unhandled numeric cast result %d", castResult))
			}

			return false
		}
	}
	return true
}
