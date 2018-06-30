package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

func (c *checker) buildDependencyGraph() {
	c.resetWalkState()
	for _, mod := range c.set.Modules {
		c.scope = mod.Scope
		for _, decl := range mod.Decls {
			c.pushTopDecl(decl)
			c.depswitchDecl(decl)
			c.popTopDecl()
		}
	}
}

func (c *checker) sortDecls() {
out:
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
				break out
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

func (c *checker) depswitchDecl(decl ir.Decl) {
	switch decl := decl.(type) {
	case *ir.TypeTopDecl:
		c.addTypeDeclDependencies(decl.Sym, &decl.TypeDeclSpec)
	case *ir.TypeDecl:
		c.addTypeDeclDependencies(decl.Sym, &decl.TypeDeclSpec)
	case *ir.ValTopDecl:
		c.addValDeclDependencies(decl.Sym, &decl.ValDeclSpec)
	case *ir.ValDecl:
		c.addValDeclDependencies(decl.Sym, &decl.ValDeclSpec)
	case *ir.FuncDecl:
		defer setScope(setScope(c, decl.Scope))
		for _, param := range decl.Params {
			c.depswitchDecl(param)
		}
		c.depswitchDecl(decl.Return)
		if decl.Body != nil {
			stmtList(decl.Body.Stmts, c.depswitchStmt)
		}
	case *ir.StructDecl:
		for _, f := range decl.Fields {
			c.depswitchDecl(f)
		}
	default:
		panic(fmt.Sprintf("Unhandled decl %T", decl))
	}
}

func (c *checker) addTypeDeclDependencies(sym *ir.Symbol, decl *ir.TypeDeclSpec) {
	if sym == nil {
		return
	}
	c.declSym = sym
	c.exprMode = exprModeType
	c.depswitchExpr(decl.Type)
	c.exprMode = exprModeNone
	c.declSym = nil
}

func (c *checker) addValDeclDependencies(sym *ir.Symbol, decl *ir.ValDeclSpec) {
	if sym == nil {
		return
	}
	c.declSym = sym
	if decl.Type != nil {
		c.exprMode = exprModeType
		c.depswitchExpr(decl.Type)
		c.exprMode = exprModeNone
	}
	if decl.Initializer != nil {
		c.depswitchExpr(decl.Initializer)
	}
	c.declSym = nil
}

func (c *checker) depswitchStmt(stmt ir.Stmt) {
	switch stmt := stmt.(type) {
	case *ir.BlockStmt:
		defer setScope(setScope(c, stmt.Scope))
		stmtList(stmt.Stmts, c.depswitchStmt)
	case *ir.DeclStmt:
		c.depswitchDecl(stmt.D)
	case *ir.IfStmt:
		c.depswitchStmt(stmt.Body)
		if stmt.Else != nil {
			c.depswitchStmt(stmt.Else)
		}
	case *ir.ForStmt:
		defer setScope(setScope(c, stmt.Body.Scope))
		if stmt.Init != nil {
			c.depswitchDecl(stmt.Init)
		}
		if stmt.Cond != nil {
			c.depswitchExpr(stmt.Cond)
		}
		if stmt.Inc != nil {
			c.depswitchStmt(stmt.Inc)
		}
		stmtList(stmt.Body.Stmts, c.depswitchStmt)
	case *ir.ReturnStmt:
		if stmt.X != nil {
			c.depswitchExpr(stmt.X)
		}
	case *ir.DeferStmt:
		c.depswitchStmt(stmt.S)
	case *ir.BranchStmt:
		// Do nothing
	case *ir.AssignStmt:
		c.depswitchExpr(stmt.Left)
		c.depswitchExpr(stmt.Right)
	case *ir.ExprStmt:
		c.depswitchExpr(stmt.X)
	default:
		panic(fmt.Sprintf("Unhandled stmt %T", stmt))
	}
}

func (c *checker) depswitchExpr(expr ir.Expr) {
	switch expr := expr.(type) {
	case *ir.PointerTypeExpr:
		c.depswitchExpr(expr.X)
	case *ir.ArrayTypeExpr:
		if expr.Size != nil {
			c.depswitchExpr(expr.Size)
		}
		c.depswitchExpr(expr.X)
	case *ir.FuncTypeExpr:
		if expr.ABI != nil {
			c.depswitchExpr(expr.ABI)
		}
		for _, param := range expr.Params {
			c.depswitchExpr(param.Type)
		}
		c.depswitchExpr(expr.Return.Type)
	case *ir.Ident:
		sym := c.lookup(expr.Literal)
		expr.SetSymbol(sym)
		c.tryAddDependency(sym, expr.Pos())
	case *ir.BasicLit:
		// Do nothing
	case *ir.StructLit:
		c.depswitchExpr(expr.Name)
		for _, arg := range expr.Args {
			c.depswitchExpr(arg.Value)
		}
	case *ir.ArrayLit:
		for _, init := range expr.Initializers {
			c.depswitchExpr(init)
		}
	case *ir.BinaryExpr:
		c.depswitchExpr(expr.Left)
		c.depswitchExpr(expr.Right)
	case *ir.UnaryExpr:
		c.depswitchExpr(expr.X)
	case *ir.DotExpr:
		c.depswitchExpr(expr.X)
		sym := ir.ExprSymbol(expr.X)

		if sym != nil && sym.ID == ir.ModuleSymbol {
			tmod := sym.T.(*ir.ModuleType)
			defer setScope(setScope(c, tmod.Scope))
			c.depswitchExpr(expr.Name)
		}
	case *ir.IndexExpr:
		c.depswitchExpr(expr.X)
		c.depswitchExpr(expr.Index)
	case *ir.SliceExpr:
		c.depswitchExpr(expr.X)
		if expr.Start != nil {
			c.depswitchExpr(expr.Start)
		}
		if expr.End != nil {
			c.depswitchExpr(expr.End)
		}
	case *ir.FuncCall:
		c.depswitchExpr(expr.X)
		for _, arg := range expr.Args {
			c.depswitchExpr(arg.Value)
		}
	case *ir.CastExpr:
		c.depswitchExpr(expr.ToType)
		c.depswitchExpr(expr.X)
	case *ir.LenExpr:
		c.depswitchExpr(expr.X)
	case *ir.SizeExpr:
		c.depswitchExpr(expr.X)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", expr))
	}
}

func (c *checker) tryAddDependency(sym *ir.Symbol, pos token.Position) {
	if sym != nil {
		if decl, ok := c.decls[sym]; ok {
			link := ir.DeclDependencyLink{Sym: c.declSym}
			link.IsType = c.exprMode == exprModeType
			link.Pos = pos

			graph := c.topDecl().DependencyGraph()
			dep := (*graph)[decl]
			if dep == nil {
				dep = &ir.DeclDependency{}
			}
			dep.Links = append(dep.Links, link)

			(*graph)[decl] = dep
		}
	}
}
