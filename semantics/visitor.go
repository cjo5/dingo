package semantics

import (
	"fmt"
)

// Visitor interface is used when walking the AST.
type Visitor interface {
	Module(mod *Module)

	// Decls
	VisitBadDecl(decl *BadDecl)
	VisitImport(decl *Import)
	VisitValTopDecl(decl *ValTopDecl)
	VisitValDecl(decl *ValDecl)
	VisitFuncDecl(decl *FuncDecl)
	VisitStructDecl(decl *StructDecl)

	// Stmts
	VisitBadStmt(stmt *BadStmt)
	VisitBlockStmt(stmt *BlockStmt)
	VisitDeclStmt(stmt *DeclStmt)
	VisitPrintStmt(stmt *PrintStmt)
	VisitIfStmt(stmt *IfStmt)
	VisitWhileStmt(stmt *WhileStmt)
	VisitReturnStmt(stmt *ReturnStmt)
	VisitBranchStmt(stmt *BranchStmt)
	VisitAssignStmt(stmt *AssignStmt)
	VisitExprStmt(stmt *ExprStmt)

	// Exprs
	VisitBadExpr(expr *BadExpr) Expr
	VisitBinaryExpr(expr *BinaryExpr) Expr
	VisitUnaryExpr(expr *UnaryExpr) Expr
	VisitBasicLit(expr *BasicLit) Expr
	VisitStructLit(expr *StructLit) Expr
	VisitIdent(expr *Ident) Expr
	VisitDotIdent(expr *DotIdent) Expr
	VisitFuncCall(expr *FuncCall) Expr
}

// BaseVisitor provides default implementations for Visitor functions.
type BaseVisitor struct{}

func (v *BaseVisitor) Module(mod *Module) {
	panic("Module")
}

func (v *BaseVisitor) VisitBadDecl(decl *BadDecl) {
	panic("VisitBadDecl")
}

func (v *BaseVisitor) VisitImport(decl *Import) {
	panic("VisitImport")
}

func (v *BaseVisitor) VisitValTopDecl(decl *ValTopDecl) {}
func (v *BaseVisitor) VisitValDecl(decl *ValDecl)       {}
func (v *BaseVisitor) VisitFuncDecl(decl *FuncDecl)     {}

func (v *BaseVisitor) VisitStructDecl(decl *StructDecl) {}

func (v *BaseVisitor) VisitBadStmt(stmt *BadStmt) {
	panic("VisitBadStmt")
}

func (v *BaseVisitor) VisitBlockStmt(stmt *BlockStmt) {}

func (v *BaseVisitor) VisitDeclStmt(stmt *DeclStmt) {}

func (v *BaseVisitor) VisitPrintStmt(stmt *PrintStmt)   {}
func (v *BaseVisitor) VisitIfStmt(stmt *IfStmt)         {}
func (v *BaseVisitor) VisitWhileStmt(stmt *WhileStmt)   {}
func (v *BaseVisitor) VisitReturnStmt(stmt *ReturnStmt) {}
func (v *BaseVisitor) VisitBranchStmt(stmt *BranchStmt) {}
func (v *BaseVisitor) VisitAssignStmt(stmt *AssignStmt) {}
func (v *BaseVisitor) VisitExprStmt(stmt *ExprStmt)     {}

func (v *BaseVisitor) VisitBadExpr(decl *BadExpr) Expr {
	panic("VisitBadExpr")
}

func (v *BaseVisitor) VisitBinaryExpr(expr *BinaryExpr) Expr { return expr }
func (v *BaseVisitor) VisitUnaryExpr(expr *UnaryExpr) Expr   { return expr }
func (v *BaseVisitor) VisitBasicLit(expr *BasicLit) Expr     { return expr }
func (v *BaseVisitor) VisitStructLit(expr *StructLit) Expr   { return expr }
func (v *BaseVisitor) VisitIdent(expr *Ident) Expr           { return expr }
func (v *BaseVisitor) VisitDotIdent(expr *DotIdent) Expr     { return expr }
func (v *BaseVisitor) VisitFuncCall(expr *FuncCall) Expr     { return expr }

