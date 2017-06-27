package parser

import (
	"fmt"
	"io/ioutil"

	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/semantics"
	"github.com/jhnl/interpreter/token"
)

func ParseFile(filename string) (*semantics.Module, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return parse(buf, filename)
}

func Parse(src []byte) (*semantics.Module, error) {
	return parse(src, "")
}

func parse(src []byte, filename string) (*semantics.Module, error) {
	var p parser
	p.init(src, filename)
	p.next()

	mod := p.parseModule()

	if len(p.errors) > 0 {
		return nil, p.errors
	}

	return mod, nil
}

type parser struct {
	scanner scanner
	errors  common.ErrorList
	trace   bool

	token  token.Token
	inLoop bool
}

func (p *parser) init(src []byte, filename string) {
	p.trace = false
	p.scanner.init(src, filename, &p.errors)
}

func (p *parser) next() {
	if p.trace && p.token.IsValid() {
		fmt.Println(p.token)
	}

	for {
		p.token = p.scanner.scan()
		if p.token.ID != token.Comment && p.token.ID != token.MultiComment {
			break
		}
	}
}

func (p *parser) error(tok token.Token, format string, args ...interface{}) {
	p.errors.Add(tok.Pos, format, args...)
}

func (p *parser) sync() {
loop:
	for {
		switch p.token.ID {
		case token.Semicolon: // TODO: Remove this?
			break loop
		case token.Var, token.Print, token.If, token.Break, token.Continue:
			return
		case token.EOF:
			return
		}
		p.next()
	}
	p.next()
}

func (p *parser) match(id token.ID) bool {
	if p.token.ID != id {
		return false
	}
	p.next()
	return true
}

func (p *parser) expect(id token.ID) bool {
	if !p.match(id) {
		p.error(p.token, "got '%s', expected '%s'", p.token.ID, id)
		p.next()
		return false
	}
	return true
}

func (p *parser) expectSemi() bool {
	res := true
	if !p.expect(token.Semicolon) {
		p.sync()
		res = false
	} else {
		// Consume empty statements
		for p.token.ID == token.Semicolon {
			p.next()
		}
	}
	return res
}

func (p *parser) parseModule() *semantics.Module {
	mod := &semantics.Module{}
	if p.token.ID == token.Module {
		mod.Mod = p.token
		p.expect(token.Module)
		mod.Name = p.parseIdent()
		p.expectSemi()
	}
	for p.token.ID != token.EOF {
		mod.Decls = append(mod.Decls, p.parseDecl())
	}
	return mod
}

func (p *parser) parseDecl() semantics.Decl {
	if p.token.ID == token.Var || p.token.ID == token.Let {
		return p.parseVarDecl()
	} else if p.token.ID == token.Func {
		return p.parseFuncDecl()
	}
	tok := p.token
	p.next()
	p.error(tok, "got '%s', expected declaration", tok.ID)
	return &semantics.BadDecl{From: tok, To: tok}
}

func (p *parser) parseVarDecl() *semantics.VarDecl {
	decl := &semantics.VarDecl{}
	decl.Decl = p.token
	p.next()
	decl.Name = p.parseIdent()
	p.expect(token.Colon)
	decl.Type = p.parseIdent()

	if p.token.ID == token.Assign {
		p.expect(token.Assign)
		decl.X = p.parseExpr()
	}

	p.expect(token.Semicolon)
	return decl
}

func (p *parser) parseFuncDecl() *semantics.FuncDecl {
	decl := &semantics.FuncDecl{}
	decl.Decl = p.token
	p.next()
	decl.Name = p.parseIdent()

	p.expect(token.Lparen)
	if p.token.ID == token.Ident {
		field := p.parseField()
		decl.Params = append(decl.Params, field)
		for p.token.ID != token.EOF && p.token.ID != token.Rparen {
			p.expect(token.Comma)
			field = p.parseField()
			decl.Params = append(decl.Params, field)
		}
	}
	p.expect(token.Rparen)

	if p.token.ID == token.Arrow {
		p.next()
		decl.Return = p.parseIdent()
	} else {
		decl.Return = &semantics.Ident{Name: token.Synthetic(token.Ident, semantics.TVoid.String())}
	}

	decl.Body = p.parseBlockStmt()
	return decl
}

