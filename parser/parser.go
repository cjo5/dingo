package parser

import (
	"fmt"
	"io/ioutil"

	"github.com/jhnl/interpreter/report"
	"github.com/jhnl/interpreter/sem"
	"github.com/jhnl/interpreter/token"
)

func ParseFile(filename string) (*sem.Module, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return parse(buf, filename)
}

func Parse(src []byte) (*sem.Module, error) {
	return parse(src, "")
}

func parse(src []byte, filename string) (*sem.Module, error) {
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
	errors  report.ErrorList
	trace   bool

	token      token.Token
	scope      *sem.Scope
	inLoop     bool
	inFunction bool
}

func (p *parser) init(src []byte, filename string) {
	p.trace = false
	p.scanner.init(src, filename, &p.errors)
}

func (p *parser) openScope() {
	p.scope = sem.NewScope(p.scope)
}

func (p *parser) closeScope() {
	p.scope = p.scope.Outer
}

func (p *parser) declare(id sem.SymbolID, name token.Token, decl sem.Node) {
	sym := sem.NewSymbol(id, name, decl)
	if existing := p.scope.Insert(sym); existing != nil {
		msg := fmt.Sprintf("redeclaration of '%s', previously declared at %s", name.Literal, existing.Pos())
		p.error(name, msg)
	}
}

