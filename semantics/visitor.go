package semantics

import (
	"fmt"
)

// Visitor interface is used when traversing the AST.
type Visitor interface {
	// Decls
	visitBadDecl(decl *BadDecl)
	visitModule(decl *Module)
	visitImport(decl *Import)
	visitVarDecl(decl *VarDecl)
	visitFuncDecl(decl *FuncDecl)
	visitStructDecl(decl *StructDecl)

	// Stmts
	visitBadStmt(stmt *BadStmt)
	visitBlockStmt(stmt *BlockStmt)
	visitDeclStmt(stmt *DeclStmt)
	visitPrintStmt(stmt *PrintStmt)
	visitIfStmt(stmt *IfStmt)
	visitWhileStmt(stmt *WhileStmt)
	visitReturnStmt(stmt *ReturnStmt)
	visitBranchStmt(stmt *BranchStmt)
	visitAssignStmt(stmt *AssignStmt)
	visitExprStmt(stmt *ExprStmt)

	// Exprs
	visitBadExpr(expr *BadExpr) Expr
	visitBinaryExpr(expr *BinaryExpr) Expr
	visitUnaryExpr(expr *UnaryExpr) Expr
	visitLiteral(expr *Literal) Expr
	visitStructLiteral(expr *StructLiteral) Expr
	visitIdent(expr *Ident) Expr
	visitFuncCall(expr *FuncCall) Expr
	visitDotExpr(expr *DotExpr) Expr
}

// BaseVisitor provides default implementations for a subset of visitor functions.
type BaseVisitor struct{}

// VisitBadDecl default implementation.
func (v *BaseVisitor) visitBadDecl(decl *BadDecl) {
	panic(fmt.Sprintf("Bad decl"))
}

// VisitBadStmt default implementation.
func (v *BaseVisitor) visitBadStmt(stmt *BadStmt) {
	panic(fmt.Sprintf("Bad stmt"))
}

// VisitBadExpr default implementation.
func (v *BaseVisitor) visitBadExpr(decl *BadExpr) Expr {
	panic(fmt.Sprintf("Bad expr"))
}

// VisitNode switches on node type and invokes corresponding visit function.
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

// VisitDecl switches on decl type and invokes corresponding visit function.
func VisitDecl(v Visitor, decl Decl) {
	switch d := decl.(type) {
	case *BadDecl:
		v.visitBadDecl(d)
	case *Module:
		v.visitModule(d)
	case *Import:
		v.visitImport(d)
	case *VarDecl:
		v.visitVarDecl(d)
	case *FuncDecl:
		v.visitFuncDecl(d)
	case *StructDecl:
		v.visitStructDecl(d)
	default:
		panic(fmt.Sprintf("Unhandled decl %T", d))
	}
}

// VisitStmtList visits each stmt.
func VisitStmtList(v Visitor, stmts []Stmt) {
	for _, stmt := range stmts {
		VisitStmt(v, stmt)
	}
}

// VisitStmt switches on stmt type and invokes corresponding visit function.
func VisitStmt(v Visitor, stmt Stmt) {
	switch s := stmt.(type) {
	case *BadStmt:
		v.visitBadStmt(s)
	case *BlockStmt:
		v.visitBlockStmt(s)
	case *DeclStmt:
		v.visitDeclStmt(s)
	case *PrintStmt:
		v.visitPrintStmt(s)
	case *IfStmt:
		v.visitIfStmt(s)
	case *WhileStmt:
		v.visitWhileStmt(s)
	case *ReturnStmt:
		v.visitReturnStmt(s)
	case *BranchStmt:
		v.visitBranchStmt(s)
	case *AssignStmt:
		v.visitAssignStmt(s)
	case *ExprStmt:
		v.visitExprStmt(s)
	default:
		panic(fmt.Sprintf("Unhandled stmt %T", s))
	}
}

// VisitExpr switches on expr type and invokes corresponding visit function.
func VisitExpr(v Visitor, expr Expr) Expr {
	switch e := expr.(type) {
	case *BadExpr:
		return v.visitBadExpr(e)
	case *BinaryExpr:
		return v.visitBinaryExpr(e)
	case *UnaryExpr:
		return v.visitUnaryExpr(e)
	case *Literal:
		return v.visitLiteral(e)
	case *StructLiteral:
		return v.visitStructLiteral(e)
	case *Ident:
		return v.visitIdent(e)
	case *FuncCall:
		return v.visitFuncCall(e)
	case *DotExpr:
		return v.visitDotExpr(e)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", e))
	}
}
