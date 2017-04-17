package parser

import (
	"fmt"
	"io/ioutil"

	"github.com/jhnl/interpreter/ast"
	"github.com/jhnl/interpreter/scanner"
	"github.com/jhnl/interpreter/token"
)

func ParseFile(filename string) (*ast.Module, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return Parse(buf)
}

func Parse(src []byte) (*ast.Module, error) {
	var p parser
	p.init(src)
	p.next()

	mod := p.parseModule()

	if len(p.errors) > 0 {
		return nil, p.errors
	}

	return mod, nil
}

type parser struct {
	trace   bool
	scanner scanner.Scanner
	scope   *ast.Scope
	token   token.Token

	errors scanner.ErrorList
}

func (p *parser) init(src []byte) {
	p.trace = false
	p.scanner.Init(src)
}

func (p *parser) openScope() {
	p.scope = ast.NewScope(p.scope)
}

func (p *parser) closeScope() {
	p.scope = p.scope.Outer
}

func (p *parser) declare(decl *ast.DeclStmt) {
	sym := ast.NewSymbol(ast.VarSymbol, decl)
	if existing := p.scope.Insert(sym); existing != nil {
		msg := fmt.Sprintf("redeclaration of '%s', previously declared at %s", decl.Name.Name.Literal, existing.Pos())
		p.error(decl.Name.Name, msg)
	}
}

func (p *parser) resolve(name token.Token) {
	if existing := p.scope.Lookup(name.Literal); existing == nil {
		msg := fmt.Sprintf("'%s' undefined", name.Literal)
		p.error(name, msg)
	}
}

func (p *parser) next() {
	if p.trace && p.token.IsValid() {
		fmt.Println(p.token)
	}
	var errMsg *string
	p.token, errMsg = p.scanner.Scan()
	if errMsg != nil {
		p.error(p.token, *errMsg)
	}
}

func (p *parser) error(tok token.Token, msg string) {
	// For now, don't add the error if it's on the same line as the last error.
	if n := len(p.errors); n > 0 && p.errors[n-1].Tok.Line == tok.Line {
		return
	}
	p.errors.Add(tok, msg)
}

func (p *parser) sync() {
loop:
	for {
		switch p.token.ID {
		case token.SEMICOLON: // TODO: Remove this?
			break loop
		case token.VAR, token.PRINT, token.IF, token.BREAK, token.CONTINUE:
			return
		case token.EOF:
			return
		}
		p.next()
	}
	p.next()
}

func (p *parser) match(id token.TokenID) bool {
	if p.token.ID != id {
		return false
	}
	p.next()
	return true
}

func (p *parser) expect(id token.TokenID) bool {
	if !p.match(id) {
		p.error(p.token, fmt.Sprintf("got '%s', expected '%s'", p.token.ID, id))
		p.next()
		return false
	}
	return true
}

func (p *parser) expectSemi() {
	if !p.expect(token.SEMICOLON) {
		p.sync()
	}
}

func (p *parser) parseModule() *ast.Module {
	mod := &ast.Module{}
	if p.token.ID == token.MODULE {
		mod.Mod = p.token
		p.expect(token.MODULE)
		mod.Name = p.parseIdent()
		p.expectSemi()
	}
	p.openScope()
	for p.token.ID != token.EOF {
		mod.Stmts = append(mod.Stmts, p.parseStmt())
	}
	mod.Scope = p.scope
	p.closeScope()
	return mod
}

func (p *parser) parseStmt() ast.Stmt {
	if p.token.ID == token.LBRACE {
		return p.parseBlockStmt()
	}
	if p.token.ID == token.VAR {
		return p.parseDeclStmt()
	}
	if p.token.ID == token.PRINT {
		return p.parsePrintStmt()
	}
	if p.token.ID == token.IF {
		return p.parseIfStmt()
	}
	if p.token.ID == token.WHILE {
		return p.parseWhileStmt()
	}
	if p.token.ID == token.BREAK || p.token.ID == token.CONTINUE {
		tok := p.token
		p.next()
		p.expectSemi()
		return &ast.BranchStmt{Tok: tok}
	}
	return p.parseExprStmt()
}

func (p *parser) parseBlockStmt() *ast.BlockStmt {
	p.openScope()
	block := &ast.BlockStmt{}
	block.Lbrace = p.token
	p.expect(token.LBRACE)
	for p.token.ID != token.RBRACE && p.token.ID != token.EOF {
		block.Stmts = append(block.Stmts, p.parseStmt())
	}
	block.Rbrace = p.token
	p.expect(token.RBRACE)
	block.Scope = p.scope
	p.closeScope()
	return block
}

