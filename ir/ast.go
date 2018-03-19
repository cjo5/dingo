package ir

import (
	"fmt"
	"math/big"

	"github.com/jhnl/dingo/token"
)

// AST flags.
const (
	AstFlagNoInit = 1 << 0
	AstFlagAnon   = 1 << 1
)

// Node interface.
type Node interface {
	Pos() token.Position
	node()
}

// Decl is the main interface for declaration nodes.
type Decl interface {
	Node
	declNode()
	Symbol() *Symbol
}

// TopDecl represents a top-level declaration.
type TopDecl interface {
	Decl
	SetColor(Color)
	Color() Color
	DependencyGraph() *DeclDependencyGraph
}

// Stmt is the main interface for statement nodes.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is the main interface for expression nodes.
type Expr interface {
	Node
	exprNode()
	EndPos() token.Position
	Type() Type
	Lvalue() bool
	ReadOnly() bool
}

type baseNode struct{}

func (n *baseNode) node() {}

// Declaration nodes.

type baseDecl struct {
	baseNode
	Sym *Symbol
}

func (d *baseDecl) declNode() {}

func (d *baseDecl) Symbol() *Symbol {
	return d.Sym
}

// Color is used to color nodes during dfs when sorting dependencies.
type Color int

// The node colors.
//
// White: node not visited
// Gray: node visit in progress
// Black: node visit finished
const (
	WhiteColor Color = iota
	GrayColor
	BlackColor
)

type DeclDependencyGraph map[TopDecl][]DeclDependencyEdge

type DeclDependencyEdge struct {
	Sym    *Symbol
	IsType bool
	Pos    token.Position
}

type Directive struct {
	Directive token.Token
	Name      *Ident
}

type baseTopDecl struct {
	baseDecl
	color      Color
	Deps       DeclDependencyGraph
	Directives []Directive
	Visibility token.Token
}

func (d *baseTopDecl) SetColor(color Color) {
	d.color = color
}

func (d *baseTopDecl) Color() Color {
	return d.color
}

func (d *baseTopDecl) DependencyGraph() *DeclDependencyGraph {
	return &d.Deps
}

type BadDecl struct {
	baseDecl
	From token.Token
	To   token.Token
}

func (d *BadDecl) Pos() token.Position { return d.From.Pos }

type File struct {
	baseNode
	Filename string
	Decl     token.Token
	ModName  Expr
	FileDeps []*FileDependency
}

type FileDependency struct {
	Decl    token.Token
	Tok     token.Token
	Literal string
}

type ValDeclSpec struct {
	Decl        token.Token
	Name        *Ident
	Type        Expr
	Assign      token.Token
	Initializer Expr
}

type ValTopDecl struct {
	baseTopDecl
	ValDeclSpec
}

func (d *ValTopDecl) Pos() token.Position {
	if d.Visibility.Pos.IsValid() {
		return d.Visibility.Pos
	}
	return d.Decl.Pos
}

type ValDecl struct {
	baseDecl
	ValDeclSpec
	Flags int
}

func (d *ValDecl) Pos() token.Position {
	return d.Decl.Pos
}

func (d *ValDecl) Init() bool {
	return (d.Flags & AstFlagNoInit) == 0
}

// FuncDecl represents a function (with body) or a function signature.
type FuncDecl struct {
	baseTopDecl
	Decl   token.Token
	ABI    *Ident
	Name   *Ident
	Lparen token.Token
	Params []*ValDecl
	Rparen token.Token
	Return *ValDecl
	Body   *BlockStmt
	Scope  *Scope
	Flags  int
}

func (d *FuncDecl) Pos() token.Position { return d.Decl.Pos }

func (d *FuncDecl) SignatureOnly() bool { return d.Body == nil }

// StructDecl represents a struct declaration.
type StructDecl struct {
	baseTopDecl
	Decl   token.Token
	Name   *Ident
	Lbrace token.Token
	Fields []*ValDecl
	Rbrace token.Token
	Scope  *Scope
}

func (d *StructDecl) Pos() token.Position { return d.Decl.Pos }

// Statement nodes.

type baseStmt struct {
	baseNode
}

