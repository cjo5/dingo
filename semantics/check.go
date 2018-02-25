package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

// Check will resolve identifiers, look for cyclic dependencies between identifiers, and type check.
func Check(set *ir.ModuleSet) error {
	c := newContext(set)

	symCheck(c)
	depCheck(c)
	typeCheck(c)
	sortDecls(c)

	return c.errors
}

const (
	exprModeNone = 0
	exprModeType = 1
	exprModeFunc = 2
)

var builtinScope = ir.NewScope(ir.RootScope, nil)

func addBuiltinType(t ir.Type) {
	sym := ir.NewSymbol(ir.TypeSymbol, builtinScope, true, t.ID().String(), ir.NewPosition("", token.NoPosition))
	sym.T = t
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
	set    *ir.ModuleSet
	errors *common.ErrorList
	decls  map[*ir.Symbol]ir.TopDecl

	// State that can change during node visits
	scope     *ir.Scope
	mod       *ir.Module
	declTrace []ir.TopDecl
}

func newContext(set *ir.ModuleSet) *context {
	c := &context{set: set, scope: builtinScope}
	c.errors = &common.ErrorList{}
	c.decls = make(map[*ir.Symbol]ir.TopDecl)
	return c
}

func (c *context) resetWalkState() {
	c.mod = nil
	c.declTrace = nil
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

func (c *context) mapTopDecl(sym *ir.Symbol, decl ir.TopDecl) {
	if sym != nil {
		c.decls[sym] = decl
	}
}

func (c *context) pushTopDecl(decl ir.TopDecl) {
	c.declTrace = append(c.declTrace, decl)
}

func (c *context) popTopDecl() {
	n := len(c.declTrace)
	if n > 0 {
		c.declTrace = c.declTrace[:n-1]
	}
}

func (c *context) topDecl() ir.TopDecl {
	n := len(c.declTrace)
	if n > 0 {
		return c.declTrace[n-1]
	}
	return nil
}

func (c *context) fileCtx() *ir.FileContext {
	n := len(c.declTrace)
	if n > 0 {
		return c.declTrace[n-1].FileContext()
	}
	return nil
}

func (c *context) filename() string {
	ctx := c.fileCtx()
	if ctx != nil {
		return ctx.Path
	}
	return ""
}

func (c *context) newSymPos(pos token.Position) ir.Position {
	return ir.NewPosition(c.filename(), pos)
}

func (c *context) fmtSymPos(pos ir.Position) string {
	if c.filename() != pos.Filename {
		return pos.String()
	}
	return pos.Pos.String()
}

func (c *context) error(pos token.Position, format string, args ...interface{}) {
	c.errors.Add(c.filename(), pos, pos, format, args...)
}

func (c *context) errorInterval(pos token.Position, endPos token.Position, format string, args ...interface{}) {
	c.errors.Add(c.filename(), pos, endPos, format, args...)
}

func (c *context) errorExpr(expr ir.Expr, format string, args ...interface{}) {
	c.errorInterval(expr.Pos(), expr.EndPos(), format, args...)
}

func (c *context) warning(pos token.Position, format string, args ...interface{}) {
	c.errors.AddWarning(c.filename(), pos, pos, format, args...)
}

func (c *context) insert(scope *ir.Scope, id ir.SymbolID, public bool, name string, pos token.Position) *ir.Symbol {
	sym := ir.NewSymbol(id, scope, public, name, c.newSymPos(pos))
	if existing := scope.Insert(sym); existing != nil {
		msg := fmt.Sprintf("redefinition of '%s' (previously defined at %s)", name, c.fmtSymPos(existing.DefPos))
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