func (p *parser) parseDeclStmt() *ast.DeclStmt {
	decl := &ast.DeclStmt{}
	decl.Decl = p.token
	p.next()
	decl.Name = p.parseIdent()
	p.expect(token.ASSIGN)
	decl.X = p.parseExpr()
	p.expectSemi()
	p.declare(decl)
	return decl
}

func (p *parser) parsePrintStmt() *ast.PrintStmt {
	s := &ast.PrintStmt{}
	s.Print = p.token
	p.expect(token.PRINT)
	s.X = p.parseExpr()
	p.expectSemi()
	return s
}

func (p *parser) parseIfStmt() *ast.IfStmt {
	s := &ast.IfStmt{}
	s.If = p.token
	p.next()
	s.Cond = p.parseExpr()
	s.Body = p.parseBlockStmt()
	if p.token.ID == token.ELIF {
		s.Else = p.parseIfStmt()
	} else if p.token.ID == token.ELSE {
		p.next() // We might wanna save this token...
		s.Else = p.parseBlockStmt()
	}

	return s
}

func (p *parser) parseWhileStmt() *ast.WhileStmt {
	s := &ast.WhileStmt{}
	s.While = p.token
	p.next()
	s.Cond = p.parseExpr()
	s.Body = p.parseBlockStmt()
	return s
}

func (p *parser) parseExprStmt() ast.Stmt {
	x := p.parseExpr()
	if p.token.ID == token.ASSIGN || p.token.ID == token.ADD_ASSIGN || p.token.ID == token.SUB_ASSIGN ||
		p.token.ID == token.MUL_ASSIGN || p.token.ID == token.DIV_ASSIGN || p.token.ID == token.MOD_ASSIGN {
		assign := p.token
		p.next()
		rhs := p.parseExpr()
		p.expectSemi()
		return &ast.AssignStmt{Left: x, Assign: assign, Right: rhs}
	}
	p.expectSemi()
	return &ast.ExprStmt{X: x}
}

func (p *parser) parseExpr() ast.Expr {
	return p.parseLogicalOr()
}

func (p *parser) parseLogicalOr() ast.Expr {
	expr := p.parseLogicalAnd()
	for p.token.ID == token.LOR {
		op := p.token
		p.next()
		right := p.parseLogicalAnd()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseLogicalAnd() ast.Expr {
	expr := p.parseEquality()
	for p.token.ID == token.LAND {
		op := p.token
		p.next()
		right := p.parseEquality()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseEquality() ast.Expr {
	expr := p.parseComparison()
	for p.token.ID == token.EQ || p.token.ID == token.NEQ {
		op := p.token
		p.next()
		right := p.parseComparison()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseComparison() ast.Expr {
	expr := p.parseTerm()
	for p.token.ID == token.GT || p.token.ID == token.GTEQ ||
		p.token.ID == token.LT || p.token.ID == token.LTEQ {
		op := p.token
		p.next()
		right := p.parseTerm()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseTerm() ast.Expr {
	expr := p.parseFactor()
	for p.token.ID == token.ADD || p.token.ID == token.SUB {
		op := p.token
		p.next()
		right := p.parseFactor()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseFactor() ast.Expr {
	expr := p.parseUnary()
	for p.token.ID == token.MUL || p.token.ID == token.DIV || p.token.ID == token.MOD {
		op := p.token
		p.next()
		right := p.parseUnary()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseUnary() ast.Expr {
	if p.token.ID == token.SUB || p.token.ID == token.LNOT {
		op := p.token
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Op: op, X: x}
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() ast.Expr {
	switch p.token.ID {
	case token.INT, token.STRING, token.TRUE, token.FALSE:
		tok := p.token
		p.next()
		return &ast.Literal{Value: tok}
	case token.IDENT:
		ident := p.parseIdent()
		p.resolve(ident.Name) // TODO: cleanup?
		return ident
	case token.LPAREN:
		p.next()
		x := p.parseExpr()
		p.expect(token.RPAREN)
		return x
	default:
		// TODO: Sync?
		tok := p.token
		p.error(tok, fmt.Sprintf("got '%s', expected expression", tok.ID))
		p.next()
		return &ast.BadExpr{From: tok, To: tok}
	}
}

func (p *parser) parseIdent() *ast.Ident {
	tok := p.token
	p.expect(token.IDENT)
	return &ast.Ident{Name: tok}
}
