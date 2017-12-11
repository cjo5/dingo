package semantics

import "github.com/jhnl/dingo/ir"

// Check program.
// Resolve identifiers, type check and look for cyclic dependencies between identifiers.
//
func Check(set *ir.ModuleSet) error {
	c := newChecker(set)

	symbolWalk(c)
	dependencyWalk(c)
	c.sortDecls()
	typeWalk(c)

	if c.errors.IsFatal() {
		return c.errors
	}

	return nil
}

// PrintExpr in-order.
func PrintExpr(expr ir.Expr) string {
	p := &exprPrinter{}
	ir.VisitExpr(p, expr)
	return p.buffer.String()
}
