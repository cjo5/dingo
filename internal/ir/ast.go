package ir

import (
	"fmt"
	"math/big"

	"github.com/jhnl/dingo/internal/token"
)

// AST flags.
const (
	AstFlagNoInit = 1 << 0
	AstFlagAnon   = 1 << 1
	AstFlagPublic = 1 << 2
)

// Node interface.
type Node interface {
	node()
	Pos() token.Position
	SetPos(token.Position)
	EndPos() token.Position
	SetEndPos(token.Position)
	SetRange(token.Position, token.Position)
}

// Decl is the main interface for declaration nodes.
type Decl interface {
	Node
	declNode()
	Symbol() *Symbol
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
	Type() Type
	SetType(Type)
	Lvalue() bool
	ReadOnly() bool
}

type baseNode struct {
	firstPos token.Position
	lastPos  token.Position
}

func (n *baseNode) node() {}

func (n *baseNode) Pos() token.Position {
	return n.firstPos
}

func (n *baseNode) SetPos(pos token.Position) {
	n.firstPos = pos
}

func (n *baseNode) EndPos() token.Position {
	return n.lastPos
}

func (n *baseNode) SetEndPos(pos token.Position) {
	n.lastPos = pos
}

func (n *baseNode) SetRange(pos1 token.Position, pos2 token.Position) {
	n.firstPos = pos1
	n.lastPos = pos2
}

// Declaration nodes.

type baseDecl struct {
	baseNode
	Sym *Symbol
}

func (d *baseDecl) declNode() {}

func (d *baseDecl) Symbol() *Symbol {
	return d.Sym
}

type BadDecl struct {
	baseDecl
	From token.Token
	To   token.Token
}

type ImportDecl struct {
	baseDecl
	Decl  token.Token
	Alias *Ident
	Name  Expr
	Items []*ImportItem
}

type ImportItem struct {
	Visibilty token.Token
	Alias     *Ident
	Name      *Ident
}

type TypeDecl struct {
	baseDecl
	Decl token.Token
	Name *Ident
	Type Expr
}

type ValDecl struct {
	baseDecl
	Decl        token.Token
	Name        *Ident
	Type        Expr
	Initializer Expr
	Flags       int
}

func (d *ValDecl) DefaultInit() bool {
	return (d.Flags & AstFlagNoInit) == 0
}

// FuncDecl represents a function (with body) or a function signature.
type FuncDecl struct {
	baseDecl
	Name   *Ident
	Lparen token.Token
	Params []*ValDecl
	Rparen token.Token
	Return *ValDecl
	Body   *BlockStmt
	Scope  *Scope
	Flags  int
}

func (d *FuncDecl) SignatureOnly() bool { return d.Body == nil }

// StructDecl represents a struct declaration.
type StructDecl struct {
	baseDecl
	Name   *Ident
	Opaque bool
	Fields []*ValDecl
	Scope  *Scope
}

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

type BlockStmt struct {
	baseStmt
	Scope *Scope
	Stmts []Stmt
}

type DeclStmt struct {
	baseStmt
	D Decl
}

// IfStmt represents a chain of if/elif/else statements.
type IfStmt struct {
	baseStmt
	Tok  token.Token
	Cond Expr
	Body *BlockStmt
	Else Stmt // Optional
}

type ForStmt struct {
	baseStmt
	Tok  token.Token
	Init Stmt
	Inc  Stmt
	Cond Expr
	Body *BlockStmt
}

type ReturnStmt struct {
	baseStmt
	X Expr
}

type DeferStmt struct {
	baseStmt
	S Stmt
}

type BranchStmt struct {
	baseStmt
	Tok token.Token
}

type AssignStmt struct {
	baseStmt
	Left   Expr
	Assign token.Token
	Right  Expr
}

type ExprStmt struct {
	baseStmt
	X Expr
}

// Expression nodes.

type baseExpr struct {
	baseNode
	T Type
}

func (x *baseExpr) exprNode() {}

func (x *baseExpr) Type() Type {
	if x.T == nil {
		return TBuiltinUnknown
	}
	return x.T
}

func (x *baseExpr) SetType(t Type) {
	x.T = t
}

func (x *baseExpr) Lvalue() bool {
	return false
}

func (x *baseExpr) ReadOnly() bool {
	return false
}

type BadExpr struct {
	baseExpr
}

type PointerTypeExpr struct {
	baseExpr
	Decl token.Token
	X    Expr
}

type ArrayTypeExpr struct {
	baseExpr
	Size Expr
	X    Expr
}

