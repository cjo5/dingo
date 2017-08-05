package parser

import (
	"fmt"
	"io/ioutil"

	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/semantics"
	"github.com/jhnl/interpreter/token"
)

func ParseFile(filepath string) (*semantics.File, []semantics.TopDecl, error) {
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, nil, err
	}
	return parse(buf, filepath)
}

func Parse(src []byte) (*semantics.File, []semantics.TopDecl, error) {
	return parse(src, "")
}

func parse(src []byte, filepath string) (*semantics.File, []semantics.TopDecl, error) {
	var p parser
	p.init(src, filepath)
	p.next()

	file, decls := p.parseFile()

	if p.errors.Count() > 0 {
		return file, decls, p.errors
	}

	return file, decls, nil
}

type parseError struct {
	tok token.Token
}

type parser struct {
	lexer  lexer
	errors *common.ErrorList
	trace  bool

	token       token.Token
	prev        token.Token
	inLoop      bool
	inCondition bool
}

func (p *parser) init(src []byte, filename string) {
	p.trace = false
	p.errors = &common.ErrorList{}
	p.lexer.init(src, filename, p.errors)
}

func (p *parser) next() {
	if p.trace {
		fmt.Println(p.token)
	}

	for {
		p.prev = p.token
		p.token = p.lexer.lex()
		if p.token.ID != token.Comment && p.token.ID != token.MultiComment {
			break
		}
	}
}

func (p *parser) error(tok token.Token, format string, args ...interface{}) {
	p.errors.Add(p.lexer.filename, tok.Pos, format, args...)
}

func (p *parser) syncDecl() {
	for {
		switch p.token.ID {
		case token.Public, token.Internal, token.Private: // Visibility modifiers
			return
		case token.Func, token.Struct, token.Var, token.Val: // Decls
			return
		case token.EOF:
			return
		}
		p.next()
	}
}

func (p *parser) syncStmt() {
	for {
		switch p.token.ID {
		case token.Semicolon:
			p.next()
			return
		case token.Rbrace:
			return
		case token.Var, token.Val:
			return
		case token.Print, token.Return, token.If, token.While, token.Break, token.Continue:
			return
		case token.EOF:
			return
		}
		p.next()
	}
}

func (p *parser) match(id token.ID) bool {
	if !p.token.Is(id) {
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
	if !p.expect(token.Semicolon) {
		return false
	}
	for p.token.Is(token.Semicolon) {
		p.next()
	}
	return true
}

func (p *parser) consume(id token.ID) bool {
	if !p.expect(id) {
		panic(parseError{p.token})
	}
	return true
}

func (p *parser) consumeSemi() bool {
	if !p.expectSemi() {
		panic(parseError{p.token})
	}
	return true
}

// Fix support for fatal parse errors.
// A fatal parse error will cause parser to stop and discard the remaining tokens.
//
// Fatal errors:
// - Invalid module declaration
// - Non-declaration outside function body
//
// Improve parsing of function signature and struct fields. Try to synchronize locally instead of panicking.
//
func (p *parser) parseFile() (*semantics.File, []semantics.TopDecl) {
	file := &semantics.File{Ctx: &semantics.FileContext{Path: p.lexer.filename}}
	if p.token.Is(token.Module) {
		file.Ctx.Decl = p.token
		p.next()
		p.expectSemi()
	} else {
		file.Ctx.Decl = token.Synthetic(token.Module, token.Module.String())
	}
	for p.token.Is(token.Import) {
		imp := p.parseImport()
		if imp != nil {
			file.Imports = append(file.Imports, imp)
		}
	}
	var decls []semantics.TopDecl
	for !p.token.Is(token.EOF) {
		decl := p.parseTopDecl()
		if decl != nil {
			decl.SetContext(file.Ctx)
			decls = append(decls, decl)
		}
	}
	return file, decls
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

func (p *parser) parseTopDecl() (decl semantics.TopDecl) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(parseError); ok {
				p.syncDecl()
				decl = nil
			} else {
				panic(r)
			}
		}
	}()

	visibility := token.Synthetic(token.Internal, token.Internal.String())
	if p.token.OneOf(token.Public, token.Internal, token.Private) {
		visibility = p.token
		p.next()
	}

	if p.token.ID == token.Var || p.token.ID == token.Val {
		decl = p.parseValTopDecl(visibility)
	} else if p.token.ID == token.Func {
		decl = p.parseFuncDecl(visibility)
	} else if p.token.ID == token.Struct {
		decl = p.parseStructDecl(visibility)
	} else {
		p.error(p.token, "non-declaration outside function body")
		p.syncDecl()
	}

	return decl
}

func (p *parser) parseValTopDecl(visibility token.Token) *semantics.ValTopDecl {
	decl := &semantics.ValTopDecl{}
	decl.Visibility = visibility
	decl.ValDeclSpec = p.parseValDeclSpec()
	p.consumeSemi()
	return decl
}

func (p *parser) parseValDecl() *semantics.ValDecl {
	decl := &semantics.ValDecl{}
	decl.ValDeclSpec = p.parseValDeclSpec()
	return decl
}

