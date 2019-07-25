package semantics

import (
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
	modeType
	modeIndirectType
	modeExprOrType
)

type checker struct {
	ctx    *common.BuildContext
	target ir.Target

	builtinScope  *ir.Scope
	objectMatrix  []*objectList
	currentSymKey int

	objectMap  map[int]*object
	constMap   map[int]ir.Expr
	importMap  map[string]*ir.Symbol
	incomplete map[*object]bool

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
		objectMap:     make(map[int]*object),
		constMap:      make(map[int]ir.Expr),
		importMap:     make(map[string]*ir.Symbol),
		incomplete:    make(map[*object]bool),
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

func (c *checker) tryAddDep(sym *ir.Symbol, pos token.Position) {
	if sym.Kind != ir.ModuleSymbol && sym.UniqKey > 0 {
		if obj, ok := c.objectMap[sym.UniqKey]; ok {
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
