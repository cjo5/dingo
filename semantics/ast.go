package semantics

import "github.com/jhnl/interpreter/token"

// Node interface.
type Node interface {
	FirstPos() token.Position
	node()
}

// Decl is the main interface for declaration nodes.
type Decl interface {
	Node
	declNode()
}

// Stmt is the main interface for statement nodes.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is the main interface for expression nodes.
type Expr interface {
	Node
	Type() *TType
	exprNode()
}

// A Module is the main unit of compilation.
type Module struct {
	Mod   token.Token
	Name  *Ident
	Scope *Scope
	Decls []Decl
}

// Field represents a function parameter.
type Field struct {
	Name *Ident
	Type *Ident
}

func (m *Module) FirstPos() token.Position { return m.Mod.Pos }
func (m *Module) node()                    {}
func (f *Field) FirstPos() token.Position  { return f.Name.FirstPos() }
func (f *Field) node()                     {}

// Decl nodes.

type BadDecl struct {
	From token.Token
	To   token.Token
}

type VarDecl struct {
	Decl   token.Token
	Name   *Ident
	Type   *Ident
	Assign token.Token
	X      Expr
}

type FuncDecl struct {
	Decl   token.Token
	Name   *Ident
	Scope  *Scope
	Params []*Field
	Return *Ident // Nil if no return value
	Body   *BlockStmt
}

// Implementation for decl nodes.

func (d *BadDecl) FirstPos() token.Position { return d.From.Pos }
func (d *BadDecl) node()                    {}
func (d *BadDecl) declNode()                {}

func (d *VarDecl) FirstPos() token.Position { return d.Decl.Pos }
func (d *VarDecl) node()                    {}
func (d *VarDecl) declNode()                {}

func (d *FuncDecl) FirstPos() token.Position { return d.Decl.Pos }
func (d *FuncDecl) node()                    {}
func (d *FuncDecl) declNode()                {}

// Stmt nodes.

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

type DeclStmt struct {
	D Decl
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

func (s *BadStmt) FirstPos() token.Position { return s.From.Pos }
func (s *BadStmt) node()                    {}
func (s *BadStmt) stmtNode()                {}

func (s *BlockStmt) FirstPos() token.Position { return s.Lbrace.Pos }
func (s *BlockStmt) node()                    {}
func (s *BlockStmt) stmtNode()                {}

func (s *DeclStmt) FirstPos() token.Position { return s.D.FirstPos() }
func (s *DeclStmt) node()                    {}
func (s *DeclStmt) stmtNode()                {}

func (s *PrintStmt) FirstPos() token.Position { return s.Print.Pos }
func (s *PrintStmt) node()                    {}
func (s *PrintStmt) stmtNode()                {}

func (s *IfStmt) FirstPos() token.Position { return s.If.Pos }
func (s *IfStmt) node()                    {}
func (s *IfStmt) stmtNode()                {}

func (s *WhileStmt) FirstPos() token.Position { return s.While.Pos }
func (s *WhileStmt) node()                    {}
func (s *WhileStmt) stmtNode()                {}

func (s *ReturnStmt) FirstPos() token.Position { return s.Return.Pos }
func (s *ReturnStmt) node()                    {}
func (s *ReturnStmt) stmtNode()                {}

func (s *BranchStmt) FirstPos() token.Position { return s.Tok.Pos }
func (s *BranchStmt) node()                    {}
func (s *BranchStmt) stmtNode()                {}

func (s *AssignStmt) FirstPos() token.Position { return s.Name.FirstPos() }
func (s *AssignStmt) node()                    {}
func (s *AssignStmt) stmtNode()                {}

func (s *ExprStmt) FirstPos() token.Position { return s.X.FirstPos() }
func (s *ExprStmt) node()                    {}
func (s *ExprStmt) stmtNode()                {}

// Expr nodes

type BadExpr struct {
	From token.Token
	To   token.Token
}

type BinaryExpr struct {
	Left  Expr
	Op    token.Token
	Right Expr
	T     *TType
}

type UnaryExpr struct {
	Op token.Token
	X  Expr
	T  *TType
}

type Literal struct {
	Value token.Token
	T     *TType
}

type Ident struct {
	Name token.Token
	Sym  *Symbol // TODO
}

func (x *Ident) Literal() string {
	return x.Name.Literal
}

type CallExpr struct {
	Name   *Ident
	Lparen token.Token
	Args   []Expr
	Rparen token.Token
	T      *TType
}

// Implementations for expr nodes.

func (x *BadExpr) FirstPos() token.Position { return x.From.Pos }
func (x *BadExpr) node()                    {}
func (x *BadExpr) Type() *TType             { return &TType{ID: TInvalid} }
func (x *BadExpr) exprNode()                {}

func (x *BinaryExpr) FirstPos() token.Position { return x.Left.FirstPos() }
func (x *BinaryExpr) node()                    {}
func (x *BinaryExpr) Type() *TType {
	if x.T == nil {
		return &TType{ID: TInvalid}
	}
	return x.T
}
func (x *BinaryExpr) exprNode() {}

func (x *UnaryExpr) FirstPos() token.Position { return x.Op.Pos }
func (x *UnaryExpr) node()                    {}
func (x *UnaryExpr) Type() *TType {
	if x.T == nil {
		return &TType{ID: TInvalid}
	}
	return x.T
}

func (x *UnaryExpr) exprNode() {}

func (x *Literal) FirstPos() token.Position { return x.Value.Pos }
func (x *Literal) node()                    {}
func (x *Literal) Type() *TType {
	if x.T == nil {
		return &TType{ID: TInvalid}
	}
	return x.T
}
func (x *Literal) exprNode() {}

func (x *Ident) FirstPos() token.Position { return x.Name.Pos }
func (x *Ident) node()                    {}
func (x *Ident) Type() *TType {
	if x.Sym == nil || x.Sym.T == nil {
		return &TType{ID: TInvalid}
	}
	return x.Sym.T
}
func (x *Ident) exprNode() {}

func (x *CallExpr) FirstPos() token.Position { return x.Name.FirstPos() }
func (x *CallExpr) node()                    {}
func (x *CallExpr) Type() *TType             { return x.T }
func (x *CallExpr) exprNode()                {}
