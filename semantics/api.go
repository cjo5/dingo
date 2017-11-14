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

// PrintTree pre-order.
func PrintTree(n ir.Node) string {
	p := &treePrinter{}
	ir.StartWalk(p, n)
	return p.buffer.String()
}

// PrintExpr in-order.
func PrintExpr(expr ir.Expr) string {
	p := &exprPrinter{}
	ir.VisitExpr(p, expr)
	return p.buffer.String()
}