func (p *parser) parseValDeclSpec() semantics.ValDeclSpec {
	decl := semantics.ValDeclSpec{}

	decl.Decl = p.token
	p.next()

	decl.Name = p.token
	p.consume(token.Ident)

	if !p.token.Is(token.Assign) {
		decl.Type = p.parseTypeSpec(nil)
	}

	if p.token.Is(token.Assign) {
		p.next()
		decl.Initializer = p.parseExpr()
	}

	return decl
}

func (p *parser) parseField(flags int, defaultDecl token.ID) *semantics.ValDecl {
	decl := &semantics.ValDecl{}
	decl.Flags = flags

	if p.token.OneOf(token.Val, token.Var) {
		decl.Decl = p.token
		p.next()
	} else {
		decl.Decl = token.Synthetic(defaultDecl, defaultDecl.String())
	}

	decl.Name = p.token
	p.consume(token.Ident)

	decl.Type = p.parseTypeSpec(nil)
	return decl
}

func (p *parser) parseFuncDecl(visibility token.Token) *semantics.FuncDecl {
	decl := &semantics.FuncDecl{}
	decl.Visibility = visibility
	decl.Decl = p.token
	p.next()
	decl.Name = p.token
	p.consume(token.Ident)

	decl.Lparen = p.token
	p.consume(token.Lparen)
	flags := semantics.ValFlagNoInit
	if !p.token.Is(token.Rparen) {
		decl.Params = append(decl.Params, p.parseField(flags, token.Val))
		for !p.token.OneOf(token.EOF, token.Rparen) {
			p.consume(token.Comma)
			decl.Params = append(decl.Params, p.parseField(flags, token.Val))
		}
	}
	decl.Rparen = p.token
	p.consume(token.Rparen)

	if !p.token.Is(token.Lbrace) {
		decl.TReturn = p.parseTypeSpec(nil)
	} else {
		decl.TReturn = &semantics.Ident{Name: token.Synthetic(token.Ident, semantics.TVoid.String())}
	}

	decl.Body = p.parseBlockStmt()
	return decl
}

func (p *parser) parseStructDecl(visibility token.Token) *semantics.StructDecl {
	decl := &semantics.StructDecl{}
	decl.Visibility = visibility
	decl.Decl = p.token
	p.next()
	decl.Name = p.token
	p.consume(token.Ident)

	decl.Lbrace = p.token
	p.consume(token.Lbrace)
	flags := semantics.ValFlagNoInit
	for !p.token.OneOf(token.EOF, token.Rbrace) {
		decl.Fields = append(decl.Fields, p.parseField(flags, token.Var))
		p.consumeSemi()
	}

	decl.Rbrace = p.token
	p.consume(token.Rbrace)

	return decl
}

func (p *parser) parseStmt() (stmt semantics.Stmt) {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(parseError); ok {
				p.syncStmt()
				stmt = &semantics.BadStmt{From: err.tok, To: p.prev}
			} else {
				panic(r)
			}
		}
	}()

	if p.token.ID == token.Lbrace {
		stmt = p.parseBlockStmt()
	} else if p.token.ID == token.Var || p.token.ID == token.Val {
		stmt = p.parseValDeclStmt()
	} else if p.token.ID == token.Print {
		stmt = p.parsePrintStmt()
	} else if p.token.ID == token.If {
		stmt = p.parseIfStmt()
	} else if p.token.ID == token.While {
		stmt = p.parseWhileStmt()
	} else if p.token.ID == token.Return {
		stmt = p.parseReturnStmt()
	} else if p.token.ID == token.Break || p.token.ID == token.Continue {
		if !p.inLoop {
			// TODO: This should be done in type checker
			p.error(p.token, "%s can only be used in a loop", p.token.ID)
		}

		tok := p.token
		p.next()
		p.consumeSemi()
		stmt = &semantics.BranchStmt{Tok: tok}
	} else {
		stmt = p.parseExprOrAssignStmt()
	}
	return stmt
}

func (p *parser) parseBlockStmt() *semantics.BlockStmt {
	block := &semantics.BlockStmt{}
	block.Lbrace = p.token
	p.consume(token.Lbrace)
	for p.token.ID != token.Rbrace && p.token.ID != token.EOF {
		block.Stmts = append(block.Stmts, p.parseStmt())
	}
	block.Rbrace = p.token
	p.consume(token.Rbrace)
	return block
}

func (p *parser) parseValDeclStmt() *semantics.DeclStmt {
	d := p.parseValDecl()
	p.consumeSemi()
	return &semantics.DeclStmt{D: d}
}

func (p *parser) parsePrintStmt() *semantics.PrintStmt {
	s := &semantics.PrintStmt{}
	s.Print = p.token
	p.consume(token.Print)
	s.Xs = append(s.Xs, p.parseExpr())
	for !p.token.OneOf(token.Semicolon, token.Invalid, token.EOF) {
		p.consume(token.Comma)
		if p.token.ID == token.Semicolon {
			break
		}
		s.Xs = append(s.Xs, p.parseExpr())
	}
	p.consumeSemi()
	return s
}

