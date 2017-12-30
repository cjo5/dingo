package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
)

type depChecker struct {
	ir.BaseVisitor
	exprMode int
	c        *context
}

func depCheck(c *context) {
	c.resetWalkState()
	v := &depChecker{c: c}
	ir.VisitModuleSet(v, c.set)
	sortDecls(v.c)
}

func sortDecls(c *context) {
	for _, mod := range c.set.Modules {
		for _, decl := range mod.Decls {
			decl.SetNodeColor(ir.NodeColorWhite)
		}
	}

	for _, mod := range c.set.Modules {
		var sortedDecls []ir.TopDecl
		for _, decl := range mod.Decls {
			sym := decl.Symbol()
			if sym == nil {
				continue
			}

			var cycleTrace []ir.TopDecl
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

				sym.Flags |= ir.SymFlagDepCycle

				trace := common.NewTrace(fmt.Sprintf("%s uses:", sym.Name), nil)
				for i := len(cycleTrace) - 1; i >= 0; i-- {
					s := cycleTrace[i].Symbol()
					s.Flags |= ir.SymFlagDepCycle
					line := cycleTrace[i].Context().Path + ":" + s.Pos.String() + ":" + s.Name
					trace.Lines = append(trace.Lines, line)
				}

				errorMsg := "initializer cycle detected"
				if sym.ID == ir.TypeSymbol {
					errorMsg = "type cycle detected"
				}

				c.errors.AddTrace(decl.Context().Path, sym.Pos, common.GenericError, trace, errorMsg)
			}
		}
		mod.Decls = sortedDecls
	}
}

// Returns false if cycle
func sortDeclDependencies(decl ir.TopDecl, trace *[]ir.TopDecl, sortedDecls *[]ir.TopDecl) bool {
	color := decl.NodeColor()
	if color == ir.NodeColorBlack {
		return true
	} else if color == ir.NodeColorGray {
		return false
	}

	sortOK := true
	decl.SetNodeColor(ir.NodeColorGray)
	for _, dep := range decl.Dependencies() {
		if !sortDeclDependencies(dep, trace, sortedDecls) {
			*trace = append(*trace, dep)
			sortOK = false
			break
		}
	}
	decl.SetNodeColor(ir.NodeColorBlack)
	*sortedDecls = append(*sortedDecls, decl)
	return sortOK
}

func (v *depChecker) Module(mod *ir.Module) {
	v.c.mod = mod
	v.c.scope = mod.Scope
	for _, decl := range mod.Decls {
		v.c.setCurrentTopDecl(decl)
		ir.VisitDecl(v, decl)
	}
}

func (v *depChecker) VisitValTopDecl(decl *ir.ValTopDecl) {
	if decl.Type != nil {
		ir.VisitExpr(v, decl.Type)
	}
	if decl.Initializer != nil {
		ir.VisitExpr(v, decl.Initializer)
	}
}

func (v *depChecker) VisitValDecl(decl *ir.ValDecl) {
	if decl.Type != nil {
		v.exprMode = exprModeType
		ir.VisitExpr(v, decl.Type)
		v.exprMode = exprModeNone
	}
	if decl.Initializer != nil {
		ir.VisitExpr(v, decl.Initializer)
	}
}

func (v *depChecker) VisitFuncDecl(decl *ir.FuncDecl) {
	defer setScope(setScope(v.c, decl.Scope))
	for _, param := range decl.Params {
		ir.VisitExpr(v, param.Type)
	}
	ir.VisitExpr(v, decl.TReturn)
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
	// Pointer types cannot be a dependency
	return expr
}

func (v *depChecker) VisitArrayTypeExpr(expr *ir.ArrayTypeExpr) ir.Expr {
	// Slice types cannot be a dependency
	if expr.Size != nil {
		ir.VisitExpr(v, expr.Size)
		ir.VisitExpr(v, expr.X)
	}
	return expr
}

func (v *depChecker) VisitFuncTypeExpr(expr *ir.FuncTypeExpr) ir.Expr {
	// Function types cannot be a dependency
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
	if sym != nil {
		if decl, ok := v.c.topDecls[sym]; ok {
			_, isFunc1 := v.c.topDecl.(*ir.FuncDecl)
			_, isFunc2 := decl.(*ir.FuncDecl)

			if !isFunc1 && !isFunc2 {
				v.c.topDecl.AddDependency(decl)
			}
		}
	}
	return expr
}

func (v *depChecker) VisitDotExpr(expr *ir.DotExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	v.VisitIdent(expr.Name)
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
