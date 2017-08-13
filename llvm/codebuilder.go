package llvm

import (
	"fmt"

	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
	"llvm.org/llvm/bindings/go/llvm"
)

type codeBuilder struct {
	b      llvm.Builder
	mod    llvm.Module
	values map[*ir.Symbol]llvm.Value
}

// Build LLVM code.
func Build(set *ir.ModuleSet) {
	cb := &codeBuilder{b: llvm.NewBuilder()}

	cb.b = llvm.NewBuilder()
	for _, mod := range set.Modules {
		cb.buildModule(mod)
	}
}

func (cb *codeBuilder) buildModule(mod *ir.Module) {
	cb.mod = llvm.NewModule(mod.Name.Literal)

	for _, decl := range mod.Decls {
		cb.buildDecl(decl)
	}

	cb.mod.Dump()
}

func (cb *codeBuilder) buildDecl(decl ir.Decl) {
	switch t := decl.(type) {
	case *ir.ValTopDecl:
		cb.buildValTopDecl(t)
	case *ir.ValDecl:
		cb.buildValDecl(t)
	case *ir.FuncDecl:
		cb.buildFuncDecl(t)
	case *ir.StructDecl:
		cb.buildStructDecl(t)
	default:
		panic(fmt.Sprintf("Unhandled decl %T", decl))
	}
}

func (cb *codeBuilder) buildValTopDecl(decl *ir.ValTopDecl) {
	cb.buildExpr(decl.Initializer)
}

func (cb *codeBuilder) buildValDecl(decl *ir.ValDecl) {

}

func (cb *codeBuilder) buildFuncDecl(decl *ir.FuncDecl) {
	funType := llvm.FunctionType(llvm.VoidType(), nil, false)
	fun := llvm.AddFunction(cb.mod, decl.Name.Literal, funType)
	block := llvm.AddBasicBlock(fun, "entry")
	cb.b.SetInsertPointAtEnd(block)

	cb.buildBlockStmt(decl.Body)
}

func (cb *codeBuilder) buildStructDecl(decl *ir.StructDecl) {

}

func (cb *codeBuilder) buildStmtList(stmts []ir.Stmt) {
	for _, stmt := range stmts {
		cb.buildStmt(stmt)
	}
}

func (cb *codeBuilder) buildStmt(stmt ir.Stmt) {
	switch t := stmt.(type) {
	case *ir.BlockStmt:
		cb.buildBlockStmt(t)
	case *ir.DeclStmt:
		cb.buildDeclStmt(t)
	case *ir.PrintStmt:
		cb.buildPrintStmt(t)
	case *ir.IfStmt:
		cb.buildIfStmt(t)
	case *ir.WhileStmt:
		cb.buildWhileStmt(t)
	case *ir.ReturnStmt:
		cb.buildReturnStmt(t)
	case *ir.BranchStmt:
		cb.buildBranchStmt(t)
	case *ir.AssignStmt:
		cb.buildAssignStmt(t)
	case *ir.ExprStmt:
		cb.buildExprStmt(t)
	default:
		panic(fmt.Sprintf("Unhandled stmt %T", t))
	}
}

func (cb *codeBuilder) buildBlockStmt(stmt *ir.BlockStmt) {
	cb.buildStmtList(stmt.Stmts)
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
	cb.buildExpr(stmt.X)
}

func toLLVMType(t ir.Type) llvm.Type {
	switch t.ID() {
	case ir.TVoid:
		return llvm.VoidType()
	case ir.TBool:
		return llvm.IntType(1)
	case ir.TUInt64, ir.TInt64:
		return llvm.IntType(64)
	case ir.TUInt32, ir.TInt32:
		return llvm.IntType(32)
	case ir.TUInt16, ir.TInt16:
		return llvm.IntType(16)
	case ir.TUInt8, ir.TInt8:
		return llvm.IntType(8)
	default:
		panic(fmt.Sprintf("Unhandled type %s", t.ID()))
	}
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
	left := cb.buildExpr(expr.Left)
	right := cb.buildExpr(expr.Right)

	switch expr.Op.ID {
	case token.Add:
		if ir.IsFloatingType(expr.T) {
			return cb.b.CreateFAdd(left, right, "")
		}
		return cb.b.CreateAdd(left, right, "")
	case token.Sub:
	case token.Mul:
	case token.Div:
	case token.Mod:
	}

	return llvm.Value{}
}

func (cb *codeBuilder) buildUnaryExpr(expr *ir.UnaryExpr) llvm.Value {
	return llvm.Value{}
}

func (cb *codeBuilder) buildBasicLit(expr *ir.BasicLit) llvm.Value {
	llvmType := toLLVMType(expr.T)

	fmt.Println("BASIC")

	if expr.Value.ID == token.Integer {
		return llvm.ConstInt(llvmType, expr.AsU64(), false)
	} else if expr.Value.ID == token.Float {
		return llvm.ConstFloat(llvmType, expr.AsF64())
	} else if expr.Value.ID == token.True {
		return llvm.ConstInt(llvmType, 1, false)
	} else if expr.Value.ID == token.False {
		return llvm.ConstInt(llvmType, 0, false)
	}

	// TODO: Handle strings
	panic(fmt.Sprintf("Unhandled basic lit %s", expr.Value.ID))
}

func (cb *codeBuilder) buildStructLit(expr *ir.StructLit) llvm.Value {
	return llvm.Value{}
}

func (cb *codeBuilder) buildIdent(expr *ir.Ident) llvm.Value {
	if val, ok := cb.values[expr.Sym]; ok {
		return val
	}
	panic(fmt.Sprintf("%s not found", expr.Sym))
}

func (cb *codeBuilder) buildDotExpr(expr *ir.DotExpr) llvm.Value {
	return llvm.Value{}
}

func (cb *codeBuilder) buildFuncCall(expr *ir.FuncCall) llvm.Value {
	return llvm.Value{}
}