func (s *baseStmt) stmtNode() {}

// BadStmt is a placeholder node for a statement that failed parsing.
type BadStmt struct {
	baseStmt
	From token.Token
	To   token.Token
}

func (s *BadStmt) Pos() token.Position { return s.From.Pos }

type BlockStmt struct {
	baseStmt
	Lbrace token.Token
	Scope  *Scope
	Stmts  []Stmt
	Rbrace token.Token
}

func (s *BlockStmt) Pos() token.Position { return s.Lbrace.Pos }

type DeclStmt struct {
	baseStmt
	D Decl
}

func (s *DeclStmt) Pos() token.Position { return s.D.Pos() }

// IfStmt represents a chain of if/elif/else statements.
type IfStmt struct {
	baseStmt
	If   token.Token
	Cond Expr
	Body *BlockStmt
	Else Stmt // Optional
}

func (s *IfStmt) Pos() token.Position { return s.If.Pos }

type ForStmt struct {
	baseStmt
	For   token.Token
	Scope *Scope
	Init  *ValDecl
	Inc   Stmt
	Cond  Expr
	Body  *BlockStmt
}

func (s *ForStmt) Pos() token.Position { return s.For.Pos }

type ReturnStmt struct {
	baseStmt
	Return token.Token
	X      Expr
}

func (s *ReturnStmt) Pos() token.Position { return s.Return.Pos }

type BranchStmt struct {
	baseStmt
	Tok token.Token
}

func (s *BranchStmt) Pos() token.Position { return s.Tok.Pos }

type AssignStmt struct {
	baseStmt
	Left   Expr
	Assign token.Token
	Right  Expr
}

func (s *AssignStmt) Pos() token.Position { return s.Left.Pos() }

type ExprStmt struct {
	baseStmt
	X Expr
}

func (s *ExprStmt) Pos() token.Position { return s.X.Pos() }

// Expression nodes.

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

func (x *baseExpr) Lvalue() bool {
	return false
}

func (x *baseExpr) ReadOnly() bool {
	return false
}

type BadExpr struct {
	baseExpr
	From token.Token
	To   token.Token
}

func (x *BadExpr) Pos() token.Position    { return x.From.Pos }
func (x *BadExpr) EndPos() token.Position { return x.To.Pos }

type PointerTypeExpr struct {
	baseExpr
	Pointer token.Token
	Decl    token.Token
	X       Expr
}

func (x *PointerTypeExpr) Pos() token.Position    { return x.Pointer.Pos }
func (x *PointerTypeExpr) EndPos() token.Position { return x.X.EndPos() }

type ArrayTypeExpr struct {
	baseExpr
	Lbrack token.Token
	Size   Expr
	X      Expr
	Rbrack token.Token
}

func (x *ArrayTypeExpr) Pos() token.Position    { return x.Lbrack.Pos }
func (x *ArrayTypeExpr) EndPos() token.Position { return x.Rbrack.Pos }

type FuncTypeExpr struct {
	baseExpr
	Fun    token.Token
	ABI    *Ident
	Lparen token.Token
	Params []*ValDecl
	Rparen token.Token
	Return *ValDecl
}

func (x *FuncTypeExpr) Pos() token.Position { return x.Fun.Pos }
func (x *FuncTypeExpr) EndPos() token.Position {
	pos := x.Return.Type.EndPos()
	if pos.IsValid() {
		return pos
	}
	return x.Rparen.Pos
}

type BinaryExpr struct {
	baseExpr
	Left  Expr
	Op    token.Token
	Right Expr
}

func (x *BinaryExpr) Pos() token.Position    { return x.Left.Pos() }
func (x *BinaryExpr) EndPos() token.Position { return x.Right.EndPos() }

type UnaryExpr struct {
	baseExpr
	Op token.Token
	X  Expr
}

func (x *UnaryExpr) Pos() token.Position    { return x.Op.Pos }
func (x *UnaryExpr) EndPos() token.Position { return x.X.EndPos() }

func (x *UnaryExpr) Lvalue() bool {
	switch x.Op.ID {
	case token.Mul:
		return x.X.Lvalue()
	}
	return false
}