func (p *parser) resolve(name token.Token) {
	if existing, _ := p.scope.Lookup(name.Literal); existing == nil {
		p.error(name, "'%s' undefined", name.Literal)
	}
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

func (p *parser) parseModule() *sem.Module {
	mod := &sem.Module{}
	if p.token.ID == token.Module {
		mod.Mod = p.token
		p.expect(token.Module)
		mod.Name = p.parseIdent()
		p.expectSemi()
	}
	p.openScope()
	for p.token.ID != token.EOF {
		mod.Decls = append(mod.Decls, p.parseDecl())
	}
	mod.Scope = p.scope
	p.closeScope()
	return mod
}

func (p *parser) parseDecl() sem.Decl {
	if p.token.ID == token.Var {
		return p.parseVarDecl()
	} else if p.token.ID == token.Func {
		return p.parseFuncDecl()
	}
	tok := p.token
	p.next()
	p.error(tok, "got '%s', expected declaration", tok.ID)
	return &sem.BadDecl{From: tok, To: tok}
}

func (p *parser) parseVarDecl() *sem.VarDecl {
	decl := &sem.VarDecl{}
	decl.Decl = p.token
	p.next()
	decl.Name = p.parseIdent()
	p.expect(token.Assign)
	decl.X = p.parseExpr()
	p.expect(token.Semicolon)
	p.declare(sem.VarSymbol, decl.Name.Name, decl)
	return decl
}

func (p *parser) parseFuncDecl() *sem.FuncDecl {
	decl := &sem.FuncDecl{}
	decl.Decl = p.token
	p.next()
	decl.Name = p.parseIdent()
	p.declare(sem.FuncSymbol, decl.Name.Name, decl)
	p.openScope()

	p.expect(token.Lparen)
	if p.token.ID == token.Ident {
		field := p.parseIdent()
		p.declare(sem.VarSymbol, field.Name, field)
		decl.Fields = append(decl.Fields, field)
		for p.token.ID != token.EOF && p.token.ID != token.Rparen {
			p.expect(token.Comma)
			field = p.parseIdent()
			p.declare(sem.VarSymbol, field.Name, field)
			decl.Fields = append(decl.Fields, field)
		}
	}
	p.expect(token.Rparen)

	p.inFunction = true
	decl.Body = p.parseBlockStmt(false)
	p.inFunction = false
	decl.Scope = p.scope
	p.closeScope()

	// Ensure there is atlesem 1 return statement and that every return has an expression

	lit0 := token.Synthetic(token.Int, "0")
	endsWithReturn := false
	for i, stmt := range decl.Body.Stmts {
		if t, ok := stmt.(*sem.ReturnStmt); ok {
			if t.X == nil {
				t.X = &sem.Literal{Value: lit0}
			}
			if (i + 1) == len(decl.Body.Stmts) {
				endsWithReturn = true
			}
		}
	}

	if !endsWithReturn {
		tok := token.Synthetic(token.Return, "return")
		returnStmt := &sem.ReturnStmt{Return: tok, X: &sem.Literal{Value: lit0}}
		decl.Body.Stmts = append(decl.Body.Stmts, returnStmt)
	}

	return decl
}

func (p *parser) parseStmt() sem.Stmt {
	if p.token.ID == token.Lbrace {
		return p.parseBlockStmt(true)
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
		return &sem.BranchStmt{Tok: tok}
	}
	if p.token.ID == token.Ident {
		return p.parseAssignOrCallStmt()
	}
	tok := p.token
	p.next()
	p.error(tok, "got '%s', expected statement", tok.ID)
	return &sem.BadStmt{From: tok, To: tok}
}

func (p *parser) parseBlockStmt(newScope bool) *sem.BlockStmt {
	if newScope {
		p.openScope()
	}
	block := &sem.BlockStmt{}
	block.Lbrace = p.token
	p.expect(token.Lbrace)
	for p.token.ID != token.Rbrace && p.token.ID != token.EOF {
		block.Stmts = append(block.Stmts, p.parseStmt())
	}
	block.Rbrace = p.token
	p.expect(token.Rbrace)
	block.Scope = p.scope
	if newScope {
		p.closeScope()
	}
	return block
}

func (p *parser) parseDeclStmt() *sem.DeclStmt {
	d := p.parseVarDecl()
	return &sem.DeclStmt{D: d}
}

func (p *parser) parsePrintStmt() *sem.PrintStmt {
	s := &sem.PrintStmt{}
	s.Print = p.token
	p.expect(token.Print)
	s.X = p.parseExpr()
	p.expectSemi()
	return s
}

func (p *parser) parseIfStmt() *sem.IfStmt {
	s := &sem.IfStmt{}
	s.If = p.token
	p.next()
	s.Cond = p.parseExpr()
	s.Body = p.parseBlockStmt(true)
	if p.token.ID == token.Elif {
		s.Else = p.parseIfStmt()
	} else if p.token.ID == token.Else {
		p.next() // We might wanna save this token...
		s.Else = p.parseBlockStmt(true)
	}

	return s
}

func (p *parser) parseWhileStmt() *sem.WhileStmt {
	s := &sem.WhileStmt{}
	p.inLoop = true
	s.While = p.token
	p.next()
	s.Cond = p.parseExpr()
	s.Body = p.parseBlockStmt(true)
	p.inLoop = false
	return s
}

func (p *parser) parseReturnStmt() *sem.ReturnStmt {
	if !p.inFunction {
		p.error(p.token, "%s can only be used in a function", p.token.ID)
	}
	s := &sem.ReturnStmt{}
	s.Return = p.token
	p.next()
	if p.token.ID != token.Semicolon {
		s.X = p.parseExpr()
	}
	p.expect(token.Semicolon)
	return s
}

func (p *parser) parseAssignOrCallStmt() sem.Stmt {
	id := p.parseIdent()
	if p.token.IsAssignOperator() {
		return p.parseAssignStmt(id)
	} else if p.token.ID == token.Lparen {
		return p.parseCallStmt(id)
	}
	tok := p.token
	p.next()
	p.error(tok, "got %s, expected assign or call statement", tok.ID)
	return &sem.BadStmt{From: id.Name, To: tok}
}

func (p *parser) parseAssignStmt(id *sem.Ident) *sem.AssignStmt {
	assign := p.token
	p.next()
	rhs := p.parseExpr()
	p.expectSemi()
	return &sem.AssignStmt{Name: id, Assign: assign, Right: rhs}
}

func (p *parser) parseCallStmt(id *sem.Ident) *sem.ExprStmt {
	x := p.parseCallExpr(id)
	p.expect(token.Semicolon)
	return &sem.ExprStmt{X: x}
}

func (p *parser) parseExpr() sem.Expr {
	return p.parseLogicalOr()
}

func (p *parser) parseLogicalOr() sem.Expr {
	expr := p.parseLogicalAnd()
	for p.token.ID == token.Lor {
		op := p.token
		p.next()
		right := p.parseLogicalAnd()
		expr = &sem.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseLogicalAnd() sem.Expr {
	expr := p.parseEquality()
	for p.token.ID == token.Land {
		op := p.token
		p.next()
		right := p.parseEquality()
		expr = &sem.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseEquality() sem.Expr {
	expr := p.parseComparison()
	for p.token.ID == token.Eq || p.token.ID == token.Neq {
		op := p.token
		p.next()
		right := p.parseComparison()
		expr = &sem.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseComparison() sem.Expr {
	expr := p.parseTerm()
	for p.token.ID == token.Gt || p.token.ID == token.GtEq ||
		p.token.ID == token.Lt || p.token.ID == token.LtEq {
		op := p.token
		p.next()
		right := p.parseTerm()
		expr = &sem.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseTerm() sem.Expr {
	expr := p.parseFactor()
	for p.token.ID == token.Add || p.token.ID == token.Sub {
		op := p.token
		p.next()
		right := p.parseFactor()
		expr = &sem.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseFactor() sem.Expr {
	expr := p.parseUnary()
	for p.token.ID == token.Mul || p.token.ID == token.Div || p.token.ID == token.Mod {
		op := p.token
		p.next()
		right := p.parseUnary()
		expr = &sem.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseUnary() sem.Expr {
	if p.token.ID == token.Sub || p.token.ID == token.Lnot {
		op := p.token
		p.next()
		x := p.parseUnary()
		return &sem.UnaryExpr{Op: op, X: x}
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() sem.Expr {
	switch p.token.ID {
	case token.Int, token.String, token.True, token.False:
		tok := p.token
		p.next()
		return &sem.Literal{Value: tok}
	case token.Ident:
		ident := p.parseIdent()
		p.resolve(ident.Name) // TODO: cleanup?
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
		return &sem.BadExpr{From: tok, To: tok}
	}
}

func (p *parser) parseIdent() *sem.Ident {
	tok := p.token
	p.expect(token.Ident)
	return &sem.Ident{Name: tok}
}

func (p *parser) parseCallExpr(id *sem.Ident) *sem.CallExpr {
	sym, _ := p.scope.Lookup(id.Name.Literal)
	if sym == nil {
		p.error(id.Name, "'%s' undefined", id.Name.Literal)
	} else if sym.ID != sem.FuncSymbol {
		p.error(id.Name, "'%s' is not a function", sym.Name.Literal)
	}
	lparen := p.token
	p.expect(token.Lparen)
	var args []sem.Expr
	if p.token.ID != token.Rparen {
		args = append(args, p.parseExpr())
		for p.token.ID != token.EOF && p.token.ID != token.Rparen {
			p.expect(token.Comma)
			args = append(args, p.parseExpr())
		}
	}
	if sym != nil {
		decl, _ := sym.Decl.(*sem.FuncDecl)
		if len(decl.Fields) != len(args) {
			p.error(id.Name, "'%s' takes %d argument(s), but called with %d", sym.Name.Literal, len(decl.Fields), len(args))
		}
	}
	rparen := p.token
	p.expect(token.Rparen)
	return &sem.CallExpr{Name: id, Lparen: lparen, Args: args, Rparen: rparen}
}
