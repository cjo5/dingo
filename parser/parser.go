package parser

import (
	"fmt"
	"io/ioutil"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

func ParseFile(filepath string) (*ir.File, []ir.TopDecl, error) {
	buf, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, nil, err
	}
	return parse(buf, filepath)
}

func Parse(src []byte) (*ir.File, []ir.TopDecl, error) {
	return parse(src, "")
}

func parse(src []byte, filepath string) (*ir.File, []ir.TopDecl, error) {
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
	msg := fmt.Sprintf(format, args...)
	p.errors.Add(p.lexer.filename, tok.Pos, common.SyntaxError, msg)
}

func (p *parser) syncDecl() {
	for {
		switch p.token.ID {
		case token.Public, token.Private: // Visibility modifiers
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
		case token.Var, token.Val:
			return
		case token.Return, token.If, token.While, token.For, token.Break, token.Continue:
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
		p.error(p.token, "got '%s', expected '%s'", p.token.Quote(), id)
		p.next()
		return false
	}
	return true
}

func (p *parser) expectSemi() bool {
	if !p.token.OneOf(token.Rbrace, token.Rbrack) {
		if !p.match(token.Semicolon) {
			p.error(p.token, "got '%s', expected semicolon or newline", p.token.Quote())
			p.next()
		}
		for p.token.Is(token.Semicolon) {
			p.next()
		}
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
func (p *parser) parseFile() (*ir.File, []ir.TopDecl) {
	file := &ir.File{Ctx: &ir.FileContext{Path: p.lexer.filename}}

	if p.token.Is(token.Module) {
		file.Ctx.Decl = p.token
		p.next()
		file.Ctx.Module = p.token
		p.expect(token.Ident)
		p.expectSemi()
	} else {
		file.Ctx.Decl = token.Synthetic(token.Module, token.Module.String())
		file.Ctx.Module = token.Synthetic(token.Ident, "")
	}

	for p.token.Is(token.Include) {
		inc := p.parseInclude()
		if inc != nil {
			file.Includes = append(file.Includes, inc)
		}
	}

	var decls []ir.TopDecl
	for !p.token.Is(token.EOF) {
		decl := p.parseTopDecl()
		if decl != nil {
			decl.SetContext(file.Ctx)
			decls = append(decls, decl)
		}
	}

	return file, decls
}

func (p *parser) parseInclude() *ir.Include {
	tok := p.token
	p.next()
	path := p.token
	if !p.expect(token.String) {
		return nil
	}
	p.expectSemi()
	return &ir.Include{Include: tok, Literal: path}
}

func (p *parser) parseTopDecl() (decl ir.TopDecl) {
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

	visibility := token.Synthetic(token.Private, token.Private.String())
	if p.token.OneOf(token.Public, token.Private) {
		visibility = p.token
		p.next()
	}

	if p.token.ID == token.Var || p.token.ID == token.Val {
		decl = p.parseValTopDecl(visibility)
		p.consumeSemi()
	} else if p.token.ID == token.Func {
		decl = p.parseFuncDecl(visibility)
		p.consumeSemi()
	} else if p.token.ID == token.Struct {
		decl = p.parseStructDecl(visibility)
		p.consumeSemi()
	} else {
		p.error(p.token, "got '%s', expected declaration", p.token.Quote())
		p.syncDecl()
	}

	return decl
}

func (p *parser) parseValTopDecl(visibility token.Token) *ir.ValTopDecl {
	decl := &ir.ValTopDecl{}
	decl.Visibility = visibility
	decl.ValDeclSpec = p.parseValDeclSpec()
	return decl
}

func (p *parser) parseValDecl() *ir.ValDecl {
	decl := &ir.ValDecl{}
	decl.ValDeclSpec = p.parseValDeclSpec()
	return decl
}

func (p *parser) parseValDeclSpec() ir.ValDeclSpec {
	decl := ir.ValDeclSpec{}

	decl.Decl = p.token
	p.next()

	decl.Name = p.token
	p.consume(token.Ident)

	if !p.token.Is(token.Assign) {
		decl.Type = p.parseTypeSpec()
	}

	if p.token.Is(token.Assign) {
		p.next()
		decl.Initializer = p.parseExpr()
	}

	return decl
}

func (p *parser) parseField(flags int, defaultDecl token.ID) *ir.ValDecl {
	decl := &ir.ValDecl{}
	decl.Flags = flags

	if p.token.OneOf(token.Val, token.Var) {
		decl.Decl = p.token
		p.next()
	} else {
		decl.Decl = token.Synthetic(defaultDecl, defaultDecl.String())
	}

	decl.Name = p.token
	p.consume(token.Ident)

	decl.Type = p.parseTypeSpec()
	return decl
}

func (p *parser) parseFuncDecl(visibility token.Token) *ir.FuncDecl {
	decl := &ir.FuncDecl{}
	decl.Visibility = visibility
	decl.Decl = p.token
	p.next()
	decl.Name = p.token
	p.consume(token.Ident)

	decl.Lparen = p.token
	p.consume(token.Lparen)
	flags := ir.AstFlagNoInit
	if !p.token.Is(token.Rparen) {
		decl.Params = append(decl.Params, p.parseField(flags, token.Val))
		for !p.token.OneOf(token.EOF, token.Rparen) {
			p.consume(token.Comma)
			if p.token.Is(token.Rparen) {
				break
			}
			decl.Params = append(decl.Params, p.parseField(flags, token.Val))
		}
	}
	decl.Rparen = p.token
	p.consume(token.Rparen)

	if !p.token.OneOf(token.Lbrace, token.Semicolon) {
		decl.TReturn = p.parseTypeSpec()
	} else {
		decl.TReturn = &ir.Ident{Name: token.Synthetic(token.Ident, ir.TVoid.String())}
	}

	if p.token.Is(token.Semicolon) {
		return decl
	}

	decl.Body = p.parseBlockStmt()
	return decl
}

func (p *parser) parseStructDecl(visibility token.Token) *ir.StructDecl {
	decl := &ir.StructDecl{}
	decl.Visibility = visibility
	decl.Decl = p.token
	p.next()
	decl.Name = p.token
	p.consume(token.Ident)

	decl.Lbrace = p.token
	p.consume(token.Lbrace)
	flags := ir.AstFlagNoInit
	for !p.token.OneOf(token.EOF, token.Rbrace) {
		decl.Fields = append(decl.Fields, p.parseField(flags, token.Var))
		p.consumeSemi()
	}

	decl.Rbrace = p.token
	p.consume(token.Rbrace)

	return decl
}

func (p *parser) parseStmt() (stmt ir.Stmt) {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(parseError); ok {
				p.syncStmt()
				stmt = &ir.BadStmt{From: err.tok, To: p.prev}
			} else {
				panic(r)
			}
		}
	}()

	if p.token.ID == token.Semicolon {
		stmt = nil
	} else if p.token.ID == token.Lbrace {
		stmt = p.parseBlockStmt()
	} else if p.token.ID == token.Var || p.token.ID == token.Val {
		stmt = p.parseValDeclStmt()
	} else if p.token.ID == token.If {
		stmt = p.parseIfStmt()
	} else if p.token.ID == token.While {
		stmt = p.parseWhileStmt()
	} else if p.token.ID == token.For {
		stmt = p.parseForStmt()
	} else if p.token.ID == token.Return {
		stmt = p.parseReturnStmt()
	} else if p.token.ID == token.Break || p.token.ID == token.Continue {
		if !p.inLoop {
			// TODO: This should be done in type checker
			p.error(p.token, "%s can only be used in a loop", p.token.Quote())
		}

		tok := p.token
		p.next()
		stmt = &ir.BranchStmt{Tok: tok}
	} else {
		stmt = p.parseExprOrAssignStmt()
	}
	p.consumeSemi()
	return stmt
}

func (p *parser) parseBlockStmt() *ir.BlockStmt {
	block := &ir.BlockStmt{}
	block.Lbrace = p.token
	p.consume(token.Lbrace)
	for p.token.ID != token.Rbrace && p.token.ID != token.EOF {
		stmt := p.parseStmt()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		}
	}
	block.Rbrace = p.token
	p.consume(token.Rbrace)
	return block
}