func (x *UnaryExpr) ReadOnly() bool {
	switch x.Op.ID {
	case token.Mul:
		t := x.X.Type()
		if t != nil {
			if tptr, ok := t.(*PointerType); ok {
				return tptr.ReadOnly
			}
		}
	}
	return false
}

type AddressExpr struct {
	baseExpr
	And  token.Token
	Decl token.Token
	X    Expr
}

func (x *AddressExpr) Pos() token.Position    { return x.And.Pos }
func (x *AddressExpr) EndPos() token.Position { return x.X.EndPos() }

type IndexExpr struct {
	baseExpr
	X      Expr
	Lbrack token.Token
	Index  Expr
	Rbrack token.Token
}

func (x *IndexExpr) Pos() token.Position    { return x.X.Pos() }
func (x *IndexExpr) EndPos() token.Position { return x.Rbrack.Pos }

func (x *IndexExpr) Lvalue() bool {
	return x.X.Lvalue()
}

func (x *IndexExpr) ReadOnly() bool {
	t := x.X.Type()
	if t != nil {
		if tslice, ok := t.(*SliceType); ok {
			return tslice.ReadOnly
		}
	}
	return x.X.ReadOnly()
}

type SliceExpr struct {
	baseExpr
	X      Expr
	Lbrack token.Token
	Start  Expr
	Colon  token.Token
	End    Expr
	Rbrack token.Token
}

func (x *SliceExpr) Pos() token.Position    { return x.X.Pos() }
func (x *SliceExpr) EndPos() token.Position { return x.Rbrack.Pos }

func (x *SliceExpr) Lvalue() bool {
	return x.X.Lvalue()
}

func (x *SliceExpr) ReadOnly() bool {
	t := x.X.Type()
	if t != nil {
		if tslice, ok := t.(*SliceType); ok {
			return tslice.ReadOnly
		}
	}
	return x.X.ReadOnly()
}

type BasicLit struct {
	baseExpr
	Prefix  *Ident
	Tok     token.Token
	Value   string
	Suffix  *Ident
	Raw     interface{}
	Rewrite int
}

func (x *BasicLit) Pos() token.Position    { return x.Tok.Pos }
func (x *BasicLit) EndPos() token.Position { return x.Tok.Pos }

func (x *BasicLit) AsString() string {
	return x.Raw.(string)
}

func (x *BasicLit) AsU64() uint64 {
	bigInt := x.Raw.(*big.Int)
	return bigInt.Uint64()
}

func (x *BasicLit) NegatigeInteger() bool {
	bigInt := x.Raw.(*big.Int)
	return bigInt.Sign() < 0
}

func (x *BasicLit) Zero() bool {
	switch t := x.Raw.(type) {
	case *big.Int:
		return t.Uint64() == 0
	case *big.Float:
		val, _ := t.Float64()
		return val == 0
	}
	return false
}

func (x *BasicLit) AsF64() float64 {
	bigFloat := x.Raw.(*big.Float)
	val, _ := bigFloat.Float64()
	return val
}

type KeyValue struct {
	baseNode
	Key   *Ident
	Equal token.Token
	Value Expr
}

func (k *KeyValue) Pos() token.Position { return k.Key.Pos() }

type StructLit struct {
	baseExpr
	Name         Expr // Ident or DotIdent
	Lbrace       token.Token
	Initializers []*KeyValue
	Rbrace       token.Token
}

func (x *StructLit) Pos() token.Position    { return x.Name.Pos() }
func (x *StructLit) EndPos() token.Position { return x.Rbrace.Pos }

type ArrayLit struct {
	baseExpr
	Lbrack       token.Token
	Initializers []Expr
	Rbrack       token.Token
}

func (x *ArrayLit) Pos() token.Position    { return x.Lbrack.Pos }
func (x *ArrayLit) EndPos() token.Position { return x.Rbrack.Pos }

type Ident struct {
	baseExpr
	Tok     token.Token
	Literal string
	Sym     *Symbol
}

func NewIdent2(tok token.Token, literal string) *Ident {
	return &Ident{Tok: tok, Literal: literal}
}