func (p *parser) parseField() *semantics.Field {
	field := &semantics.Field{}
	field.Name = p.parseIdent()
	p.expect(token.Colon)
	field.Type = p.parseIdent()
	return field
}

func (p *parser) parseStmt() semantics.Stmt {
	if p.token.ID == token.Lbrace {
		return p.parseBlockStmt()
	}
	if p.token.ID == token.Var || p.token.ID == token.Let {
		return p.parseDeclStmt()
	}
	if p.token.ID == token.Print {
		return p.parsePrintStmt()
	}
	if p.token.ID == token.If {
		return p.parseIfStmt()
	}
	if p.token.ID == token.While {
		return p.parseWhileStmt()
	}
	if p.token.ID == token.Return {
		return p.parseReturnStmt()
	}
	if p.token.ID == token.Break || p.token.ID == token.Continue {
		if !p.inLoop {
			p.error(p.token, "%s can only be used in a loop", p.token.ID)
		}

		tok := p.token
		p.next()
		p.expectSemi()
		return &semantics.BranchStmt{Tok: tok}
	}
	if p.token.ID == token.Ident {
		return p.parseAssignOrCallStmt()
	}
	tok := p.token
	p.next()
	p.error(tok, "got '%s', expected statement", tok.ID)
	return &semantics.BadStmt{From: tok, To: tok}
}

func (p *parser) parseBlockStmt() *semantics.BlockStmt {
	block := &semantics.BlockStmt{}
	block.Lbrace = p.token
	p.expect(token.Lbrace)
	for p.token.ID != token.Rbrace && p.token.ID != token.EOF {
		block.Stmts = append(block.Stmts, p.parseStmt())
	}
	block.Rbrace = p.token
	p.expect(token.Rbrace)
	return block
}

func (p *parser) parseDeclStmt() *semantics.DeclStmt {
	d := p.parseVarDecl()
	return &semantics.DeclStmt{D: d}
}

func (p *parser) parsePrintStmt() *semantics.PrintStmt {
	s := &semantics.PrintStmt{}
	s.Print = p.token
	p.expect(token.Print)
	s.X = p.parseExpr()
	p.expectSemi()
	return s
}

func (p *parser) parseIfStmt() *semantics.IfStmt {
	s := &semantics.IfStmt{}
	s.If = p.token
	p.next()
	s.Cond = p.parseExpr()
	s.Body = p.parseBlockStmt()
	if p.token.ID == token.Elif {
		s.Else = p.parseIfStmt()
	} else if p.token.ID == token.Else {
		p.next() // We might wanna save this token...
		s.Else = p.parseBlockStmt()
	}
	return s
}

func (p *parser) parseWhileStmt() *semantics.WhileStmt {
	s := &semantics.WhileStmt{}
	p.inLoop = true
	s.While = p.token
	p.next()
	s.Cond = p.parseExpr()
	s.Body = p.parseBlockStmt()
	p.inLoop = false
	return s
}

func (p *parser) parseReturnStmt() *semantics.ReturnStmt {
	s := &semantics.ReturnStmt{}
	s.Return = p.token
	p.next()
	if p.token.ID != token.Semicolon {
		s.X = p.parseExpr()
	}
	p.expect(token.Semicolon)
	return s
}

func (p *parser) parseAssignOrCallStmt() semantics.Stmt {
	id := p.parseIdent()
	if p.token.IsAssignOperator() {
		return p.parseAssignStmt(id)
	} else if p.token.ID == token.Lparen {
		return p.parseCallStmt(id)
	}
	tok := p.token
	p.next()
	p.error(tok, "got %s, expected assign or call statement", tok.ID)
	return &semantics.BadStmt{From: id.Name, To: tok}
}

