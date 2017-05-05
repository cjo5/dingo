package ast

import "github.com/jhnl/interpreter/token"

// Node interface.
type Node interface {
	First() token.Token
	Last() token.Token
}

// Expr is the main interface for expression nodes.
type Expr interface {
	Node
	exprNode()
}

// Stmt is the main interface for statement nodes.
type Stmt interface {
	Node
	stmtNode()
}

// A Module is the main unit of compilation.
type Module struct {
	Mod   token.Token
	Name  *Ident
	Scope *Scope
	Stmts []Stmt
}

func (m *Module) First() token.Token {
	return m.Mod
}

func (m *Module) Last() token.Token {
	if n := len(m.Stmts); n > 0 {
		return m.Stmts[n-1].Last()
	}
	return m.Name.Last()
}

// Stmt nodes

type BadStmt struct {
	From token.Token
	To   token.Token
}

type BlockStmt struct {
	Lbrace token.Token
	Scope  *Scope
	Stmts  []Stmt
	Rbrace token.Token
}

type VarDecl struct {
	Decl   token.Token
	Name   *Ident
	Assign token.Token
	X      Expr
}

type FuncDecl struct {
	Decl   token.Token
	Name   *Ident
	Fields []*Ident
	Body   *BlockStmt
}

type PrintStmt struct {
	Print token.Token
	X     Expr
}

type IfStmt struct {
	If   token.Token
	Cond Expr
	Body *BlockStmt
	Else Stmt // Optional
}

type WhileStmt struct {
	While token.Token
	Cond  Expr
	Body  *BlockStmt
}

type BranchStmt struct {
	Tok token.Token
}

type AssignStmt struct {
	ID     *Ident
	Assign token.Token
	Right  Expr
}

// Implementation for stmt nodes.

func (s *BadStmt) First() token.Token { return s.From }
func (s *BadStmt) Last() token.Token  { return s.To }

func (s *BlockStmt) First() token.Token { return s.Lbrace }
func (s *BlockStmt) Last() token.Token  { return s.Rbrace }

func (s *VarDecl) First() token.Token { return s.Decl }
func (s *VarDecl) Last() token.Token  { return s.X.Last() }

func (s *FuncDecl) First() token.Token { return s.Decl }
func (s *FuncDecl) Last() token.Token  { return s.Body.Last() }

func (s *PrintStmt) First() token.Token { return s.Print }
func (s *PrintStmt) Last() token.Token  { return s.X.Last() }

func (s *IfStmt) First() token.Token { return s.If }
func (s *IfStmt) Last() token.Token {
	if s.Else != nil {
		return s.Else.Last()
	}
	return s.Body.Last()
}

func (s *WhileStmt) First() token.Token { return s.While }
func (s *WhileStmt) Last() token.Token  { return s.Body.Last() }

func (s *BranchStmt) First() token.Token { return s.Tok }
func (s *BranchStmt) Last() token.Token  { return s.Tok }

func (s *AssignStmt) First() token.Token { return s.ID.First() }
func (s *AssignStmt) Last() token.Token  { return s.Right.Last() }

func (s *BadStmt) stmtNode()    {}
func (s *BlockStmt) stmtNode()  {}
func (s *VarDecl) stmtNode()    {}
func (s *FuncDecl) stmtNode()   {}
func (s *PrintStmt) stmtNode()  {}
func (s *IfStmt) stmtNode()     {}
func (s *WhileStmt) stmtNode()  {}
func (s *BranchStmt) stmtNode() {}
func (s *AssignStmt) stmtNode() {}

// Expr nodes

type BadExpr struct {
	From token.Token
	To   token.Token
}

type BinaryExpr struct {
	Left  Expr
	Op    token.Token
	Right Expr
}

type UnaryExpr struct {
	Op token.Token
	X  Expr
}

type Literal struct {
	Value token.Token
}

type Ident struct {
	Name token.Token
}

// Implementations for expr nodes.

func (x *BadExpr) First() token.Token { return x.From }
func (x *BadExpr) Last() token.Token  { return x.To }

func (x *BinaryExpr) First() token.Token { return x.Left.First() }
func (x *BinaryExpr) Last() token.Token  { return x.Right.First() }

func (x *UnaryExpr) First() token.Token { return x.Op }
func (x *UnaryExpr) Last() token.Token  { return x.X.Last() }

func (x *Literal) First() token.Token { return x.Value }
func (x *Literal) Last() token.Token  { return x.Value }

func (x *Ident) First() token.Token { return x.Name }
func (x *Ident) Last() token.Token  { return x.Name }

func (x *BadExpr) exprNode()    {}
func (x *BinaryExpr) exprNode() {}
func (x *UnaryExpr) exprNode()  {}
func (x *Literal) exprNode()    {}
func (x *Ident) exprNode()      {}