func (x *Ident) Pos() token.Position    { return x.Tok.Pos }
func (x *Ident) EndPos() token.Position { return x.Tok.Pos }

func (x *Ident) Lvalue() bool {
	if x.Sym != nil && x.Sym.ID == ValSymbol {
		return true
	}
	return false
}

func (x *Ident) ReadOnly() bool {
	if x.Sym != nil {
		return x.Sym.ReadOnly()
	}
	return false
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

func (x *DotExpr) Pos() token.Position    { return x.X.Pos() }
func (x *DotExpr) EndPos() token.Position { return x.Name.EndPos() }

func (x *DotExpr) Lvalue() bool {
	return x.X.Lvalue()
}

func (x *DotExpr) ReadOnly() bool {
	return x.Name.ReadOnly() || x.X.ReadOnly()
}

type CastExpr struct {
	baseExpr
	Cast   token.Token
	Lparen token.Token
	ToTyp  Expr
	X      Expr
	Rparen token.Token
}

func (x *CastExpr) Pos() token.Position    { return x.Cast.Pos }
func (x *CastExpr) EndPos() token.Position { return x.Rparen.Pos }

type LenExpr struct {
	baseExpr
	Len    token.Token
	Lparen token.Token
	X      Expr
	Rparen token.Token
}

func (x *LenExpr) Pos() token.Position    { return x.Len.Pos }
func (x *LenExpr) EndPos() token.Position { return x.Rparen.Pos }

type FuncCall struct {
	baseExpr
	X      Expr
	Lparen token.Token
	Args   []Expr
	Rparen token.Token
}

func (x *FuncCall) Pos() token.Position    { return x.X.Pos() }
func (x *FuncCall) EndPos() token.Position { return x.Rparen.Pos }

func ExprToModuleFQN(expr Expr) string {
	switch t := expr.(type) {
	case *Ident:
		return t.Literal
	case *DotExpr:
		x := ExprToModuleFQN(t.X)
		if len(x) == 0 {
			return ""
		}
		return x + "." + t.Name.Literal
	default:
		return ""
	}
}

func TypeExprToIdent(expr Expr) *Ident {
	switch t := expr.(type) {
	case *Ident:
		return t
	case *PointerTypeExpr:
		return TypeExprToIdent(t.X)
	case *ArrayTypeExpr:
		return TypeExprToIdent(t.X)
	}
	return nil
}

func ExprSymbol(expr Expr) *Symbol {
	switch t := expr.(type) {
	case *Ident:
		return t.Sym
	case *DotExpr:
		return t.Name.Sym
	}
	return nil
}

// Lower number means higher precedence.

// LowestPrec is the initial precedence used in parsing.
const LowestPrec int = 100

// BinaryPrec returns the precedence for a binary operation.
func BinaryPrec(op token.ID) int {
	switch op {
	case token.Mul, token.Div, token.Mod:
		return 5
	case token.Add, token.Sub:
		return 6
	case token.Lt, token.LtEq, token.Gt, token.GtEq:
		return 9
	case token.Eq, token.Neq:
		return 10
	case token.Land:
		return 14
	case token.Lor:
		return 15
	default:
		panic(fmt.Sprintf("Unhandled binary op %s", op))
	}
}

// UnaryPrec returns the precedence for a unary operation.
func UnaryPrec(op token.ID) int {
	switch op {
	case token.Lnot, token.Sub, token.Mul, token.And:
		return 3
	default:
		panic(fmt.Sprintf("Unhandled unary op %s", op))
	}
}

// ExprPrec returns the precedence for an expression.
func ExprPrec(expr Expr) int {
	switch t := expr.(type) {
	case *BinaryExpr:
		return BinaryPrec(t.Op.ID)
	case *UnaryExpr:
		return UnaryPrec(t.Op.ID)
	case *AddressExpr:
		return UnaryPrec(t.And.ID)
	case *IndexExpr, *SliceExpr, *DotExpr, *CastExpr, *FuncCall:
		return 1
	case *BasicLit, *StructLit, *Ident:
		return 0
	default:
		panic(fmt.Sprintf("Unhandled expr %T", expr))
	}
}
