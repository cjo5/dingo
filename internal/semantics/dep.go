package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
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
outer:
	for _, mod := range c.set.Modules {
		var sortedDecls []ir.TopDecl
		c.set.ResetDeclColors()

		for _, decl := range mod.Decls {
			if decl.Symbol() == nil {
				continue
			}

			var cycleTrace []ir.TopDecl
			if !sortDeclDependencies(mod.FQN, decl, &cycleTrace, &sortedDecls) {
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

				var lines []string
				for i, j := len(cycleTrace)-1, 0; i > 0; i, j = i-1, j+1 {
					next := j + 1
					if next == len(cycleTrace)-1 {
						next = 0
					}

					s := cycleTrace[i].Symbol()
					line := fmt.Sprintf("  >> [%d] %s:%s uses [%d]", j, s.DefPos, s.Name, next)
					lines = append(lines, line)
				}

				c.errors.AddContext(sym.DefPos, lines, "cycle detected")
				break outer
			}
		}

		mod.Decls = sortedDecls
	}
}

func sortDeclDependencies(fqn string, decl ir.TopDecl, trace *[]ir.TopDecl, sortedDecls *[]ir.TopDecl) bool {
	color := decl.Color()
	if color == ir.BlackColor {
		return true
	} else if color == ir.GrayColor {
		return false
	}

	sortOK := true
	decl.SetColor(ir.GrayColor)

	graph := *decl.DependencyGraph()
	sym := decl.Symbol()

	var weak []ir.TopDecl

	for dep, node := range graph {
		depSym := dep.Symbol()

		if sym.ModFQN() != fqn && !depSym.IsType() {
			// Only type symbols from other modules are needed
			continue
		} else if node.Weak {
			weak = append(weak, dep)
			continue
		}

		if !sortDeclDependencies(fqn, dep, trace, sortedDecls) {
			*trace = append(*trace, dep)
			sortOK = false
			break
		}
	}

	decl.SetColor(ir.BlackColor)
	*sortedDecls = append(*sortedDecls, decl)

	if sortOK {
		for _, dep := range weak {
			if dep.Color() == ir.WhiteColor {
				// Ensure dependencies from other modules are included
				if !sortDeclDependencies(fqn, dep, trace, sortedDecls) {
					*trace = append(*trace, dep)
					sortOK = false
					break
				}
			}
		}
	}

	return sortOK
}

func (v *depChecker) Module(mod *ir.Module) {
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
	defer setScope(setScope(v.c, stmt.Body.Scope))

	if stmt.Init != nil {
		v.VisitValDecl(stmt.Init)
	}

	if stmt.Cond != nil {
		ir.VisitExpr(v, stmt.Cond)
	}

	if stmt.Inc != nil {
		ir.VisitStmt(v, stmt.Inc)
	}

	ir.VisitStmtList(v, stmt.Body.Stmts)
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
	for _, arg := range expr.Args {
		ir.VisitExpr(v, arg.Value)
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
	sym := v.c.lookup(expr.Literal)
	expr.SetSymbol(sym)
	v.tryAddDependency(sym, expr.Pos())
	return expr
}

func (v *depChecker) tryAddDependency(sym *ir.Symbol, pos token.Position) {
	if sym != nil {
		if decl, ok := v.c.decls[sym]; ok {
			link := ir.DeclDependencyLink{Sym: v.declSym}
			link.IsType = v.exprMode == exprModeType
			link.Pos = pos

			graph := v.c.topDecl().DependencyGraph()
			dep := (*graph)[decl]
			if dep == nil {
				dep = &ir.DeclDependency{}
			}
			dep.Links = append(dep.Links, link)

			(*graph)[decl] = dep
		}
	}
}

func (v *depChecker) VisitDotExpr(expr *ir.DotExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	sym := ir.ExprSymbol(expr.X)

	if sym != nil && sym.ID == ir.ModuleSymbol {
		tmod := sym.T.(*ir.ModuleType)
		defer setScope(setScope(v.c, tmod.Scope))
		v.VisitIdent(expr.Name)
	}

	return expr
}

func (v *depChecker) VisitCastExpr(expr *ir.CastExpr) ir.Expr {
	ir.VisitExpr(v, expr.ToType)
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *depChecker) VisitLenExpr(expr *ir.LenExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *depChecker) VisitSizeExpr(expr *ir.SizeExpr) ir.Expr {
	ir.VisitExpr(v, expr.X)
	return expr
}

func (v *depChecker) VisitFuncCall(expr *ir.FuncCall) ir.Expr {
	ir.VisitExpr(v, expr.X)
	for _, arg := range expr.Args {
		ir.VisitExpr(v, arg.Value)
	}
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
