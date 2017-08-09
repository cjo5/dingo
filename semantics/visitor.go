package semantics

import "fmt"
import "github.com/jhnl/interpreter/ir"

// Visitor interface is used when walking the ir.
type Visitor interface {
	Module(mod *ir.Module)

	// Decls
	VisitBadDecl(decl *ir.BadDecl)
	VisitImport(decl *ir.Import)
	VisitValTopDecl(decl *ir.ValTopDecl)
	VisitValDecl(decl *ir.ValDecl)
	VisitFuncDecl(decl *ir.FuncDecl)
	VisitStructDecl(decl *ir.StructDecl)

	// Stmts
	VisitBadStmt(stmt *ir.BadStmt)
	VisitBlockStmt(stmt *ir.BlockStmt)
	VisitDeclStmt(stmt *ir.DeclStmt)
	VisitPrintStmt(stmt *ir.PrintStmt)
	VisitIfStmt(stmt *ir.IfStmt)
	VisitWhileStmt(stmt *ir.WhileStmt)
	VisitReturnStmt(stmt *ir.ReturnStmt)
	VisitBranchStmt(stmt *ir.BranchStmt)
	VisitAssignStmt(stmt *ir.AssignStmt)
	VisitExprStmt(stmt *ir.ExprStmt)

	// Exprs
	VisitBadExpr(expr *ir.BadExpr) ir.Expr
	VisitBinaryExpr(expr *ir.BinaryExpr) ir.Expr
	VisitUnaryExpr(expr *ir.UnaryExpr) ir.Expr
	VisitBasicLit(expr *ir.BasicLit) ir.Expr
	VisitStructLit(expr *ir.StructLit) ir.Expr
	VisitIdent(expr *ir.Ident) ir.Expr
	VisitDotExpr(expr *ir.DotExpr) ir.Expr
	VisitFuncCall(expr *ir.FuncCall) ir.Expr
}

// BaseVisitor provides default implementations for Visitor functions.
type BaseVisitor struct{}

func (v *BaseVisitor) Module(mod *ir.Module) {
	panic("Module")
}

func (v *BaseVisitor) VisitBadDecl(decl *ir.BadDecl) {
	panic("VisitBadDecl")
}

func (v *BaseVisitor) VisitImport(decl *ir.Import) {
	panic("VisitImport")
}

func (v *BaseVisitor) VisitValTopDecl(decl *ir.ValTopDecl) {}
func (v *BaseVisitor) VisitValDecl(decl *ir.ValDecl)       {}
func (v *BaseVisitor) VisitFuncDecl(decl *ir.FuncDecl)     {}

func (v *BaseVisitor) VisitStructDecl(decl *ir.StructDecl) {}

func (v *BaseVisitor) VisitBadStmt(stmt *ir.BadStmt) {
	panic("VisitBadStmt")
}

func (v *BaseVisitor) VisitBlockStmt(stmt *ir.BlockStmt) {}

func (v *BaseVisitor) VisitDeclStmt(stmt *ir.DeclStmt) {}

func (v *BaseVisitor) VisitPrintStmt(stmt *ir.PrintStmt)   {}
func (v *BaseVisitor) VisitIfStmt(stmt *ir.IfStmt)         {}
func (v *BaseVisitor) VisitWhileStmt(stmt *ir.WhileStmt)   {}
func (v *BaseVisitor) VisitReturnStmt(stmt *ir.ReturnStmt) {}
func (v *BaseVisitor) VisitBranchStmt(stmt *ir.BranchStmt) {}
func (v *BaseVisitor) VisitAssignStmt(stmt *ir.AssignStmt) {}
func (v *BaseVisitor) VisitExprStmt(stmt *ir.ExprStmt)     {}

func (v *BaseVisitor) VisitBadExpr(decl *ir.BadExpr) ir.Expr {
	panic("VisitBadExpr")
}

func (v *BaseVisitor) VisitBinaryExpr(expr *ir.BinaryExpr) ir.Expr { return expr }
func (v *BaseVisitor) VisitUnaryExpr(expr *ir.UnaryExpr) ir.Expr   { return expr }
func (v *BaseVisitor) VisitBasicLit(expr *ir.BasicLit) ir.Expr     { return expr }
func (v *BaseVisitor) VisitStructLit(expr *ir.StructLit) ir.Expr   { return expr }
func (v *BaseVisitor) VisitIdent(expr *ir.Ident) ir.Expr           { return expr }
func (v *BaseVisitor) VisitDotExpr(expr *ir.DotExpr) ir.Expr       { return expr }
func (v *BaseVisitor) VisitFuncCall(expr *ir.FuncCall) ir.Expr     { return expr }

