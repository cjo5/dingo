package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

const (
	exprModeNone = 0
	exprModeType = 1
	exprModeFunc = 2
)

var builtinScope = ir.NewScope(ir.RootScope, nil)

func addBuiltinType(t ir.Type) {
	sym := &ir.Symbol{}
	sym.ID = ir.TypeSymbol
	sym.T = t
	sym.Name = t.ID().String()
	sym.Pos = token.NoPosition
	builtinScope.Insert(sym)
}

func init() {
	addBuiltinType(ir.TBuiltinVoid)
	addBuiltinType(ir.TBuiltinBool)
	addBuiltinType(ir.TBuiltinUInt64)
	addBuiltinType(ir.TBuiltinInt64)
	addBuiltinType(ir.TBuiltinUInt32)
	addBuiltinType(ir.TBuiltinInt32)
	addBuiltinType(ir.TBuiltinUInt16)
	addBuiltinType(ir.TBuiltinInt16)
	addBuiltinType(ir.TBuiltinUInt8)
	addBuiltinType(ir.TBuiltinInt8)
	addBuiltinType(ir.TBuiltinFloat64)
	addBuiltinType(ir.TBuiltinFloat32)
}

type context struct {
	set      *ir.ModuleSet
	errors   *common.ErrorList
	topDecls map[*ir.Symbol]ir.TopDecl

	// State that can change during node visits
	scope   *ir.Scope
	mod     *ir.Module
	fileCtx *ir.FileContext
	topDecl ir.TopDecl
}

func newContext(set *ir.ModuleSet) *context {
	c := &context{set: set, scope: builtinScope}
	c.errors = &common.ErrorList{}
	c.topDecls = make(map[*ir.Symbol]ir.TopDecl)
	return c
}

func (c *context) resetWalkState() {
	c.mod = nil
	c.fileCtx = nil
	c.topDecl = nil
}

func (c *context) openScope(id ir.ScopeID) {
	c.scope = ir.NewScope(id, c.scope)
}

func (c *context) closeScope() {
	c.scope = c.scope.Outer
}

func setScope(c *context, scope *ir.Scope) (*context, *ir.Scope) {
	curr := c.scope
	c.scope = scope
	return c, curr
}

func (c *context) visibilityScope(tok token.Token) *ir.Scope {
	return c.mod.Scope
}

func (c *context) setCurrentTopDecl(decl ir.TopDecl) {
	c.topDecl = decl
	c.fileCtx = decl.Context()
}

func (c *context) mapTopDecl(sym *ir.Symbol, decl ir.TopDecl) {
	if sym != nil {
		c.topDecls[sym] = decl
	}
}

func (c *context) error(pos token.Position, format string, args ...interface{}) {
	filename := ""
	if c.fileCtx != nil {
		filename = c.fileCtx.Path
	}
	c.errors.Add(filename, pos, common.GenericError, format, args...)
}

func (c *context) insert(scope *ir.Scope, id ir.SymbolID, name string, pos token.Position) *ir.Symbol {
	sym := ir.NewSymbol(id, scope.ID, name, pos)
	if existing := scope.Insert(sym); existing != nil {
		msg := fmt.Sprintf("redeclaration of '%s', previously declared at %s", name, existing.Pos)
		c.error(pos, msg)
		return nil
	}
	return sym
}

func (c *context) lookup(name string) *ir.Symbol {
	if existing := c.scope.Lookup(name); existing != nil {
		return existing
	}
	return nil
}
