package llvm

import (
	"fmt"

	"github.com/jhnl/interpreter/ir"
	"llvm.org/llvm/bindings/go/llvm"
)

type codeBuilder struct {
	b   llvm.Builder
	mod llvm.Module
}

// Build LLVM code.
func Build(set *ir.ModuleSet) {
	c := &codeBuilder{b: llvm.NewBuilder()}

	c.b = llvm.NewBuilder()

}

func (cb *codeBuilder) buildModule(mod *ir.Module) {
	cb.mod = llvm.NewModule(mod.Name.Literal)
}

func (cb *codeBuilder) buildValTopDecl(decl *ir.ValTopDecl) {
	cb.buildExpr(decl.Initializer)
}

func (cb *codeBuilder) buildValDecl(decl *ir.ValDecl) {

}

func (cb *codeBuilder) buildFuncDecl(decl *ir.FuncDecl) {

}

func (cb *codeBuilder) buildStructDecl(decl *ir.StructDecl) {

}

func (cb *codeBuilder) buildBlockStmt(stmt *ir.BlockStmt) {

}

func (cb *codeBuilder) buildDeclStmt(stmt *ir.DeclStmt) {

}

func (cb *codeBuilder) buildPrintStmt(stmt *ir.PrintStmt) {

}

func (cb *codeBuilder) buildIfStmt(stmt *ir.IfStmt) {

}

func (cb *codeBuilder) buildWhileStmt(stmt *ir.WhileStmt) {

}

func (cb *codeBuilder) buildReturnStmt(stmt *ir.ReturnStmt) {

}

func (cb *codeBuilder) buildBranchStmt(stmt *ir.BranchStmt) {

}
func (cb *codeBuilder) buildAssignStmt(stmt *ir.AssignStmt) {

}

func (cb *codeBuilder) buildExprStmt(stmt *ir.ExprStmt) {

}

func (cb *codeBuilder) buildExpr(expr ir.Expr) llvm.Value {
	switch t := expr.(type) {
	case *ir.BinaryExpr:
		return cb.buildBinaryExpr(t)
	case *ir.UnaryExpr:
		return cb.buildUnaryExpr(t)
	case *ir.BasicLit:
		return cb.buildBasicLit(t)
	case *ir.StructLit:
		return cb.buildStructLit(t)
	case *ir.Ident:
		return cb.buildIdent(t)
	case *ir.DotExpr:
		return cb.buildDotExpr(t)
	case *ir.FuncCall:
		return cb.buildFuncCall(t)
	default:
		panic(fmt.Sprintf("Unhandled expr %T", t))
	}
}

func (cb *codeBuilder) buildBinaryExpr(expr *ir.BinaryExpr) llvm.Value {
	return llvm.Value{}
}

func (cb *codeBuilder) buildUnaryExpr(expr *ir.UnaryExpr) llvm.Value {
	return llvm.Value{}
}

func (cb *codeBuilder) buildBasicLit(expr *ir.BasicLit) llvm.Value {
	return llvm.Value{}
}

func (cb *codeBuilder) buildStructLit(expr *ir.StructLit) llvm.Value {
	return llvm.Value{}
}

func (cb *codeBuilder) buildIdent(expr *ir.Ident) llvm.Value {
	return llvm.Value{}
}

func (cb *codeBuilder) buildDotExpr(expr *ir.DotExpr) llvm.Value {
	return llvm.Value{}
}

func (cb *codeBuilder) buildFuncCall(expr *ir.FuncCall) llvm.Value {
	return llvm.Value{}
}
