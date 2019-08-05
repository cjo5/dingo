package semantics

import (
	"sort"

	"github.com/cjo5/dingo/internal/common"
	"github.com/cjo5/dingo/internal/ir"
	"github.com/cjo5/dingo/internal/token"
)

// Check semantics.
func Check(ctx *common.BuildContext, target ir.Target, fileMatrix ir.FileMatrix) (ir.DeclMatrix, bool) {
	ctx.SetCheckpoint()
	c := newChecker(ctx, target)
	c.initBuiltinScope()
	modMatrix := c.createModuleMatrix(fileMatrix)
	c.initObjectMatrix(modMatrix)
	c.checkTypes()
	declMatrix := c.createDeclMatrix()
	return declMatrix, !ctx.IsErrorSinceCheckpoint()
}

const (
	modeExpr int = iota
	modeIndirectExpr
	modeType
	modeIndirectType
	modeBoth
)

type checker struct {
	ctx    *common.BuildContext
	target ir.Target

	builtinScope  *ir.Scope
	objectMatrix  []*objectList
	currentSymKey ir.SymbolKey

	importMap  map[string]*ir.Symbol
	constMap   map[ir.SymbolKey]ir.Expr
	objectMap  map[ir.SymbolKey]*object
	incomplete map[ir.SymbolKey]*object

	// Ast traversal state
	objectList *objectList
	object     *object
	scope      *ir.Scope
	mode       int
	step       int
	loop       int
}

func stmtList(stmts []ir.Stmt, visit func(ir.Stmt)) {
	for _, stmt := range stmts {
		visit(stmt)
	}
}

func newChecker(ctx *common.BuildContext, target ir.Target) *checker {
	return &checker{
		ctx:           ctx,
		target:        target,
		currentSymKey: 1,
		importMap:     make(map[string]*ir.Symbol),
		constMap:      make(map[ir.SymbolKey]ir.Expr),
		objectMap:     make(map[ir.SymbolKey]*object),
		incomplete:    make(map[ir.SymbolKey]*object),
	}
}

func (c *checker) sortedIncompleteKeys() []ir.SymbolKey {
	var keys []ir.SymbolKey
	for key := range c.incomplete {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}

func (c *checker) nextSymKey() ir.SymbolKey {
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

func (c *checker) openScope(name string) {
	c.scope = ir.NewScope(name, c.scope, c.scope.CUID)
}

func (c *checker) closeScope() {
	c.scope = c.scope.Parent
}

func (c *checker) error(pos token.Position, format string, args ...interface{}) {
	c.ctx.Errors.Add(pos, format, args...)
}

func (c *checker) nodeError(node ir.Node, format string, args ...interface{}) {
	pos := node.Pos()
	endPos := node.EndPos()
	c.ctx.Errors.AddRange(pos, endPos, format, args...)
}

func (c *checker) warning(pos token.Position, format string, args ...interface{}) {
	c.ctx.Errors.AddWarning(pos, format, args...)
}

func (c *checker) lookup(name string) *ir.Symbol {
	if existing := c.scope.Lookup(name); existing != nil {
		return existing
	}
	return nil
}

func (c *checker) setIncompleteObject() {
	c.object.incomplete = true
	c.incomplete[c.object.sym().UniqKey] = c.object
}

func (c *checker) isWeakDep(sym *ir.Symbol) bool {
	isWeak := false
	switch c.object.sym().Kind {
	case ir.ModuleSymbol:
		isWeak = true
	case ir.FuncSymbol:
		if sym.Kind == ir.FuncSymbol {
			isWeak = true
		}
	default:
		if c.mode == modeIndirectType ||
			c.mode == modeIndirectExpr ||
			sym.Kind == ir.FuncSymbol {
			isWeak = true
		}
	}
	return isWeak
}

func (c *checker) trySetDep(sym *ir.Symbol, checkWeak bool) {
	if sym.Kind != ir.ModuleSymbol && sym.UniqKey > 0 {
		if obj, ok := c.objectMap[sym.UniqKey]; ok {
			isWeak := false
			if checkWeak {
				isWeak = c.isWeakDep(sym)
			}
			c.object.setDep(obj, isWeak)
			if !obj.checked || obj.incomplete {
				c.setIncompleteObject()
			}
		}
	}
}