// VisitNode switches on node type and invokes corresponding Visit function.
func VisitNode(v Visitor, node ir.Node) {
	switch n := node.(type) {
	case ir.Decl:
		VisitDecl(v, n)
	case ir.Stmt:
		VisitStmt(v, n)
	case ir.Expr:
		VisitExpr(v, n)
	default:
		panic(fmt.Sprintf("Unhandled node %T", n))
	}
}

// VisitDecl switches on decl type and invokes corresponding Visit function.
func VisitDecl(v Visitor, decl ir.Decl) {
	switch d := decl.(type) {
	case *ir.BadDecl:
		v.VisitBadDecl(d)
	case *ir.Import:
		v.VisitImport(d)
	case *ir.ValTopDecl:
		v.VisitValTopDecl(d)
	case *ir.ValDecl:
		v.VisitValDecl(d)
	case *ir.FuncDecl:
		v.VisitFuncDecl(d)
	case *ir.StructDecl:
		v.VisitStructDecl(d)
	default:
		panic(fmt.Sprintf("Unhandled decl %T", d))
	}
}

// VisitStmt switches on stmt type and invokes corresponding Visit function.
func VisitStmt(v Visitor, stmt ir.Stmt) {
	switch s := stmt.(type) {
	case *ir.BadStmt:
		v.VisitBadStmt(s)
	case *ir.BlockStmt:
		v.VisitBlockStmt(s)
	case *ir.DeclStmt:
		v.VisitDeclStmt(s)
	case *ir.PrintStmt:
		v.VisitPrintStmt(s)
	case *ir.IfStmt:
		v.VisitIfStmt(s)
	case *ir.WhileStmt:
		v.VisitWhileStmt(s)
	case *ir.ReturnStmt:
		v.VisitReturnStmt(s)
	case *ir.BranchStmt:
		v.VisitBranchStmt(s)
	case *ir.AssignStmt:
		v.VisitAssignStmt(s)
	case *ir.ExprStmt:
		v.VisitExprStmt(s)
	default:
		panic(fmt.Sprintf("Unhandled stmt %T", s))
	}
}

// VisitExpr switches on expr type and invokes corresponding Visit function.
func VisitExpr(v Visitor, expr ir.Expr) ir.Expr {
	switch e := expr.(type) {
	case *ir.BadExpr:
		return v.VisitBadExpr(e)
	case *ir.BinaryExpr:
		return v.VisitBinaryExpr(e)
	case *ir.UnaryExpr:
		return v.VisitUnaryExpr(e)
	case *ir.BasicLit:
		return v.VisitBasicLit(e)
	case *ir.StructLit:
		return v.VisitStructLit(e)
	case *ir.Ident:
		return v.VisitIdent(e)
	case *ir.DotExpr:
		return v.VisitDotExpr(e)
	case *ir.FuncCall:
		return v.VisitFuncCall(e)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", e))
	}
}

func StartWalk(v Visitor, node ir.Node) {
	switch t := node.(type) {
	case *ir.ModuleSet:
		VisitModuleSet(v, t)
	case *ir.Module:
		v.Module(t)
	default:
		VisitNode(v, node)
	}
}

// VisitModuleSet will visit each module.
func VisitModuleSet(v Visitor, set *ir.ModuleSet) {
	for _, mod := range set.Modules {
		v.Module(mod)
	}
}

// VisitDeclList visits each decl
func VisitDeclList(v Visitor, decls []ir.Decl) {
	for _, decl := range decls {
		VisitDecl(v, decl)
	}
}

// VisitTopDeclList visits each top decl.
func VisitTopDeclList(v Visitor, decls []ir.TopDecl) {
	for _, decl := range decls {
		VisitDecl(v, decl)
	}
}

// VisitImportList visits each import.
func VisitImportList(v Visitor, decls []*ir.Import) {
	for _, decl := range decls {
		v.VisitImport(decl)
	}
}

// VisitStmtList visits each stmt.
func VisitStmtList(v Visitor, stmts []ir.Stmt) {
	for _, stmt := range stmts {
		VisitStmt(v, stmt)
	}
}

// VisitExprList visits each expr.
func VisitExprList(v Visitor, exprs []ir.Expr) {
	for i, expr := range exprs {
		res := VisitExpr(v, expr)
		if res == nil {
			panic(fmt.Sprintf("Visitor %T returns nil on expr %T", v, expr))
		}
		exprs[i] = res
	}
}