// VisitNode switches on node type and invokes corresponding Visit function.
func VisitNode(v Visitor, node Node) {
	switch n := node.(type) {
	case Decl:
		VisitDecl(v, n)
	case Stmt:
		VisitStmt(v, n)
	case Expr:
		VisitExpr(v, n)
	default:
		panic(fmt.Sprintf("Unhandled node %T", n))
	}
}

// VisitDecl switches on decl type and invokes corresponding Visit function.
func VisitDecl(v Visitor, decl Decl) {
	switch d := decl.(type) {
	case *BadDecl:
		v.VisitBadDecl(d)
	case *Import:
		v.VisitImport(d)
	case *ValTopDecl:
		v.VisitValTopDecl(d)
	case *ValDecl:
		v.VisitValDecl(d)
	case *FuncDecl:
		v.VisitFuncDecl(d)
	case *StructDecl:
		v.VisitStructDecl(d)
	default:
		panic(fmt.Sprintf("Unhandled decl %T", d))
	}
}

// VisitStmt switches on stmt type and invokes corresponding Visit function.
func VisitStmt(v Visitor, stmt Stmt) {
	switch s := stmt.(type) {
	case *BadStmt:
		v.VisitBadStmt(s)
	case *BlockStmt:
		v.VisitBlockStmt(s)
	case *DeclStmt:
		v.VisitDeclStmt(s)
	case *PrintStmt:
		v.VisitPrintStmt(s)
	case *IfStmt:
		v.VisitIfStmt(s)
	case *WhileStmt:
		v.VisitWhileStmt(s)
	case *ReturnStmt:
		v.VisitReturnStmt(s)
	case *BranchStmt:
		v.VisitBranchStmt(s)
	case *AssignStmt:
		v.VisitAssignStmt(s)
	case *ExprStmt:
		v.VisitExprStmt(s)
	default:
		panic(fmt.Sprintf("Unhandled stmt %T", s))
	}
}

// VisitExpr switches on expr type and invokes corresponding Visit function.
func VisitExpr(v Visitor, expr Expr) Expr {
	switch e := expr.(type) {
	case *BadExpr:
		return v.VisitBadExpr(e)
	case *BinaryExpr:
		return v.VisitBinaryExpr(e)
	case *UnaryExpr:
		return v.VisitUnaryExpr(e)
	case *BasicLit:
		return v.VisitBasicLit(e)
	case *StructLit:
		return v.VisitStructLit(e)
	case *Ident:
		return v.VisitIdent(e)
	case *DotIdent:
		return v.VisitDotIdent(e)
	case *FuncCall:
		return v.VisitFuncCall(e)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", e))
	}
}

func StartWalk(v Visitor, node Node) {
	switch t := node.(type) {
	case *Program:
		StartProgramWalk(v, t)
	case *Module:
		v.Module(t)
	default:
		VisitNode(v, node)
	}
}

// StartProgramWalk will visit the program's modules.
func StartProgramWalk(v Visitor, prog *Program) {
	for _, mod := range prog.Modules {
		v.Module(mod)
	}
}

// VisitDeclList visits each decl
func VisitDeclList(v Visitor, decls []Decl) {
	for _, decl := range decls {
		VisitDecl(v, decl)
	}
}

// VisitTopDeclList visits each top decl
func VisitTopDeclList(v Visitor, decls []TopDecl) {
	for _, decl := range decls {
		VisitDecl(v, decl)
	}
}

// VisitImportList visits each import.
func VisitImportList(v Visitor, decls []*Import) {
	for _, decl := range decls {
		v.VisitImport(decl)
	}
}

// VisitStmtList visits each stmt.
func VisitStmtList(v Visitor, stmts []Stmt) {
	for _, stmt := range stmts {
		VisitStmt(v, stmt)
	}
}

// VisitExprList visits each expr.
func VisitExprList(v Visitor, exprs []Expr) {
	for i, expr := range exprs {
		res := VisitExpr(v, expr)
		if res == nil {
			panic(fmt.Sprintf("Visitor %T returns nil on expr %T", v, expr))
		}
		exprs[i] = res
	}
}
