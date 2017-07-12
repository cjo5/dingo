package semantics

import "github.com/jhnl/interpreter/token"

// TODO:
// - Add TypeSpec as a Node interface to represent type annotations.
//   Right now types can only be represented as idents.
// - Replace Name Idents for Decls with simple tokens.
//

// NodeColor is used to color nodes during dfs to sort dependencies.
type NodeColor int

// The node colors.
//
// White: node not visited
// Gray: node visit in progress
// Black: node visit finished
const (
	NodeColorWhite NodeColor = iota
	NodeColorGray
	NodeColorBlack
)

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

type TopDecl interface {
	Decl
	Symbol() *Symbol

	SetContext(*FileContext)
	Context() *FileContext

	addDependency(TopDecl)
	dependencies() []TopDecl
	setNodeColor(NodeColor)
	nodeColor() NodeColor
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
}

func (d *baseDecl) declNode() {}

type baseTopDecl struct {
	baseDecl
	Sym *Symbol
	Ctx *FileContext

	deps  []TopDecl
	color NodeColor
}

func (d *baseTopDecl) Symbol() *Symbol {
	return d.Sym
}

func (d *baseTopDecl) SetContext(ctx *FileContext) {
	d.Ctx = ctx
}

func (d *baseTopDecl) Context() *FileContext {
	return d.Ctx
}

func (d *baseTopDecl) addDependency(dep TopDecl) {
	d.deps = append(d.deps, dep)
}

func (d *baseTopDecl) dependencies() []TopDecl {
	return d.deps
}

func (d *baseTopDecl) setNodeColor(color NodeColor) {
	d.color = color
}

func (d *baseTopDecl) nodeColor() NodeColor {
	return d.color
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
	baseDecl
	Path     string
	Name     token.Token
	External *Scope
	Internal *Scope
	Files    []*File
	Decls    []TopDecl
	color    NodeColor
}

func (m *Module) FirstPos() token.Position { return token.NoPosition }

type File struct {
	Ctx     *FileContext
	Imports []*Import
}

type FileContext struct {
	Path  string
	Decl  token.Token
	Scope *Scope
}

type Import struct {
	baseDecl
	Import  token.Token
	Literal token.Token
	Mod     *Module
}

func (i *Import) FirstPos() token.Position { return i.Import.Pos }

type ValDeclSpec struct {
	Decl        token.Token
	Name        token.Token
	Type        token.Token
	Assign      token.Token
	Initializer Expr
}

type ValTopDecl struct {
	baseTopDecl
	ValDeclSpec
	Visibility token.Token
}

func (d *ValTopDecl) FirstPos() token.Position {
	if d.Visibility.Pos.IsValid() {
		return d.Visibility.Pos
	}
	return d.Decl.Pos
}

type ValDecl struct {
	baseDecl
	ValDeclSpec
	Sym *Symbol
}

func (d *ValDecl) FirstPos() token.Position {
	return d.Decl.Pos
}

type FuncDecl struct {
	baseTopDecl
	Visibility token.Token
	Decl       token.Token
	Name       token.Token
	Lparen     token.Token
	Params     []*ValDecl
	Rparen     token.Token
	TReturn    *Ident
	Body       *BlockStmt
	Scope      *Scope
}

func (d *FuncDecl) FirstPos() token.Position { return d.Decl.Pos }

type StructDecl struct {
	baseTopDecl
	Visibility token.Token
	Decl       token.Token
	Name       token.Token
	Lbrace     token.Token
	Fields     []*ValDecl
	Rbrace     token.Token
	Scope      *Scope
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
}

func (x *Ident) FirstPos() token.Position { return x.Name.Pos }
func (x *Ident) Literal() string {
	return x.Name.Literal
}
func (x *Ident) Pos() token.Position {
	return x.Name.Pos
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