func (p *parser) parseIfStmt() *semantics.IfStmt {
	s := &semantics.IfStmt{}
	s.If = p.token
	p.next()
	p.inCondition = true
	s.Cond = p.parseExpr()
	p.inCondition = false
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
	p.inCondition = true
	s.Cond = p.parseExpr()
	p.inCondition = false
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
	p.consumeSemi()
	return s
}

func (p *parser) parseExprOrAssignStmt() semantics.Stmt {
	var stmt semantics.Stmt
	expr := p.parseExpr()
	if p.token.IsAssignOperator() {
		assign := p.token
		p.next()
		right := p.parseExpr()
		stmt = &semantics.AssignStmt{Left: expr, Assign: assign, Right: right}
	} else {
		stmt = &semantics.ExprStmt{X: expr}
	}
	p.consumeSemi()
	return stmt
}

func (p *parser) parseTypeSpec(name1 *semantics.Ident) semantics.Expr {
	if name1 == nil {
		if p.token.ID == token.Ident {
			name1 = &semantics.Ident{Name: p.token}
			p.next()
		}
	}
	if p.token.ID == token.Dot {
		dot := p.token
		p.next()
		name2Tok := p.token
		p.consume(token.Ident)
		return &semantics.DotIdent{X: name1, Dot: dot, Name: &semantics.Ident{Name: name2Tok}}
	} else if name1 != nil {
		return name1
	}
	tok := p.token
	p.next()
	p.error(tok, "got '%s', expected type specifier", tok.ID)
	panic(parseError{tok})
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
		x := p.parseExpr()
		return &semantics.UnaryExpr{Op: op, X: x}
	}
	return p.parseOperand()
}

func (p *parser) parseOperand() semantics.Expr {
	if p.token.Is(token.Lparen) {
		p.next()
		x := p.parseExpr()
		p.consume(token.Rparen)
		return x
	} else if p.token.Is(token.Ident) {
		ident := p.parseIdent()
		var x semantics.Expr = ident
		if p.token.Is(token.Dot) {
			x = p.parseDotIdent(ident)
		}
		if !p.inCondition && p.token.Is(token.Lbrace) {
			return p.parseStructLit(x)
		}
		return p.parsePrimary(x)
	}
	return p.parseBasicLit()
}

func (p *parser) parsePrimary(expr semantics.Expr) semantics.Expr {
	if p.token.Is(token.Lparen) {
		return p.parsePrimary(p.parseFuncCall(expr))
	} else if p.token.Is(token.Dot) {
		return p.parsePrimary(p.parseDotIdent(expr))
	}
	return expr
}

func (p *parser) parseIdent() *semantics.Ident {
	tok := p.token
	p.consume(token.Ident)
	return &semantics.Ident{Name: tok}
}

func (p *parser) parseDotIdent(expr semantics.Expr) semantics.Expr {
	dot := p.token
	p.consume(token.Dot)
	name := p.parseIdent()
	return &semantics.DotIdent{X: expr, Dot: dot, Name: name}
}

func (p *parser) parseFuncCall(expr semantics.Expr) semantics.Expr {
	lparen := p.token
	p.consume(token.Lparen)
	var args []semantics.Expr
	if p.token.ID != token.Rparen {
		args = append(args, p.parseExpr())
		for p.token.ID != token.EOF && p.token.ID != token.Rparen {
			p.consume(token.Comma)
			args = append(args, p.parseExpr())
		}
	}
	rparen := p.token
	p.consume(token.Rparen)
	return &semantics.FuncCall{X: expr, Lparen: lparen, Args: args, Rparen: rparen}
}

func (p *parser) parseBasicLit() semantics.Expr {
	switch p.token.ID {
	case token.Integer, token.Float, token.String, token.True, token.False:
		tok := p.token
		p.next()
		return &semantics.BasicLit{Value: tok}
	default:
		tok := p.token
		p.error(tok, "got '%s', expected expression", tok.ID)
		p.next()
		panic(parseError{tok})
	}
}

func (p *parser) parseKeyValue() *semantics.KeyValue {
	key := p.token
	p.consume(token.Ident)
	equal := p.token
	p.consume(token.Assign)
	value := p.parseExpr()
	return &semantics.KeyValue{Key: key, Equal: equal, Value: value}
}

func (p *parser) parseStructLit(name semantics.Expr) semantics.Expr {
	lbrace := p.token
	p.consume(token.Lbrace)
	var inits []*semantics.KeyValue
	if p.token.Is(token.Ident) {
		inits = append(inits, p.parseKeyValue())
		for p.token.ID != token.EOF && p.token.ID != token.Rbrace {
			p.consume(token.Comma)
			inits = append(inits, p.parseKeyValue())
		}
	}
	rbrace := p.token
	p.consume(token.Rbrace)
	return &semantics.StructLit{Name: name, Lbrace: lbrace, Initializers: inits, Rbrace: rbrace}
}
