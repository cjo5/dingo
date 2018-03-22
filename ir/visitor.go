package ir

import "fmt"

// Visitor interface.
type Visitor interface {
	Module(mod *Module)

	// Decls
	VisitBadDecl(decl *BadDecl)
	VisitValTopDecl(decl *ValTopDecl)
	VisitValDecl(decl *ValDecl)
	VisitFuncDecl(decl *FuncDecl)
	VisitStructDecl(decl *StructDecl)

	// Stmts
	VisitBadStmt(stmt *BadStmt)
	VisitBlockStmt(stmt *BlockStmt)
	VisitDeclStmt(stmt *DeclStmt)
	VisitIfStmt(stmt *IfStmt)
	VisitForStmt(stmt *ForStmt)
	VisitReturnStmt(stmt *ReturnStmt)
	VisitBranchStmt(stmt *BranchStmt)
	VisitAssignStmt(stmt *AssignStmt)
	VisitExprStmt(stmt *ExprStmt)

	// Exprs
	VisitBadExpr(expr *BadExpr) Expr
	VisitPointerTypeExpr(expr *PointerTypeExpr) Expr
	VisitArrayTypeExpr(expr *ArrayTypeExpr) Expr
	VisitFuncTypeExpr(expr *FuncTypeExpr) Expr
	VisitBinaryExpr(expr *BinaryExpr) Expr
	VisitUnaryExpr(expr *UnaryExpr) Expr
	VisitBasicLit(expr *BasicLit) Expr
	VisitStructLit(expr *StructLit) Expr
	VisitArrayLit(expr *ArrayLit) Expr
	VisitIdent(expr *Ident) Expr
	VisitDotExpr(expr *DotExpr) Expr
	VisitCastExpr(expr *CastExpr) Expr
	VisitLenExpr(expr *LenExpr) Expr
	VisitFuncCall(expr *FuncCall) Expr
	VisitAddressExpr(expr *AddressExpr) Expr
	VisitIndexExpr(expr *IndexExpr) Expr
	VisitSliceExpr(expr *SliceExpr) Expr
}

// BaseVisitor provides default implementations for Visitor functions.
type BaseVisitor struct{}

func (v *BaseVisitor) Module(mod *Module) {
	panic("Module")
}

func (v *BaseVisitor) VisitBadDecl(decl *BadDecl) {
	panic("VisitBadDecl")
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

func (v *BaseVisitor) VisitIfStmt(stmt *IfStmt)         {}
func (v *BaseVisitor) VisitForStmt(stmt *ForStmt)       {}
func (v *BaseVisitor) VisitReturnStmt(stmt *ReturnStmt) {}
func (v *BaseVisitor) VisitBranchStmt(stmt *BranchStmt) {}
func (v *BaseVisitor) VisitAssignStmt(stmt *AssignStmt) {}
func (v *BaseVisitor) VisitExprStmt(stmt *ExprStmt)     {}

func (v *BaseVisitor) VisitBadExpr(decl *BadExpr) Expr {
	panic("VisitBadExpr")
}

func (v *BaseVisitor) VisitPointerTypeExpr(expr *PointerTypeExpr) Expr { return expr }
func (v *BaseVisitor) VisitArrayTypeExpr(expr *ArrayTypeExpr) Expr     { return expr }
func (v *BaseVisitor) VisitFuncTypeExpr(expr *FuncTypeExpr) Expr       { return expr }
func (v *BaseVisitor) VisitBinaryExpr(expr *BinaryExpr) Expr           { return expr }
func (v *BaseVisitor) VisitUnaryExpr(expr *UnaryExpr) Expr             { return expr }
func (v *BaseVisitor) VisitBasicLit(expr *BasicLit) Expr               { return expr }
func (v *BaseVisitor) VisitStructLit(expr *StructLit) Expr             { return expr }
func (v *BaseVisitor) VisitArrayLit(expr *ArrayLit) Expr               { return expr }
func (v *BaseVisitor) VisitIdent(expr *Ident) Expr                     { return expr }
func (v *BaseVisitor) VisitDotExpr(expr *DotExpr) Expr                 { return expr }
func (v *BaseVisitor) VisitCastExpr(expr *CastExpr) Expr               { return expr }
func (v *BaseVisitor) VisitLenExpr(expr *LenExpr) Expr                 { return expr }
func (v *BaseVisitor) VisitFuncCall(expr *FuncCall) Expr               { return expr }
func (v *BaseVisitor) VisitAddressExpr(expr *AddressExpr) Expr         { return expr }
func (v *BaseVisitor) VisitIndexExpr(expr *IndexExpr) Expr             { return expr }
func (v *BaseVisitor) VisitSliceExpr(expr *SliceExpr) Expr             { return expr }

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
	case *IfStmt:
		v.VisitIfStmt(s)
	case *ForStmt:
		v.VisitForStmt(s)
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
	case *PointerTypeExpr:
		return v.VisitPointerTypeExpr(e)
	case *ArrayTypeExpr:
		return v.VisitArrayTypeExpr(e)
	case *FuncTypeExpr:
		return v.VisitFuncTypeExpr(e)
	case *BinaryExpr:
		return v.VisitBinaryExpr(e)
	case *UnaryExpr:
		return v.VisitUnaryExpr(e)
	case *BasicLit:
		return v.VisitBasicLit(e)
	case *StructLit:
		return v.VisitStructLit(e)
	case *ArrayLit:
		return v.VisitArrayLit(e)
	case *Ident:
		return v.VisitIdent(e)
	case *DotExpr:
		return v.VisitDotExpr(e)
	case *CastExpr:
		return v.VisitCastExpr(e)
	case *LenExpr:
		return v.VisitLenExpr(e)
	case *FuncCall:
		return v.VisitFuncCall(e)
	case *AddressExpr:
		return v.VisitAddressExpr(e)
	case *IndexExpr:
		return v.VisitIndexExpr(e)
	case *SliceExpr:
		return v.VisitSliceExpr(e)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", e))
	}
}

// VisitModuleSet will visit each module.
func VisitModuleSet(v Visitor, set *ModuleSet) {
	for _, mod := range set.Modules {
		v.Module(mod)
	}
}

// VisitDeclList visits each decl
func VisitDeclList(v Visitor, decls []Decl) {
	for _, decl := range decls {
		VisitDecl(v, decl)
	}
}

// VisitTopDeclList visits each top decl.
func VisitTopDeclList(v Visitor, decls []TopDecl) {
	for _, decl := range decls {
		VisitDecl(v, decl)
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
