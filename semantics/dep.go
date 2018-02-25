package semantics

import (
	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

type depChecker struct {
	ir.BaseVisitor
	c        *context
	exprMode int
	declSym  *ir.Symbol
}

func depCheck(c *context) {
	c.resetWalkState()
	v := &depChecker{c: c}
	ir.VisitModuleSet(v, c.set)
}

func sortDecls(c *context) {
	c.set.ResetDeclColors()

	for _, mod := range c.set.Modules {
		var sortedDecls []ir.TopDecl
		for _, decl := range mod.Decls {
			if decl.Symbol() == nil {
				continue
			}

			var cycleTrace []ir.TopDecl
			if !sortDeclDependencies(decl, &cycleTrace, &sortedDecls) {
				cycleTrace = append(cycleTrace, decl)

				// Find most specific cycle
				j := len(cycleTrace) - 1
				for ; j >= 0; j = j - 1 {
					if cycleTrace[0] == cycleTrace[j] {
						break
					}
				}

				cycleTrace = cycleTrace[:j+1]
				sym := cycleTrace[0].Symbol()

				trace := common.NewTrace("", nil)
				for i := len(cycleTrace) - 1; i >= 0; i-- {
					s := cycleTrace[i].Symbol()
					line := c.fmtSymPos(s.DefPos) + ":" + s.Name
					trace.Lines = append(trace.Lines, line)
				}

				c.errors.AddTrace(sym.DefPos.Filename, sym.DefPos.Pos, sym.DefPos.Pos, trace, "cycle detected")
			}
		}
		mod.Decls = sortedDecls
	}
}

// Returns false if cycle
func sortDeclDependencies(decl ir.TopDecl, trace *[]ir.TopDecl, sortedDecls *[]ir.TopDecl) bool {
	color := decl.Color()
	if color == ir.BlackColor {
		return true
	} else if color == ir.GrayColor {
		return false
	}

	sortOK := true
	decl.SetColor(ir.GrayColor)
	graph := *decl.DependencyGraph()

	for dep := range graph {
		if !sortDeclDependencies(dep, trace, sortedDecls) {
			*trace = append(*trace, dep)
			sortOK = false
			break
		}
	}
	decl.SetColor(ir.BlackColor)
	*sortedDecls = append(*sortedDecls, decl)
	return sortOK
}

func checkCycle(decl ir.TopDecl) bool {
	var visited []ir.TopDecl
	return checkCycle2(decl, visited)
}

func checkCycle2(decl ir.TopDecl, visited []ir.TopDecl) bool {
	for _, v := range visited {
		if v == decl {
			return true
		}
	}

	visited = append(visited, decl)
	graph := *decl.DependencyGraph()

	for dep := range graph {
		if checkCycle2(dep, visited) {
			return true
		}
	}

	visited = visited[:len(visited)-1]
	return false
}

func (v *depChecker) Module(mod *ir.Module) {
	v.c.mod = mod
	v.c.scope = mod.Scope
	for _, decl := range mod.Decls {
		v.c.pushTopDecl(decl)
		ir.VisitDecl(v, decl)
		v.c.popTopDecl()
	}
}

func (v *depChecker) VisitValTopDecl(decl *ir.ValTopDecl) {
	v.visitValDeclSpec(decl.Sym, &decl.ValDeclSpec)
}

func (v *depChecker) VisitValDecl(decl *ir.ValDecl) {
	v.visitValDeclSpec(decl.Sym, &decl.ValDeclSpec)
}

func (v *depChecker) visitValDeclSpec(sym *ir.Symbol, decl *ir.ValDeclSpec) {
	if sym == nil {
		return
	}
	v.declSym = sym
	if decl.Type != nil {
		v.exprMode = exprModeType
		ir.VisitExpr(v, decl.Type)
		v.exprMode = exprModeNone
	}
	if decl.Initializer != nil {
		ir.VisitExpr(v, decl.Initializer)
	}
	v.declSym = nil
}

func (v *depChecker) VisitFuncDecl(decl *ir.FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))
	for _, param := range decl.Params {
		v.VisitValDecl(param)
	}
	v.VisitValDecl(decl.Return)
	if decl.Body != nil {
		ir.VisitStmtList(v, decl.Body.Stmts)
	}
}

func (v *depChecker) VisitStructDecl(decl *ir.StructDecl) {
	for _, f := range decl.Fields {
		v.VisitValDecl(f)
	}
}

func (v *depChecker) VisitBlockStmt(stmt *ir.BlockStmt) {
	defer setScope(setScope(v.c, stmt.Scope))
	ir.VisitStmtList(v, stmt.Stmts)
}

func (v *depChecker) VisitDeclStmt(stmt *ir.DeclStmt) {
	ir.VisitDecl(v, stmt.D)
}

