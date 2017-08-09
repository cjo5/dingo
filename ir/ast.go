package ir

import "github.com/jhnl/interpreter/token"
import "fmt"

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

	AddDependency(TopDecl)
	Dependencies() []TopDecl
	SetNodeColor(NodeColor)
	NodeColor() NodeColor
}

// Stmt is the main interface for statement nodes.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is the main interface for expression nodes.
type Expr interface {
	Node
	Type() Type
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

	Deps  []TopDecl // Dependencies don't cross module boundaries
	Color NodeColor
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

func (d *baseTopDecl) AddDependency(dep TopDecl) {
	d.Deps = append(d.Deps, dep)
}

func (d *baseTopDecl) Dependencies() []TopDecl {
	return d.Deps
}

func (d *baseTopDecl) SetNodeColor(color NodeColor) {
	d.Color = color
}

func (d *baseTopDecl) NodeColor() NodeColor {
	return d.Color
}

type BadDecl struct {
	baseDecl
	From token.Token
	To   token.Token
}

func (d *BadDecl) FirstPos() token.Position { return d.From.Pos }

type ModuleSet struct {
	baseNode
	Modules []*Module
}

func (d *ModuleSet) FirstPos() token.Position { return token.NoPosition }

// Module flags.
const (
	ModFlagMain = 1 << 0
)

type Module struct {
	baseDecl
	ID       int
	Flags    int
	Path     string
	Name     token.Token
	Public   *Scope
	Internal *Scope
	Files    []*File
	Decls    []TopDecl
	Color    NodeColor
}

func (m *Module) FirstPos() token.Position { return token.NoPosition }

func (m *Module) Main() bool {
	return (m.Flags & ModFlagMain) != 0
}

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
	Type        Expr
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

// Val decl flags.
const (
	ValFlagNoInit = 1 << 0
)

type ValDecl struct {
	baseDecl
	ValDeclSpec
	Sym   *Symbol
	Flags int
}

func (d *ValDecl) FirstPos() token.Position {
	return d.Decl.Pos
}

func (d *ValDecl) Init() bool {
	return (d.Flags & ValFlagNoInit) == 0
}

type FuncDecl struct {
	baseTopDecl
	Visibility token.Token
	Decl       token.Token
	Name       token.Token
	Lparen     token.Token
	Params     []*ValDecl
	Rparen     token.Token
	TReturn    Expr
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
	Xs    []Expr
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
	Left   Expr
	Assign token.Token
	Right  Expr
}

func (s *AssignStmt) FirstPos() token.Position { return s.Left.FirstPos() }

type ExprStmt struct {
	baseStmt
	X Expr
}

func (s *ExprStmt) FirstPos() token.Position { return s.X.FirstPos() }

// Expr nodes

type baseExpr struct {
	baseNode
	T Type
}

func (x *baseExpr) exprNode() {}

func (x *baseExpr) Type() Type {
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

type BasicLit struct {
	baseExpr
	Value   token.Token
	Raw     interface{}
	Rewrite int
}

func (x *BasicLit) FirstPos() token.Position { return x.Value.Pos }

type KeyValue struct {
	baseNode
	Key   token.Token
	Equal token.Token
	Value Expr
}

func (k *KeyValue) FirstPos() token.Position { return k.Key.Pos }

type StructLit struct {
	baseExpr
	Name         Expr // Ident or DotIdent
	Lbrace       token.Token
	Initializers []*KeyValue
	Rbrace       token.Token
}

func (x *StructLit) FirstPos() token.Position { return x.Name.FirstPos() }

type Ident struct {
	baseExpr
	Name token.Token
	Sym  *Symbol
}

func (x *Ident) FirstPos() token.Position { return x.Name.Pos }

func (x *Ident) Literal() string {
	return x.Name.Literal
}
func (x *Ident) Pos() token.Position {
	return x.Name.Pos
}

func (x *Ident) SetSymbol(sym *Symbol) {
	x.Sym = sym
	if sym != nil {
		x.T = sym.T
	}
}

type DotExpr struct {
	baseExpr
	X    Expr
	Dot  token.Token
	Name *Ident
}

func (x *DotExpr) FirstPos() token.Position { return x.X.FirstPos() }

type FuncCall struct {
	baseExpr
	X      Expr
	Lparen token.Token
	Args   []Expr
	Rparen token.Token
}

func (x *FuncCall) FirstPos() token.Position { return x.X.FirstPos() }

func ExprToIdent(expr Expr) *Ident {
	switch t := expr.(type) {
	case *Ident:
		return t
	case *DotExpr:
		return t.Name
	}
	return nil
}

func ExprSymbol(expr Expr) *Symbol {
	if id := ExprToIdent(expr); id != nil {
		return id.Sym
	}
	return nil
}

func CopyExpr(expr Expr, includePositions bool) Expr {
	switch src := expr.(type) {
	case *Ident:
		dst := &Ident{}
		dst.Name = src.Name
		if !includePositions {
			dst.Name.Pos = token.NoPosition
		}
		dst.SetSymbol(src.Sym)
		return dst
	case *DotExpr:
		dst := &DotExpr{}
		dst.X = CopyExpr(src.X, includePositions)
		dst.Name = CopyExpr(src.Name, includePositions).(*Ident)
		dst.T = src.T
		return dst
	default:
		panic(fmt.Sprintf("Unhandled CopyExpr src %T", src))
	}
}
