package semantics

import (
	"fmt"
)

// Visitor interface is used when walking the AST.
type Visitor interface {
	// Generic nodes
	VisitNode(node Node)
	VisitProgram(prog *Program)
	VisitModule(mod *Module)
	VisitFile(decl *File)

	// Decls
	VisitDecl(decl Decl)
	VisitBadDecl(decl *BadDecl)
	VisitImport(decl *Import)
	VisitVarDecl(decl *VarDecl)
	VisitFuncDecl(decl *FuncDecl)
	VisitStructDecl(decl *StructDecl)

	// Stmts
	VisitStmt(stmt Stmt)
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
	VisitExpr(expr Expr) Expr
	VisitBadExpr(expr *BadExpr) Expr
	VisitBinaryExpr(expr *BinaryExpr) Expr
	VisitUnaryExpr(expr *UnaryExpr) Expr
	VisitLiteral(expr *Literal) Expr
	VisitStructLiteral(expr *StructLiteral) Expr
	VisitIdent(expr *Ident) Expr
	VisitFuncCall(expr *FuncCall) Expr
	VisitDotExpr(expr *DotExpr) Expr
}

// BaseVisitor provides default implementations for Visitor functions.
type BaseVisitor struct{}

// VisitNode switches on node type and invokes corresponding Visit function.
func (v *BaseVisitor) VisitNode(node Node) {
	switch n := node.(type) {
	case *Program:
		v.VisitProgram(n)
	case *Module:
		v.VisitModule(n)
	case *File:
		v.VisitFile(n)
	case Decl:
		v.VisitDecl(n)
	case Stmt:
		v.VisitStmt(n)
	case Expr:
		v.VisitExpr(n)
	default:
		panic(fmt.Sprintf("Unhandled node %T", n))
	}
}

func (v *BaseVisitor) VisitProgram(prog *Program) {
	VisitModuleList(v, prog.Modules)
}

func (v *BaseVisitor) VisitModule(mod *Module) {}

func (v *BaseVisitor) VisitFile(file *File) {}

// VisitDecl switches on decl type and invokes corresponding Visit function.
func (v *BaseVisitor) VisitDecl(decl Decl) {
	switch d := decl.(type) {
	case *BadDecl:
		v.VisitBadDecl(d)
	case *Import:
		v.VisitImport(d)
	case *VarDecl:
		v.VisitVarDecl(d)
	case *FuncDecl:
		v.VisitFuncDecl(d)
	case *StructDecl:
		v.VisitStructDecl(d)
	default:
		panic(fmt.Sprintf("Unhandled decl %T", d))
	}
}

func (v *BaseVisitor) VisitBadDecl(decl *BadDecl) {
	panic("VisitBadDecl")
}

func (v *BaseVisitor) VisitImport(decl *Import) {
	panic("VisitImport")
}

func (v *BaseVisitor) VisitVarDecl(decl *VarDecl)   {}
func (v *BaseVisitor) VisitFuncDecl(decl *FuncDecl) {}

func (v *BaseVisitor) VisitStructDecl(decl *StructDecl) {
	panic("VisitStructDecl")
}

// VisitStmt switches on stmt type and invokes corresponding Visit function.
func (v *BaseVisitor) VisitStmt(stmt Stmt) {
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

func (v *BaseVisitor) VisitBadStmt(stmt *BadStmt) {
	panic("VisitBadStmt")
}

func (v *BaseVisitor) VisitBlockStmt(stmt *BlockStmt) {}

func (v *BaseVisitor) VisitDeclStmt(stmt *DeclStmt) {
	v.VisitDecl(stmt.D)
}

func (v *BaseVisitor) VisitPrintStmt(stmt *PrintStmt)   {}
func (v *BaseVisitor) VisitIfStmt(stmt *IfStmt)         {}
func (v *BaseVisitor) VisitWhileStmt(stmt *WhileStmt)   {}
func (v *BaseVisitor) VisitReturnStmt(stmt *ReturnStmt) {}
func (v *BaseVisitor) VisitBranchStmt(stmt *BranchStmt) {}
func (v *BaseVisitor) VisitAssignStmt(stmt *AssignStmt) {}
func (v *BaseVisitor) VisitExprStmt(stmt *ExprStmt)     {}

// VisitExpr switches on expr type and invokes corresponding Visit function.
func (v *BaseVisitor) VisitExpr(expr Expr) Expr {
	switch e := expr.(type) {
	case *BadExpr:
		return v.VisitBadExpr(e)
	case *BinaryExpr:
		return v.VisitBinaryExpr(e)
	case *UnaryExpr:
		return v.VisitUnaryExpr(e)
	case *Literal:
		return v.VisitLiteral(e)
	case *StructLiteral:
		return v.VisitStructLiteral(e)
	case *Ident:
		return v.VisitIdent(e)
	case *FuncCall:
		return v.VisitFuncCall(e)
	case *DotExpr:
		return v.VisitDotExpr(e)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", e))
	}
}

func (v *BaseVisitor) VisitBadExpr(decl *BadExpr) Expr {
	panic("VisitBadExpr")
}

func (v *BaseVisitor) VisitBinaryExpr(expr *BinaryExpr) Expr       { return nil }
func (v *BaseVisitor) VisitUnaryExpr(expr *UnaryExpr) Expr         { return nil }
func (v *BaseVisitor) VisitLiteral(expr *Literal) Expr             { return nil }
func (v *BaseVisitor) VisitStructLiteral(expr *StructLiteral) Expr { return nil }
func (v *BaseVisitor) VisitIdent(expr *Ident) Expr                 { return nil }
func (v *BaseVisitor) VisitFuncCall(expr *FuncCall) Expr           { return nil }
func (v *BaseVisitor) VisitDotExpr(expr *DotExpr) Expr             { return nil }

// VisitModuleList vists each module.
func VisitModuleList(v Visitor, modules []*Module) {
	for _, module := range modules {
		v.VisitModule(module)
	}
}

// VisitFileList vists each file.
func VisitFileList(v Visitor, files []*File) {
	for _, file := range files {
		v.VisitFile(file)
	}
}

// VisitDeclList Visits each decl
func VisitDeclList(v Visitor, decls []Decl) {
	for _, decl := range decls {
		v.VisitDecl(decl)
	}
}

// VisitImportList Visits each import.
func VisitImportList(v Visitor, decls []*Import) {
	for _, decl := range decls {
		v.VisitImport(decl)
	}
}

// VisitStmtList Visits each stmt.
func VisitStmtList(v Visitor, stmts []Stmt) {
	for _, stmt := range stmts {
		v.VisitStmt(stmt)
	}
}
