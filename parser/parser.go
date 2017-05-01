package parser

import (
	"fmt"
	"io/ioutil"

	"github.com/jhnl/interpreter/ast"
	"github.com/jhnl/interpreter/report"
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
	scanner Scanner
	scope   *ast.Scope
	token   token.Token

	errors report.ErrorList
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
		case token.Semicolon: // TODO: Remove this?
			break loop
		case token.Var, token.Print, token.If, token.Break, token.Continue:
			return
		case token.Eof:
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

func (p *parser) parseModule() *ast.Module {
	mod := &ast.Module{}
	if p.token.ID == token.Module {
		mod.Mod = p.token
		p.expect(token.Module)
		mod.Name = p.parseIdent()
		p.expectSemi()
	}
	p.openScope()
	for p.token.ID != token.Eof {
		mod.Stmts = append(mod.Stmts, p.parseStmt())
	}
	mod.Scope = p.scope
	p.closeScope()
	return mod
}

func (p *parser) parseStmt() ast.Stmt {
	if p.token.ID == token.Lbrace {
		return p.parseBlockStmt()
	}
	if p.token.ID == token.Var {
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
	if p.token.ID == token.Break || p.token.ID == token.Continue {
		tok := p.token
		p.next()
		p.expectSemi()
		return &ast.BranchStmt{Tok: tok}
	}
	if p.token.ID == token.Ident {
		return p.parseAssignStmt()
	}
	tok := p.token
	p.next()
	p.error(tok, fmt.Sprintf("got '%s', expected statement", tok.ID))
	return &ast.BadStmt{From: tok, To: tok}
}

func (p *parser) parseBlockStmt() *ast.BlockStmt {
	p.openScope()
	block := &ast.BlockStmt{}
	block.Lbrace = p.token
	p.expect(token.Lbrace)
	for p.token.ID != token.Rbrace && p.token.ID != token.Eof {
		block.Stmts = append(block.Stmts, p.parseStmt())
	}
	block.Rbrace = p.token
	p.expect(token.Rbrace)
	block.Scope = p.scope
	p.closeScope()
	return block
}

func (p *parser) parseDeclStmt() *ast.DeclStmt {
	decl := &ast.DeclStmt{}
	decl.Decl = p.token
	p.next()
	decl.Name = p.parseIdent()
	p.expect(token.Assign)
	decl.X = p.parseExpr()

	// TODO: Remove this check when functions have been added
	if p.expectSemi() && p.scope.Outer != nil {
		p.error(decl.Name.Name, "Variables must be declared in global scope")
	} else {
		p.declare(decl)
	}

	return decl
}

func (p *parser) parsePrintStmt() *ast.PrintStmt {
	s := &ast.PrintStmt{}
	s.Print = p.token
	p.expect(token.Print)
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
	if p.token.ID == token.Elif {
		s.Else = p.parseIfStmt()
	} else if p.token.ID == token.Else {
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

func (p *parser) parseAssignStmt() ast.Stmt {
	id := p.parseIdent()
	assign := p.token
	p.next()
	rhs := p.parseExpr()
	p.expectSemi()
	return &ast.AssignStmt{ID: id, Assign: assign, Right: rhs}
}

func (p *parser) parseExpr() ast.Expr {
	return p.parseLogicalOr()
}

func (p *parser) parseLogicalOr() ast.Expr {
	expr := p.parseLogicalAnd()
	for p.token.ID == token.Lor {
		op := p.token
		p.next()
		right := p.parseLogicalAnd()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseLogicalAnd() ast.Expr {
	expr := p.parseEquality()
	for p.token.ID == token.Land {
		op := p.token
		p.next()
		right := p.parseEquality()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseEquality() ast.Expr {
	expr := p.parseComparison()
	for p.token.ID == token.Eq || p.token.ID == token.Neq {
		op := p.token
		p.next()
		right := p.parseComparison()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseComparison() ast.Expr {
	expr := p.parseTerm()
	for p.token.ID == token.Gt || p.token.ID == token.GtEq ||
		p.token.ID == token.Lt || p.token.ID == token.LtEq {
		op := p.token
		p.next()
		right := p.parseTerm()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseTerm() ast.Expr {
	expr := p.parseFactor()
	for p.token.ID == token.Add || p.token.ID == token.Sub {
		op := p.token
		p.next()
		right := p.parseFactor()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseFactor() ast.Expr {
	expr := p.parseUnary()
	for p.token.ID == token.Mul || p.token.ID == token.Div || p.token.ID == token.Mod {
		op := p.token
		p.next()
		right := p.parseUnary()
		expr = &ast.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseUnary() ast.Expr {
	if p.token.ID == token.Sub || p.token.ID == token.Lnot {
		op := p.token
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Op: op, X: x}
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() ast.Expr {
	switch p.token.ID {
	case token.Int, token.String, token.True, token.False:
		tok := p.token
		p.next()
		return &ast.Literal{Value: tok}
	case token.Ident:
		ident := p.parseIdent()
		p.resolve(ident.Name) // TODO: cleanup?
		return ident
	case token.Lparen:
		p.next()
		x := p.parseExpr()
		p.expect(token.Rparen)
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
	p.expect(token.Ident)
	return &ast.Ident{Name: tok}
}
