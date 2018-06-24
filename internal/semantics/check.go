package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

// Check will resolve identifiers, look for cyclic dependencies between identifiers, and type check.
func Check(set *ir.ModuleSet, target ir.Target) error {
	c := newContext(set, target)

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
	exprModeDot  = 3
)

var builtinScope = ir.NewScope(ir.RootScope, "-", nil)

func addBuiltinType(t ir.Type) {
	addBuiltinAliasType(t.ID().String(), t)
}

func addBuiltinAliasType(name string, t ir.Type) {
	sym := ir.NewSymbol(ir.TypeSymbol, builtinScope, true, ir.DGABI, name, token.NoPosition)
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

	// TODO: Change to distinct types
	addBuiltinAliasType("c_void", ir.TBuiltinVoid)
	addBuiltinAliasType("c_char", ir.TBuiltinInt8)
	addBuiltinAliasType("c_uchar", ir.TBuiltinUInt8)
	addBuiltinAliasType("c_short", ir.TBuiltinInt16)
	addBuiltinAliasType("c_ushort", ir.TBuiltinUInt16)
	addBuiltinAliasType("c_int", ir.TBuiltinInt32)
	addBuiltinAliasType("c_uint", ir.TBuiltinUInt32)
	addBuiltinAliasType("c_longlong", ir.TBuiltinInt64)
	addBuiltinAliasType("c_ulonglong", ir.TBuiltinUInt64)
	addBuiltinAliasType("c_usize", ir.TBuiltinUInt64)
	addBuiltinAliasType("c_float", ir.TBuiltinFloat32)
	addBuiltinAliasType("c_double", ir.TBuiltinFloat64)
}

type context struct {
	set        *ir.ModuleSet
	target     ir.Target
	errors     *common.ErrorList
	decls      map[*ir.Symbol]ir.TopDecl
	constExprs map[*ir.Symbol]ir.Expr

	// State that can change during node visits
	scope     *ir.Scope
	declTrace []ir.TopDecl
}

func newContext(set *ir.ModuleSet, target ir.Target) *context {
	c := &context{set: set, target: target, scope: builtinScope}
	c.errors = &common.ErrorList{}
	c.decls = make(map[*ir.Symbol]ir.TopDecl)
	c.constExprs = make(map[*ir.Symbol]ir.Expr)
	return c
}

func (c *context) resetWalkState() {
	c.declTrace = nil
}

func (c *context) openScope(id ir.ScopeID, fqn string) {
	c.scope = ir.NewScope(id, fqn, c.scope)
}

func (c *context) closeScope() {
	c.scope = c.scope.Parent
}

func setScope(c *context, scope *ir.Scope) (*context, *ir.Scope) {
	curr := c.scope
	c.scope = scope
	return c, curr
}

func (c *context) addTopDeclSymbol(decl ir.TopDecl, name *ir.Ident, id ir.SymbolID, abi *ir.Ident) *ir.Symbol {
	public := decl.Visibility().Is(token.Public)
	effectiveABI := ir.DGABI
	if abi != nil {
		effectiveABI = abi.Literal
	}
	sym := c.insert(c.scope, id, public, effectiveABI, name.Literal, name.Pos())
	if sym != nil {
		c.decls[sym] = decl
	}
	return sym
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

func (c *context) error(pos token.Position, format string, args ...interface{}) {
	c.errors.Add(pos, format, args...)
}

func (c *context) errorNode(node ir.Node, format string, args ...interface{}) {
	pos := node.Pos()
	endPos := node.EndPos()
	c.errors.AddRange(pos, endPos, format, args...)
}

func (c *context) warning(pos token.Position, format string, args ...interface{}) {
	c.errors.AddWarning(pos, format, args...)
}

func (c *context) insert(scope *ir.Scope, id ir.SymbolID, public bool, abi string, name string, pos token.Position) *ir.Symbol {
	sym := ir.NewSymbol(id, scope, public, abi, name, pos)
	if existing := scope.Insert(sym); existing != nil {
		msg := fmt.Sprintf("redefinition of '%s' (previously defined at %s)", name, existing.DefPos)
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