func (p *parser) parseAssignStmt(id *semantics.Ident) *semantics.AssignStmt {
	assign := p.token
	p.next()
	rhs := p.parseExpr()
	p.expectSemi()
	return &semantics.AssignStmt{Name: id, Assign: assign, Right: rhs}
}

func (p *parser) parseCallStmt(id *semantics.Ident) *semantics.ExprStmt {
	x := p.parseCallExpr(id)
	p.expect(token.Semicolon)
	return &semantics.ExprStmt{X: x}
}

func (p *parser) parseExpr() semantics.Expr {
	return p.parseLogicalOr()
}

func (p *parser) parseLogicalOr() semantics.Expr {
	expr := p.parseLogicalAnd()
	for p.token.ID == token.Lor {
		op := p.token
		p.next()
		right := p.parseLogicalAnd()
		expr = &semantics.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseLogicalAnd() semantics.Expr {
	expr := p.parseEquality()
	for p.token.ID == token.Land {
		op := p.token
		p.next()
		right := p.parseEquality()
		expr = &semantics.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseEquality() semantics.Expr {
	expr := p.parseComparison()
	for p.token.ID == token.Eq || p.token.ID == token.Neq {
		op := p.token
		p.next()
		right := p.parseComparison()
		expr = &semantics.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseComparison() semantics.Expr {
	expr := p.parseTerm()
	for p.token.ID == token.Gt || p.token.ID == token.GtEq ||
		p.token.ID == token.Lt || p.token.ID == token.LtEq {
		op := p.token
		p.next()
		right := p.parseTerm()
		expr = &semantics.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseTerm() semantics.Expr {
	expr := p.parseFactor()
	for p.token.ID == token.Add || p.token.ID == token.Sub {
		op := p.token
		p.next()
		right := p.parseFactor()
		expr = &semantics.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseFactor() semantics.Expr {
	expr := p.parseUnary()
	for p.token.ID == token.Mul || p.token.ID == token.Div || p.token.ID == token.Mod {
		op := p.token
		p.next()
		right := p.parseUnary()
		expr = &semantics.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseUnary() semantics.Expr {
	if p.token.ID == token.Sub || p.token.ID == token.Lnot {
		op := p.token
		p.next()
		x := p.parseUnary()
		return &semantics.UnaryExpr{Op: op, X: x}
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() semantics.Expr {
	switch p.token.ID {
	case token.LitInteger, token.LitString, token.True, token.False:
		tok := p.token
		p.next()
		return &semantics.Literal{Value: tok}
	case token.Ident:
		ident := p.parseIdent()
		if p.token.ID == token.Lparen {
			return p.parseCallExpr(ident)
		}
		return ident
	case token.Lparen:
		p.next()
		x := p.parseExpr()
		p.expect(token.Rparen)
		return x
	default:
		// TODO: Sync?
		tok := p.token
		p.error(tok, "got '%s', expected expression", tok.ID)
		p.next()
		return &semantics.BadExpr{From: tok, To: tok}
	}
}

func (p *parser) parseIdent() *semantics.Ident {
	tok := p.token
	p.expect(token.Ident)
	return &semantics.Ident{Name: tok}
}

func (p *parser) parseCallExpr(id *semantics.Ident) *semantics.CallExpr {
	lparen := p.token
	p.expect(token.Lparen)
	var args []semantics.Expr
	if p.token.ID != token.Rparen {
		args = append(args, p.parseExpr())
		for p.token.ID != token.EOF && p.token.ID != token.Rparen {
			p.expect(token.Comma)
			args = append(args, p.parseExpr())
		}
	}
	rparen := p.token
	p.expect(token.Rparen)
	return &semantics.CallExpr{Name: id, Lparen: lparen, Args: args, Rparen: rparen}
}