func (p *parser) parseValDeclStmt() *ir.DeclStmt {
	d := p.parseValDecl()
	return &ir.DeclStmt{D: d}
}

func (p *parser) parseIfStmt() *ir.IfStmt {
	s := &ir.IfStmt{}
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

func (p *parser) parseWhileStmt() *ir.ForStmt {
	s := &ir.ForStmt{}
	s.For = p.token
	p.next()
	p.inCondition = true
	s.Cond = p.parseExpr()
	p.inCondition = false
	p.inLoop = true
	s.Body = p.parseBlockStmt()
	p.inLoop = false
	return s
}

func (p *parser) parseForStmt() *ir.ForStmt {
	s := &ir.ForStmt{}
	s.For = p.token
	p.next()

	if p.token.ID != token.Semicolon {
		s.Init = &ir.ValDecl{}
		s.Init.Decl = token.Synthetic(token.Var, token.Var.String())

		s.Init.Name = p.token
		p.expect(token.Ident)

		if !p.token.Is(token.Assign) {
			s.Init.Type = p.parseTypeSpec()
		}

		p.expect(token.Assign)
		s.Init.Initializer = p.parseExpr()
	}

	p.expect(token.Semicolon)

	if p.token.ID != token.Semicolon {
		p.inCondition = true
		s.Cond = p.parseExpr()
		p.inCondition = false
	}

	p.expect(token.Semicolon)

	if p.token.ID != token.Lbrace {
		s.Inc = p.parseExprOrAssignStmt()
	}

	p.inLoop = true
	s.Body = p.parseBlockStmt()
	p.inLoop = false

	return s
}

func (p *parser) parseReturnStmt() *ir.ReturnStmt {
	s := &ir.ReturnStmt{}
	s.Return = p.token
	p.next()
	if p.token.ID != token.Semicolon {
		s.X = p.parseExpr()
	}
	return s
}

func (p *parser) parseExprOrAssignStmt() ir.Stmt {
	var stmt ir.Stmt
	expr := p.parseExpr()
	if p.token.IsAssignOperator() || p.token.OneOf(token.Inc, token.Dec) {
		assign := p.token
		p.next()

		var right ir.Expr

		if assign.Is(token.Inc) {
			right = &ir.BasicLit{Value: token.Synthetic(token.Integer, "1")}
			assign.ID = token.AddAssign
		} else if assign.Is(token.Dec) {
			right = &ir.BasicLit{Value: token.Synthetic(token.Integer, "1")}
			assign.ID = token.SubAssign
		} else {
			right = p.parseExpr()
		}

		stmt = &ir.AssignStmt{Left: expr, Assign: assign, Right: right}
	} else {
		stmt = &ir.ExprStmt{X: expr}
	}
	return stmt
}

func (p *parser) parseTypeSpec() ir.Expr {
	if p.token.Is(token.Mul) {
		return p.parsePointerType()
	} else if p.token.Is(token.Lbrack) {
		return p.parseArrayType()
	}
	tok := p.token
	p.expect(token.Ident)
	return &ir.Ident{Name: tok}
}

func (p *parser) parsePointerType() ir.Expr {
	tok := p.token
	p.expect(token.Mul)
	decl := p.token
	if p.token.OneOf(token.Var, token.Val) {
		p.next()
	} else {
		decl = token.Synthetic(token.Val, token.Val.String())
	}
	x := p.parseTypeSpec()
	return &ir.PointerTypeExpr{Pointer: tok, Decl: decl, X: x}
}

func (p *parser) parseArrayType() ir.Expr {
	array := &ir.ArrayTypeExpr{}
	array.Lbrack = p.token
	p.expect(token.Lbrack)

	if !p.token.Is(token.Colon) {
		array.Size = p.parseExpr()
	}

	array.Colon = p.token
	p.expect(token.Colon)

	array.X = p.parseTypeSpec()

	array.Rbrack = p.token
	p.expect(token.Rbrack)
	return array
}

func (p *parser) parseExpr() ir.Expr {
	return p.parseLogicalOr()
}

func (p *parser) parseLogicalOr() ir.Expr {
	expr := p.parseLogicalAnd()
	for p.token.ID == token.Lor {
		op := p.token
		p.next()
		right := p.parseLogicalAnd()
		expr = &ir.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseLogicalAnd() ir.Expr {
	expr := p.parseEquality()
	for p.token.ID == token.Land {
		op := p.token
		p.next()
		right := p.parseEquality()
		expr = &ir.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseEquality() ir.Expr {
	expr := p.parseComparison()
	for p.token.ID == token.Eq || p.token.ID == token.Neq {
		op := p.token
		p.next()
		right := p.parseComparison()
		expr = &ir.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseComparison() ir.Expr {
	expr := p.parseTerm()
	for p.token.ID == token.Gt || p.token.ID == token.GtEq ||
		p.token.ID == token.Lt || p.token.ID == token.LtEq {
		op := p.token
		p.next()
		right := p.parseTerm()
		expr = &ir.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseTerm() ir.Expr {
	expr := p.parseFactor()
	for p.token.ID == token.Add || p.token.ID == token.Sub {
		op := p.token
		p.next()
		right := p.parseFactor()
		expr = &ir.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseFactor() ir.Expr {
	expr := p.parseUnary()
	for p.token.OneOf(token.Mul, token.Div, token.Mod) {
		op := p.token
		p.next()
		right := p.parseUnary()
		expr = &ir.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseUnary() ir.Expr {
	if p.token.OneOf(token.Sub, token.Lnot, token.Mul) {
		op := p.token
		p.next()
		x := p.parseExpr()
		return &ir.UnaryExpr{Op: op, X: x}
	} else if p.token.Is(token.And) {
		return p.parseAddressExpr()
	}
	return p.parseOperand()
}

func (p *parser) parseAddressExpr() ir.Expr {
	and := p.token
	p.expect(token.And)

	decl := p.token
	if p.token.OneOf(token.Var, token.Val) {
		p.next()
	} else {
		decl = token.Synthetic(token.Val, token.Val.String())
	}

	x := p.parseExpr()
	return &ir.AddressExpr{And: and, Decl: decl, X: x}
}

func (p *parser) parseOperand() ir.Expr {
	var x ir.Expr
	if p.token.Is(token.Cast) {
		x = p.parseCastExpr()
	} else if p.token.Is(token.Lparen) {
		p.next()
		x = p.parseExpr()
		p.expect(token.Rparen)
	} else if p.token.Is(token.Lbrack) {
		x = p.parseArrayLit()
	} else if p.token.Is(token.Ident) {
		ident := p.parseIdent()
		if !p.inCondition && p.token.Is(token.Lbrace) {
			x = p.parseStructLit(ident)
		} else if p.token.Is(token.String) {
			x = p.parseBasicLit(ident)
		} else {
			x = ident
		}
	} else {
		x = p.parseBasicLit(nil)
	}
	return p.parsePrimary(x)
}

func (p *parser) parseCastExpr() *ir.CastExpr {
	cast := &ir.CastExpr{}
	cast.Cast = p.token
	p.next()
	cast.Lparen = p.token
	p.expect(token.Lparen)
	cast.ToTyp = p.parseTypeSpec()
	p.expect(token.Comma)
	cast.X = p.parseExpr()
	cast.Rparen = p.token
	p.expect(token.Rparen)
	return cast
}

func (p *parser) parsePrimary(expr ir.Expr) ir.Expr {
	if p.token.Is(token.Lbrack) {
		return p.parseSliceOrIndexExpr(expr)
	} else if p.token.Is(token.Lparen) {
		return p.parsePrimary(p.parseFuncCall(expr))
	} else if p.token.Is(token.Dot) {
		return p.parsePrimary(p.parseDotExpr(expr))
	}
	return expr
}

func (p *parser) parseSliceOrIndexExpr(expr ir.Expr) ir.Expr {
	var index1 ir.Expr
	var index2 ir.Expr
	colon := token.Synthetic(token.Invalid, token.Invalid.String())

	lbrack := p.token
	p.expect(token.Lbrack)

	if !p.token.Is(token.Colon) {
		index1 = p.parseExpr()
	}

	if p.token.Is(token.Colon) {
		colon = p.token
		p.next()
		if !p.token.Is(token.Rbrack) {
			index2 = p.parseExpr()
		}
	}

	rbrack := p.token
	p.expect(token.Rbrack)

	if colon.IsValid() {
		return &ir.SliceExpr{X: expr, Lbrack: lbrack, Start: index1, Colon: colon, End: index2, Rbrack: rbrack}
	}

	res := &ir.IndexExpr{X: expr, Lbrack: lbrack, Index: index1, Rbrack: rbrack}
	return p.parsePrimary(res)
}

func (p *parser) parseIdent() *ir.Ident {
	tok := p.token
	p.consume(token.Ident)
	return &ir.Ident{Name: tok}
}

func (p *parser) parseDotExpr(expr ir.Expr) ir.Expr {
	dot := p.token
	p.consume(token.Dot)
	name := p.parseIdent()
	return &ir.DotExpr{X: expr, Dot: dot, Name: name}
}

func (p *parser) parseFuncCall(expr ir.Expr) ir.Expr {
	lparen := p.token
	p.consume(token.Lparen)
	var args []ir.Expr
	if p.token.ID != token.Rparen {
		args = append(args, p.parseExpr())
		for p.token.ID != token.EOF && p.token.ID != token.Rparen {
			p.consume(token.Comma)
			if p.token.Is(token.Rparen) {
				break
			}
			args = append(args, p.parseExpr())
		}
	}
	rparen := p.token
	p.consume(token.Rparen)
	return &ir.FuncCall{X: expr, Lparen: lparen, Args: args, Rparen: rparen}
}

func (p *parser) parseBasicLit(prefix *ir.Ident) ir.Expr {
	switch p.token.ID {
	case token.Integer, token.Float, token.String, token.True, token.False, token.Null:
		tok := p.token
		p.next()
		return &ir.BasicLit{Prefix: prefix, Value: tok}
	default:
		tok := p.token
		p.error(tok, "got '%s', expected expression", tok.Quote())
		p.next()
		panic(parseError{tok})
	}
}

func (p *parser) parseKeyValue() *ir.KeyValue {
	key := p.token
	p.consume(token.Ident)
	equal := p.token
	p.consume(token.Assign)
	value := p.parseExpr()
	return &ir.KeyValue{Key: key, Equal: equal, Value: value}
}

func (p *parser) parseStructLit(name ir.Expr) ir.Expr {
	lbrace := p.token
	p.expect(token.Lbrace)
	var inits []*ir.KeyValue
	if !p.token.Is(token.Rbrace) {
		inits = append(inits, p.parseKeyValue())
		for p.token.ID != token.EOF && p.token.ID != token.Rbrace {
			p.expect(token.Comma)
			if p.token.Is(token.Rbrace) {
				break
			}
			inits = append(inits, p.parseKeyValue())
		}
	}
	rbrace := p.token
	p.expect(token.Rbrace)
	return &ir.StructLit{Name: name, Lbrace: lbrace, Initializers: inits, Rbrace: rbrace}
}

func (p *parser) parseArrayLit() ir.Expr {
	lbrack := p.token
	p.expect(token.Lbrack)
	var inits []ir.Expr
	if !p.token.Is(token.Rbrack) {
		inits = append(inits, p.parseExpr())
		for p.token.ID != token.EOF && p.token.ID != token.Rbrack {
			p.expect(token.Comma)
			if p.token.Is(token.Rbrack) {
				break
			}
			inits = append(inits, p.parseExpr())
		}
	}
	rbrack := p.token
	p.expect(token.Rbrack)
	return &ir.ArrayLit{Lbrack: lbrack, Initializers: inits, Rbrack: rbrack}
}
