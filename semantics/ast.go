package semantics

import "github.com/jhnl/interpreter/token"

// TODO: Add TypeSpec as a Node interface to represent type annotations.
// Right now types can only be represented as idents.

// Node interface.
type Node interface {
	FirstPos() token.Position
	node()
}

// Decl is the main interface for declaration nodes.
type Decl interface {
	Node
	declNode()
	addDependency(symbol *Symbol)
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

type baseNode struct{}

func (n *baseNode) node() {}

// Decl nodes.

type baseDecl struct {
	baseNode
	dependencies []*Symbol
}

func (d *baseDecl) declNode() {}

func (d *baseDecl) addDependency(symbol *Symbol) {
	d.dependencies = append(d.dependencies, symbol)
}

type BadDecl struct {
	baseDecl
	From token.Token
	To   token.Token
}

func (d *BadDecl) FirstPos() token.Position { return d.From.Pos }

type Program struct {
	baseNode
	Modules []*Module
	Main    *Module // Main is also included in the Modules list
}

func (d *Program) FirstPos() token.Position { return token.NoPosition }

type Module struct {
	baseNode
	Path     string
	Name     token.Token
	Files    []*File
	External *Scope
	Internal *Scope
}

func (m *Module) FirstPos() token.Position { return token.NoPosition }

type File struct {
	baseNode
	Path    string
	Decl    token.Token
	Imports []*Import
	Decls   []Decl
	Scope   *Scope
}

func (m *File) FirstPos() token.Position { return token.NoPosition }

type Import struct {
	baseDecl
	Import  token.Token
	Literal token.Token
	Mod     *Module
}

func (i *Import) FirstPos() token.Position { return i.Import.Pos }

type VarDecl struct {
	baseDecl
	Visibility token.Token
	Decl       token.Token
	Name       *Ident
	Type       *Ident // Use as token.Token instead of Ident
	Assign     token.Token
	X          Expr
}

func (d *VarDecl) FirstPos() token.Position { return d.Decl.Pos }

// Field represents a function parameter.
type Field struct {
	baseNode
	Name *Ident
	Type *Ident
}

func (f *Field) FirstPos() token.Position { return f.Name.FirstPos() }

type FuncDecl struct {
	baseDecl
	Visibility token.Token
	Decl       token.Token
	Name       *Ident
	Lparen     token.Token
	Params     []*Field
	Rparen     token.Token
	Return     *Ident // Nil if no return value
	Body       *BlockStmt
	Scope      *Scope
}

func (d *FuncDecl) FirstPos() token.Position { return d.Decl.Pos }

// TODO: Move some fields to Field

type StructField struct {
	baseNode
	Qualifier   token.Token // ID == token.Invalid if missing
	Name        *Ident
	Type        Expr
	Initializer Expr // Nil if missing
}

func (f *StructField) FirstPos() token.Position { return f.Name.FirstPos() }

type StructDecl struct {
	baseDecl
	Decl   token.Token
	Name   *Ident
	Lbrace token.Token
	Fields []*StructField
	Rbrace token.Token
	Scope  *Scope
}

func (d *StructDecl) FirstPos() token.Position { return d.Decl.Pos }

// Stmt nodes.

type baseStmt struct {
	baseNode
}

func (s *baseStmt) stmtNode() {}

type BadStmt struct {
	baseStmt
	From token.Token
	To   token.Token
}

func (s *BadStmt) FirstPos() token.Position { return s.From.Pos }

type BlockStmt struct {
	baseStmt
	Lbrace token.Token
	Scope  *Scope
	Stmts  []Stmt
	Rbrace token.Token
}

func (s *BlockStmt) FirstPos() token.Position { return s.Lbrace.Pos }

type DeclStmt struct {
	baseStmt
	D Decl
}

func (s *DeclStmt) FirstPos() token.Position { return s.D.FirstPos() }

type PrintStmt struct {
	baseStmt
	Print token.Token
	X     Expr
}

func (s *PrintStmt) FirstPos() token.Position { return s.Print.Pos }

type IfStmt struct {
	baseStmt
	If   token.Token
	Cond Expr
	Body *BlockStmt
	Else Stmt // Optional
}

func (s *IfStmt) FirstPos() token.Position { return s.If.Pos }

type WhileStmt struct {
	baseStmt
	While token.Token
	Cond  Expr
	Body  *BlockStmt
}

func (s *WhileStmt) FirstPos() token.Position { return s.While.Pos }

type ReturnStmt struct {
	baseStmt
	Return token.Token
	X      Expr
}

func (s *ReturnStmt) FirstPos() token.Position { return s.Return.Pos }

type BranchStmt struct {
	baseStmt
	Tok token.Token
}

func (s *BranchStmt) FirstPos() token.Position { return s.Tok.Pos }

type AssignStmt struct {
	baseStmt
	Name   *Ident
	Assign token.Token
	Right  Expr
}

func (s *AssignStmt) FirstPos() token.Position { return s.Name.FirstPos() }

type ExprStmt struct {
	baseStmt
	X Expr
}

func (s *ExprStmt) FirstPos() token.Position { return s.X.FirstPos() }

// Expr nodes

type baseExpr struct {
	baseNode
	T *TType
}

func (x *baseExpr) exprNode() {}

func (x *baseExpr) Type() *TType {
	if x.T == nil {
		return TBuiltinUntyped
	}
	return x.T
}

type BadExpr struct {
	baseExpr
	From token.Token
	To   token.Token
}

func (x *BadExpr) FirstPos() token.Position { return x.From.Pos }

type BinaryExpr struct {
	baseExpr
	Left  Expr
	Op    token.Token
	Right Expr
}

func (x *BinaryExpr) FirstPos() token.Position { return x.Left.FirstPos() }

type UnaryExpr struct {
	baseExpr
	Op token.Token
	X  Expr
}

func (x *UnaryExpr) FirstPos() token.Position { return x.Op.Pos }

type Literal struct {
	baseExpr
	Value   token.Token
	Raw     interface{}
	Rewrite int
}

func (x *Literal) FirstPos() token.Position { return x.Value.Pos }

func (x *Ident) Literal() string {
	return x.Name.Literal
}

type KeyValue struct {
	baseNode
	Key   *Ident
	Equal token.Token
	Value Expr
}

func (k *KeyValue) FirstPos() token.Position { return k.Key.FirstPos() }

type StructLiteral struct {
	baseExpr
	Name         *Ident // Nil if anonymous
	Lbrace       token.Token
	Initializers []*KeyValue
	Rbrace       token.Token
}

func (x *StructLiteral) FirstPos() token.Position { return x.Name.FirstPos() }

type Ident struct {
	baseExpr
	Name token.Token
	Sym  *Symbol
}

func (x *Ident) FirstPos() token.Position { return x.Name.Pos }

func (x *Ident) Type() *TType {
	if x.Sym == nil || x.Sym.T == nil {
		return TBuiltinUntyped
	}
	return x.Sym.T
}

type FuncCall struct {
	baseExpr
	Name   *Ident
	Lparen token.Token
	Args   []Expr
	Rparen token.Token
}

func (x *FuncCall) FirstPos() token.Position { return x.Name.FirstPos() }

type DotExpr struct {
	baseExpr
	Name *Ident
	Dot  token.Token
	X    Expr
}

func (x *DotExpr) FirstPos() token.Position { return x.Name.FirstPos() }
