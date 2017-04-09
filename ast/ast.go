package ast

import (
	"github.com/jhnl/interpreter/token"
)

// Node interface.
type Node interface {
	First() token.Token
	Last() token.Token
	Accept(AstVisitor)
}

// AstVisitor interface.
type AstVisitor interface {
	visitBinary(expr *BinaryExpr)
	visitUnary(expr *UnaryExpr)
	visitLiteral(expr *Literal)
}

type Expr interface {
	Node
	exprNode()
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

type BadExpr struct {
	From token.Token
	To   token.Token
}

// First and Last implementations for nodes.

func (x *BinaryExpr) First() token.Token { return x.Left.First() }
func (x *BinaryExpr) Last() token.Token  { return x.Right.First() }

func (x *UnaryExpr) First() token.Token { return x.Op }
func (x *UnaryExpr) Last() token.Token  { return x.X.Last() }

func (x *Literal) First() token.Token { return x.Value }
func (x *Literal) Last() token.Token  { return x.Value }

func (x *BadExpr) First() token.Token { return x.From }
func (x *BadExpr) Last() token.Token  { return x.To }

// Accept implementations for nodes.

func (x *BinaryExpr) Accept(visitor AstVisitor) { visitor.visitBinary(x) }

func (x *UnaryExpr) Accept(visitor AstVisitor) { visitor.visitUnary(x) }

func (x *Literal) Accept(visitor AstVisitor) { visitor.visitLiteral(x) }

func (x *BadExpr) Accept(visitor AstVisitor) {}

func (x *BinaryExpr) exprNode() {}
func (x *UnaryExpr) exprNode()  {}
func (x *Literal) exprNode()    {}
func (x *BadExpr) exprNode()    {}
