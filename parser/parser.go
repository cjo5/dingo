package parser

import (
	"fmt"
	"io/ioutil"

	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/semantics"
	"github.com/jhnl/interpreter/token"
)

func ParseFile(filepath string) (*semantics.FileDecls, error) {
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	return parse(buf, filepath)
}

func Parse(src []byte) (*semantics.FileDecls, error) {
	return parse(src, "")
}

func parse(src []byte, filepath string) (*semantics.FileDecls, error) {
	var p parser
	p.init(src, filepath)
	p.next()

	file := p.parseFileDecls()

	if p.errors.Count() > 0 {
		return file, p.errors
	}

	return file, nil
}

type parser struct {
	lexer  lexer
	errors *common.ErrorList
	trace  bool

	token  token.Token
	inLoop bool
}

func (p *parser) init(src []byte, filename string) {
	p.trace = false
	p.errors = &common.ErrorList{}
	p.lexer.init(src, filename, p.errors)
}

func (p *parser) next() {
	if p.trace && p.token.IsValid() {
		fmt.Println(p.token)
	}

	for {
		p.token = p.lexer.lex()
		if p.token.ID != token.Comment && p.token.ID != token.MultiComment {
			break
		}
	}
}

func (p *parser) error(tok token.Token, format string, args ...interface{}) {
	p.errors.Add(p.lexer.filename, tok.Pos, format, args...)
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

func (p *parser) parseFileDecls() *semantics.FileDecls {
	file := &semantics.FileDecls{Info: &semantics.FileInfo{}}
	if p.token.Is(token.Module) {
		file.Info.Decl = p.token
		p.expect(token.Module)
	} else {
		file.Info.Decl = token.Synthetic(token.Module, token.Module.String())
	}
	for p.token.Is(token.Import) {
		imp := p.parseImport()
		if imp != nil {
			file.Info.Imports = append(file.Info.Imports, imp)
		}
	}
	for !p.token.Is(token.EOF) {
		file.Decls = append(file.Decls, p.parseTopLevelDecl())
	}
	return file
}

func (p *parser) parseImport() *semantics.Import {
	tok := p.token
	p.next()
	path := p.token
	if !p.expect(token.String) {
		return nil
	}
	return &semantics.Import{Import: tok, Literal: path}
}

func (p *parser) parseTopLevelDecl() semantics.Decl {
	visibility := token.Synthetic(token.Internal, token.Internal.String())
	if p.token.OneOf(token.External, token.Internal, token.Restricted) {
		visibility = p.token
		p.next()
	}
	return p.parseDecl(visibility)
}

func (p *parser) parseDecl(visibility token.Token) semantics.Decl {
	if p.token.ID == token.Var || p.token.ID == token.Val {
		decl := p.parseVarDecl(visibility)
		return decl
	} else if p.token.ID == token.Func {
		decl := p.parseFuncDecl(visibility)
		return decl
	} else if p.token.ID == token.Struct {
		return p.parseStructDecl()
	}
	tok := p.token
	p.next()
	p.error(tok, "got '%s', expected declaration", tok.ID)
	return &semantics.BadDecl{From: tok, To: tok}
}

func (p *parser) parseVarDecl(visibility token.Token) *semantics.VarDecl {
	decl := &semantics.VarDecl{}
	decl.Visibility = visibility
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

func (p *parser) parseFuncDecl(visibility token.Token) *semantics.FuncDecl {
	decl := &semantics.FuncDecl{}
	decl.Visibility = visibility
	decl.Decl = p.token
	p.next()
	decl.Name = p.parseIdent()

	decl.Lparen = p.token
	p.expect(token.Lparen)
	if p.token.ID != token.Rparen {
		field := p.parseField()
		decl.Params = append(decl.Params, field)
		for p.token.ID != token.EOF && p.token.ID != token.Rparen {
			p.expect(token.Comma)
			field = p.parseField()
			decl.Params = append(decl.Params, field)
		}
	}
	decl.Rparen = p.token
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

func (p *parser) parseStructDecl() *semantics.StructDecl {
	decl := &semantics.StructDecl{}
	decl.Decl = p.token
	p.next()
	decl.Name = p.parseIdent()

	decl.Lbrace = p.token
	p.expect(token.Lbrace)

	if !p.token.Is(token.Rbrace) {
		decl.Fields = append(decl.Fields, p.parseStructField())
		for !p.token.OneOf(token.EOF, token.Rbrace) {
			p.expect(token.Comma)
			decl.Fields = append(decl.Fields, p.parseStructField())
		}
	}

	decl.Rbrace = p.token
	p.expect(token.Rbrace)

	return decl
}

func (p *parser) parseStructField() *semantics.StructField {
	field := &semantics.StructField{}
	if p.token.OneOf(token.Static, token.Val, token.Var) {
		field.Qualifier = p.token
	} else {
		field.Qualifier = token.Synthetic(token.Var, token.Var.String())
	}
	field.Name = p.parseIdent()
	p.expect(token.Colon)
	field.Type = p.parseExpr()
	return field
}

func (p *parser) parseStmt() semantics.Stmt {
	if p.token.ID == token.Lbrace {
		return p.parseBlockStmt()
	}
	if p.token.ID == token.Var || p.token.ID == token.Val {
		return p.parseVarDeclStmt()
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

func (p *parser) parseVarDeclStmt() *semantics.DeclStmt {
	visibility := token.Synthetic(token.Invalid, token.Invalid.String())
	d := p.parseVarDecl(visibility)
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
	x := p.parseFuncCall(id)
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
	case token.Integer, token.Float, token.String, token.True, token.False:
		tok := p.token
		p.next()
		return &semantics.Literal{Value: tok}
	case token.Ident:
		ident := p.parseIdent()
		if p.token.Is(token.Lparen) {
			return p.parseFuncCall(ident)
		} else if p.token.Is(token.Dot) {
			return p.parseDotExpr(ident)
		} else if p.token.Is(token.Lbrace) {
			return p.parseStructLiteral(ident)
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

func (p *parser) parseFuncCall(id *semantics.Ident) semantics.Expr {
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
	if !p.expect(token.Rparen) {
		return &semantics.BadExpr{From: lparen, To: rparen}
	}
	return &semantics.FuncCall{Name: id, Lparen: lparen, Args: args, Rparen: rparen}
}

func (p *parser) parseDotExpr(id *semantics.Ident) semantics.Expr {
	dot := p.token
	p.expect(token.Dot)
	id2 := p.parseIdent()
	expr := semantics.Expr(id2)
	if p.token.Is(token.Lparen) {
		expr = p.parseFuncCall(id2)
	}
	return &semantics.DotExpr{Name: id, Dot: dot, X: expr}
}

func (p *parser) parseStructLiteral(id *semantics.Ident) semantics.Expr {
	lbrace := p.token
	p.expect(token.Lbrace)
	var inits []*semantics.KeyValue
	if p.token.Is(token.Ident) {
		inits = append(inits, p.parseKeyValue())
		for p.token.ID != token.EOF && p.token.ID != token.Rbrace {
			p.expect(token.Comma)
			inits = append(inits, p.parseKeyValue())
		}
	}
	rbrace := p.token
	p.expect(token.Rbrace)
	return &semantics.StructLiteral{Name: id, Lbrace: lbrace, Initializers: inits, Rbrace: rbrace}
}

func (p *parser) parseKeyValue() *semantics.KeyValue {
	key := p.parseIdent()
	equal := p.token
	p.expect(token.Assign)
	value := p.parseExpr()
	return &semantics.KeyValue{Key: key, Equal: equal, Value: value}
}