func (v *depChecker) VisitIfStmt(stmt *ir.IfStmt) {
	v.VisitBlockStmt(stmt.Body)
	if stmt.Else != nil {
		ir.VisitStmt(v, stmt.Else)
	}
}

func (v *depChecker) VisitForStmt(stmt *ir.ForStmt) {
	if stmt.Init != nil {
		v.VisitValDecl(stmt.Init)
	}

	if stmt.Cond != nil {
		ir.VisitExpr(v, stmt.Cond)
	}

	if stmt.Inc != nil {
		ir.VisitStmt(v, stmt.Inc)
	}

	v.VisitBlockStmt(stmt.Body)
}

func (v *depChecker) VisitReturnStmt(stmt *ir.ReturnStmt) {
	if stmt.X != nil {
		ir.VisitExpr(v, stmt.X)
	}
}

func (v *depChecker) VisitAssignStmt(stmt *ir.AssignStmt) {
	ir.VisitExpr(v, stmt.Left)
	ir.VisitExpr(v, stmt.Right)
}

func (v *depChecker) VisitExprStmt(stmt *ir.ExprStmt) {
	ir.VisitExpr(v, stmt.X)
}

func (v *depChecker) VisitPointerTypeExpr(expr *ir.PointerTypeExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *depChecker) VisitArrayTypeExpr(expr *ir.ArrayTypeExpr) ir.Expr {
	if expr.Size != nil {
		ir.VisitExpr(v, expr.Size)
	}
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *depChecker) VisitFuncTypeExpr(expr *ir.FuncTypeExpr) ir.Expr {
	if expr.ABI != nil {
		ir.VisitExpr(v, expr.ABI)
	}
	for _, param := range expr.Params {
		ir.VisitExpr(v, param.Type)
	}
	ir.VisitExpr(v, expr.Return.Type)
	return expr
}

func (v *depChecker) VisitBinaryExpr(expr *ir.BinaryExpr) ir.Expr {
	ir.VisitExpr(v, expr.Left)
	ir.VisitExpr(v, expr.Right)
	return expr
}

func (v *depChecker) VisitUnaryExpr(expr *ir.UnaryExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *depChecker) VisitStructLit(expr *ir.StructLit) ir.Expr {
	ir.VisitExpr(v, expr.Name)
	for _, kv := range expr.Initializers {
		ir.VisitExpr(v, kv.Value)
	}
	return expr
}

func (v *depChecker) VisitArrayLit(expr *ir.ArrayLit) ir.Expr {
	for _, init := range expr.Initializers {
		ir.VisitExpr(v, init)
	}
	return expr
}

func (v *depChecker) VisitIdent(expr *ir.Ident) ir.Expr {
	sym := v.c.lookup(expr.Literal())
	v.tryAddDependency(sym, expr.Name.Pos)
	return expr
}

func (v *depChecker) tryAddDependency(sym *ir.Symbol, pos token.Position) {
	if sym != nil {
		if decl, ok := v.c.decls[sym]; ok {
			edge := ir.DeclDependencyEdge{Sym: v.declSym}
			edge.IsType = v.exprMode == exprModeType
			edge.Pos = ir.NewPosition(v.c.filename(), pos)
			graph := v.c.topDecl().DependencyGraph()
			(*graph)[decl] = append((*graph)[decl], edge)
		}
	}
}

func (v *depChecker) VisitDotExpr(expr *ir.DotExpr) ir.Expr {
	if ident, ok := expr.X.(*ir.Ident); ok {
		sym := v.c.lookup(ident.Literal())
		if sym != nil && sym.ID == ir.ModuleSymbol {
			defer setScope(setScope(v.c, sym.Parent))
			v.VisitIdent(expr.Name)
		} else {
			v.tryAddDependency(sym, ident.Name.Pos)
		}
	} else {
		ir.VisitExpr(v, expr.X)
	}
	return expr
}

func (v *depChecker) VisitCastExpr(expr *ir.CastExpr) ir.Expr {
	ir.VisitExpr(v, expr.ToTyp)
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *depChecker) VisitLenExpr(expr *ir.LenExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *depChecker) VisitFuncCall(expr *ir.FuncCall) ir.Expr {
	ir.VisitExpr(v, expr.X)
	ir.VisitExprList(v, expr.Args)
	return expr
}

func (v *depChecker) VisitAddressExpr(expr *ir.AddressExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *depChecker) VisitIndexExpr(expr *ir.IndexExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	ir.VisitExpr(v, expr.Index)
	return expr
}

func (v *depChecker) VisitSliceExpr(expr *ir.SliceExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	if expr.Start != nil {
		ir.VisitExpr(v, expr.Start)
	}
	if expr.End != nil {
		ir.VisitExpr(v, expr.End)
	}
	return expr
}
