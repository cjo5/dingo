package semantics

import "github.com/jhnl/dingo/ir"

// Check program.
// Resolve identifiers, type check and look for cyclic dependencies between identifiers.
//
// Module dependencies are resolved before Check is invoked.
// Module[0] has no dependencies.
// Module[1] has no dependencies, or only depends on Module[0].
// Module[2] has no dependencies, or depends on one of, or both, Module[0] and Module[1].
// And so on...
//
func Check(set *ir.ModuleSet) error {
	c := newChecker(set)

	c.sortModules()
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
