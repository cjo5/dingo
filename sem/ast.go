package sem

import "github.com/jhnl/interpreter/token"

// Node interface.
type Node interface {
	node()
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

func (m *Module) node() {}

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
	Scope  *Scope
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

type ReturnStmt struct {
	Return token.Token
	X      Expr
}

type BranchStmt struct {
	Tok token.Token
}

type AssignStmt struct {
	Name   *Ident
	Assign token.Token
	Right  Expr
}

type ExprStmt struct {
	X Expr
}

// Implementation for stmt nodes.

func (s *BadStmt) node()    {}
func (s *BlockStmt) node()  {}
func (s *VarDecl) node()    {}
func (s *FuncDecl) node()   {}
func (s *PrintStmt) node()  {}
func (s *IfStmt) node()     {}
func (s *WhileStmt) node()  {}
func (s *ReturnStmt) node() {}
func (s *BranchStmt) node() {}
func (s *AssignStmt) node() {}
func (s *ExprStmt) node()   {}

func (s *BadStmt) stmtNode()    {}
func (s *BlockStmt) stmtNode()  {}
func (s *VarDecl) stmtNode()    {}
func (s *FuncDecl) stmtNode()   {}
func (s *PrintStmt) stmtNode()  {}
func (s *IfStmt) stmtNode()     {}
func (s *WhileStmt) stmtNode()  {}
func (s *ReturnStmt) stmtNode() {}
func (s *BranchStmt) stmtNode() {}
func (s *AssignStmt) stmtNode() {}
func (s *ExprStmt) stmtNode()   {}

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

func (x *Ident) Literal() string {
	return x.Name.Literal
}

type CallExpr struct {
	Name   *Ident
	Lparen token.Token
	Args   []Expr
	Rparen token.Token
}

// Implementations for expr nodes.
func (x *BadExpr) node()    {}
func (x *BinaryExpr) node() {}
func (x *UnaryExpr) node()  {}
func (x *Literal) node()    {}
func (x *Ident) node()      {}
func (x *CallExpr) node()   {}

func (x *BadExpr) exprNode()    {}
func (x *BinaryExpr) exprNode() {}
func (x *UnaryExpr) exprNode()  {}
func (x *Literal) exprNode()    {}
func (x *Ident) exprNode()      {}
func (x *CallExpr) exprNode()   {}
