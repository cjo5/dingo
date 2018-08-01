package semantics

import (
	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

var builtinScope *ir.Scope

const builtinSymFlags = ir.SymFlagBuiltin | ir.SymFlagDefined

func addBuiltinType(t ir.Type) {
	addBuiltinAliasType(t.Kind().String(), t)
}

func addBuiltinAliasType(name string, t ir.Type) {
	sym := ir.NewSymbol(ir.TypeSymbol, builtinScope, -1, "", name, token.NoPosition)
	sym.Public = true
	sym.Flags |= builtinSymFlags
	sym.T = t
	builtinScope.Insert(name, sym)
}

func init() {
	builtinScope = ir.NewScope(ir.BuiltinScope, nil, -1)

	addBuiltinType(ir.TBuiltinVoid)
	addBuiltinType(ir.TBuiltinBool)
	addBuiltinType(ir.TBuiltinInt8)
	addBuiltinType(ir.TBuiltinUInt8)
	addBuiltinType(ir.TBuiltinInt16)
	addBuiltinType(ir.TBuiltinUInt16)
	addBuiltinType(ir.TBuiltinInt32)
	addBuiltinType(ir.TBuiltinUInt32)
	addBuiltinType(ir.TBuiltinInt64)
	addBuiltinType(ir.TBuiltinUInt64)
	addBuiltinType(ir.TBuiltinUSize)
	addBuiltinType(ir.TBuiltinFloat32)
	addBuiltinType(ir.TBuiltinFloat64)

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
	addBuiltinAliasType("c_usize", ir.TBuiltinUSize)
	addBuiltinAliasType("c_float", ir.TBuiltinFloat32)
	addBuiltinAliasType("c_double", ir.TBuiltinFloat64)
}

// Check semantics.
func Check(fileMatrix ir.FileMatrix, target ir.Target) (ir.DeclMatrix, error) {
	c := newChecker(target)
	modMatrix := c.createModuleMatrix(fileMatrix)
	c.initDgObjectMatrix(modMatrix)
	c.checkTypes()
	if c.errors.IsError() {
		return nil, c.errors
	}
	declMatrix := c.createDeclMatrix()
	return declMatrix, c.errors
}

const (
	modeCheck int = iota
	modeType
	modeIndirectType
	modeDot
)

type checker struct {
	target ir.Target
	errors *common.ErrorList

	objectMatrix  []*dgObjectList
	currentSymKey int

	objectMap  map[int]*dgObject
	constMap   map[int]ir.Expr
	importMap  map[string]*ir.Symbol
	incomplete map[*dgObject]bool

	// Ast traversal state
	object *dgObject
	scope  *ir.Scope
	mode   int
	step   int
	loop   int
}

func stmtList(stmts []ir.Stmt, visit func(ir.Stmt)) {
	for _, stmt := range stmts {
		visit(stmt)
	}
}

func newChecker(target ir.Target) *checker {
	return &checker{
		target:        target,
		errors:        &common.ErrorList{},
		currentSymKey: 1,
		objectMap:     make(map[int]*dgObject),
		constMap:      make(map[int]ir.Expr),
		importMap:     make(map[string]*ir.Symbol),
		incomplete:    make(map[*dgObject]bool),
	}
}

func (c *checker) nextSymKey() int {
	key := c.currentSymKey
	c.currentSymKey++
	return key
}

func (c *checker) setMode(mode int) int {
	prev := c.mode
	c.mode = mode
	return prev
}

func (c *checker) isTypeMode() bool {
	switch c.mode {
	case modeType, modeIndirectType:
		return true
	}
	return false
}

func (c *checker) setScope(scope *ir.Scope) *ir.Scope {
	prev := c.scope
	c.scope = scope
	return prev
}

func (c *checker) openScope(kind ir.ScopeKind) {
	c.scope = ir.NewScope(kind, c.scope, c.scope.CUID)
}

func (c *checker) closeScope() {
	c.scope = c.scope.Parent
}

func (c *checker) error(pos token.Position, format string, args ...interface{}) {
	c.errors.Add(pos, format, args...)
}

func (c *checker) nodeError(node ir.Node, format string, args ...interface{}) {
	pos := node.Pos()
	endPos := node.EndPos()
	c.errors.AddRange(pos, endPos, format, args...)
}

func (c *checker) warning(pos token.Position, format string, args ...interface{}) {
	c.errors.AddWarning(pos, format, args...)
}

func (c *checker) lookup(name string) *ir.Symbol {
	if existing := c.scope.Lookup(name); existing != nil {
		return existing
	}
	return nil
}

func (c *checker) tryAddDep(sym *ir.Symbol, pos token.Position) {
	if sym.Kind != ir.ModuleSymbol && sym.Key > 0 {
		if obj, ok := c.objectMap[sym.Key]; ok {
			edge := &depEdge{
				pos:            pos,
				isIndirectType: c.mode == modeIndirectType,
			}
			c.object.addEdge(obj, edge)
			if !obj.checked || obj.incomplete {
				c.object.incomplete = true
				c.incomplete[c.object] = true
			}
		}
	}
}