type FuncTypeExpr struct {
	baseExpr
	ABI    *Ident
	Params []*ValDecl
	Return *ValDecl
}

type Ident struct {
	baseExpr
	Tok     token.Token
	Literal string
	Sym     *Symbol
}

func NewIdent1(tok token.Token) *Ident {
	return &Ident{Tok: tok, Literal: tok.String()}
}

func NewIdent2(tok token.Token, literal string) *Ident {
	return &Ident{Tok: tok, Literal: literal}
}

func (x *Ident) Lvalue() bool {
	if x.Sym != nil && x.Sym.Kind == ValSymbol {
		return true
	}
	return false
}

func (x *Ident) ReadOnly() bool {
	if x.Sym != nil {
		return x.Sym.IsReadOnly()
	}
	return false
}

type BasicLit struct {
	baseExpr
	Prefix Expr // Ident or DotExpr
	Suffix Expr // Ident or DotExpr
	Tok    token.Token
	Value  string
	Raw    interface{}
}

func NewStringLit(value string) *BasicLit {
	lit := &BasicLit{Tok: token.String}
	lit.Raw = value
	return lit
}

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

func (x *BasicLit) PositiveInteger() bool {
	bigInt := x.Raw.(*big.Int)
	return bigInt.Sign() > 0
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

type ArgExpr struct {
	baseNode
	Name  *Ident
	Value Expr
}

type StructLit struct {
	baseExpr
	Name Expr // Ident or DotExpr
	Args []*ArgExpr
}

type ArrayLit struct {
	baseExpr
	Elem         Expr
	Size         Expr
	Initializers []Expr
}

type BinaryExpr struct {
	baseExpr
	Left  Expr
	Op    token.Token
	Right Expr
}

type UnaryExpr struct {
	baseExpr
	Op   token.Token
	Decl token.Token // Used if op == token.Addr
	X    Expr
}

func (x *UnaryExpr) Lvalue() bool {
	switch x.Op {
	case token.Deref:
		return x.X.Lvalue()
	}
	return false
}

func (x *UnaryExpr) ReadOnly() bool {
	switch x.Op {
	case token.Deref:
		t := x.X.Type()
		if t != nil {
			if tptr, ok := t.(*PointerType); ok {
				return tptr.ReadOnly
			}
		}
	}
	return false
}

type DotExpr struct {
	baseExpr
	X    Expr
	Name *Ident
}

func (x *DotExpr) Lvalue() bool {
	return x.Name.Lvalue()
}

func (x *DotExpr) ReadOnly() bool {
	return x.Name.ReadOnly() || x.X.ReadOnly()
}

type IndexExpr struct {
	baseExpr
	X     Expr
	Index Expr
}

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
	X     Expr
	Start Expr
	End   Expr
}

func (x *SliceExpr) Lvalue() bool {
	return x.X.Lvalue()
}

func (x *SliceExpr) ReadOnly() bool {
	if t, ok := x.T.(*SliceType); ok {
		return t.ReadOnly
	}
	return false
}

type FuncCall struct {
	baseExpr
	X    Expr
	Args []*ArgExpr
}

type CastExpr struct {
	baseExpr
	ToType Expr
	X      Expr
}

type LenExpr struct {
	baseExpr
	X Expr
}

type SizeExpr struct {
	baseExpr
	X Expr
}

type ConstExpr struct {
	baseExpr
	X Expr
}

type DefaultInit struct {
	baseExpr
}

func NewDefaultInit(t Type) Expr {
	init := &DefaultInit{}
	init.T = t
	return init
}

func ExprToFQN(expr Expr) string {
	switch t := expr.(type) {
	case *Ident:
		return t.Literal
	case *DotExpr:
		x := ExprToFQN(t.X)
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
	case *DotExpr:
		return t.Name
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
func BinaryPrec(op token.Token) int {
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
func UnaryPrec(op token.Token) int {
	switch op {
	case token.Lnot, token.Sub, token.Deref, token.Addr:
		return 3
	default:
		panic(fmt.Sprintf("Unhandled unary op %s", op))
	}
}

// ExprPrec returns the precedence for an expression.
func ExprPrec(expr Expr) int {
	switch t := expr.(type) {
	case *BinaryExpr:
		return BinaryPrec(t.Op)
	case *UnaryExpr:
		return UnaryPrec(t.Op)
	case *IndexExpr, *SliceExpr, *DotExpr, *CastExpr, *FuncCall:
		return 1
	case *BasicLit, *StructLit, *Ident:
		return 0
	default:
		panic(fmt.Sprintf("Unhandled expr %T", expr))
	}
}
