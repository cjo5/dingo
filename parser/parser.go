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

func (p *parser) expect2(id token.ID, sync bool) bool {
	tok := p.token
	p.next()
	if !tok.Is(id) {
		p.error(tok, "got '%s', expected '%s'", tok.Quote(), id)
		if sync {
			panic(parseError{tok})
		}
		return false
	}
	return true
}

func (p *parser) expect1(id token.ID) bool {
	return p.expect2(id, true)
}

func (p *parser) expectSemi1(sync bool) bool {
	if !p.token.OneOf(token.Rbrace, token.Rbrack) {
		tok := p.token
		p.next()

		if !tok.Is(token.Semicolon) {
			p.error(p.token, "got '%s', expected semicolon or newline", tok.Quote())
			if sync {
				panic(parseError{tok})
			}
			return false
		}

		for p.token.Is(token.Semicolon) {
			p.next()
		}
	}
	return true
}

func (p *parser) expectSemi0() bool {
	return p.expectSemi1(true)
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
		p.expect2(token.Ident, false)
		p.expectSemi1(false)
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
	if !p.expect2(token.String, false) {
		return nil
	}
	p.expectSemi1(false)
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
		p.expectSemi0()
	} else if p.token.ID == token.Func {
		decl = p.parseFuncDecl(visibility)
		p.expectSemi0()
	} else if p.token.ID == token.Struct {
		decl = p.parseStructDecl(visibility)
		p.expectSemi0()
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
	p.expect1(token.Ident)

	decl.Type = p.parseType(true)

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
	p.expect1(token.Ident)

	decl.Type = p.parseType(false)
	return decl
}

func (p *parser) parseFuncDecl(visibility token.Token) *ir.FuncDecl {
	decl := &ir.FuncDecl{}
	decl.Visibility = visibility
	decl.Decl = p.token
	p.next()
	decl.Name = p.token
	p.expect1(token.Ident)

	decl.Lparen = p.token
	p.expect1(token.Lparen)
	flags := ir.AstFlagNoInit
	if !p.token.Is(token.Rparen) {
		decl.Params = append(decl.Params, p.parseField(flags, token.Val))
		for !p.token.OneOf(token.EOF, token.Rparen) {
			p.expect1(token.Comma)
			if p.token.Is(token.Rparen) {
				break
			}
			decl.Params = append(decl.Params, p.parseField(flags, token.Val))
		}
	}
	decl.Rparen = p.token
	p.expect1(token.Rparen)

	decl.TReturn = p.parseType(true)
	if decl.TReturn == nil {
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
	p.expect1(token.Ident)

	decl.Lbrace = p.token
	p.expect1(token.Lbrace)
	flags := ir.AstFlagNoInit
	for !p.token.OneOf(token.EOF, token.Rbrace) {
		decl.Fields = append(decl.Fields, p.parseField(flags, token.Var))
		p.expectSemi0()
	}

	decl.Rbrace = p.token
	p.expect1(token.Rbrace)

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
	p.expectSemi0()
	return stmt
}

func (p *parser) parseBlockStmt() *ir.BlockStmt {
	block := &ir.BlockStmt{}
	block.Lbrace = p.token
	p.expect1(token.Lbrace)
	for p.token.ID != token.Rbrace && p.token.ID != token.EOF {
		stmt := p.parseStmt()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		}
	}
	block.Rbrace = p.token
	p.expect1(token.Rbrace)
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
		p.expect1(token.Ident)

		s.Init.Type = p.parseType(true)

		p.expect1(token.Assign)
		s.Init.Initializer = p.parseExpr()
	}

	p.expectSemi0()

	if p.token.ID != token.Semicolon {
		p.inCondition = true
		s.Cond = p.parseExpr()
		p.inCondition = false
	}

	p.expectSemi0()

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
	if p.token.IsAssignOp() || p.token.OneOf(token.Inc, token.Dec) {
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

func (p *parser) parseType(optional bool) ir.Expr {
	if p.token.Is(token.Mul) {
		return p.parsePointerType()
	} else if p.token.Is(token.Lbrack) {
		return p.parseArrayType()
	} else if p.token.Is(token.Func) {
		return p.parseFuncType()
	} else if p.token.Is(token.Ident) {
		tok := p.token
		p.expect1(token.Ident)
		return &ir.Ident{Name: tok}
	} else if p.token.Is(token.Lparen) {
		p.next()
		t := p.parseType(optional)
		if t != nil {
			p.expect1(token.Rparen)
		}
		return t
	} else if optional {
		return nil
	}
	p.error(p.token, "got '%s', expected type", p.token.Quote())
	panic(parseError{p.token})
}

func (p *parser) parsePointerType() ir.Expr {
	tok := p.token
	p.expect1(token.Mul)
	decl := p.token
	if p.token.OneOf(token.Var, token.Val) {
		p.next()
	} else {
		decl = token.Synthetic(token.Val, token.Val.String())
	}
	x := p.parseType(false)
	return &ir.PointerTypeExpr{Pointer: tok, Decl: decl, X: x}
}

func (p *parser) parseArrayType() ir.Expr {
	array := &ir.ArrayTypeExpr{}
	array.Lbrack = p.token
	p.expect1(token.Lbrack)

	if !p.token.Is(token.Colon) {
		array.Size = p.parseExpr()
	}

	array.Colon = p.token
	p.expect1(token.Colon)

	array.X = p.parseType(false)

	array.Rbrack = p.token
	p.expect1(token.Rbrack)
	return array
}

func (p *parser) parseFuncType() ir.Expr {
	fun := &ir.FuncTypeExpr{}

	fun.Fun = p.token
	p.expect1(token.Func)

	fun.Lparen = p.token
	p.expect1(token.Lparen)

	if !p.token.Is(token.Rparen) {
		fun.Params = append(fun.Params, p.parseType(false))
		for !p.token.Is(token.Rparen) {
			p.expect1(token.Comma)
			fun.Params = append(fun.Params, p.parseType(false))
		}
	}

	fun.Rparen = p.token
	p.expect1(token.Rparen)

	fun.Return = p.parseType(true)
	if fun.Return == nil {
		fun.Return = &ir.Ident{Name: token.Synthetic(token.Ident, ir.TVoid.String())}
	}

	return fun
}

func (p *parser) parseExpr() ir.Expr {
	return p.parseBinaryExpr(ir.LowestPrec)
}

func (p *parser) parseBinaryExpr(prec int) ir.Expr {
	expr := p.parseUnaryExpr()
	for p.token.IsBinaryOp() {
		op := p.token
		opPrec := ir.BinaryPrec(op.ID)
		if prec < opPrec {
			break
		}
		p.next()
		right := p.parseBinaryExpr(opPrec - 1)
		expr = &ir.BinaryExpr{Left: expr, Op: op, Right: right}
	}
	return expr
}

func (p *parser) parseUnaryExpr() ir.Expr {
	if p.token.OneOf(token.Sub, token.Lnot, token.Mul) {
		op := p.token
		p.next()
		x := p.parseOperand()
		return &ir.UnaryExpr{Op: op, X: x}
	} else if p.token.Is(token.And) {
		and := p.token
		p.expect1(token.And)

		decl := p.token
		if p.token.OneOf(token.Var, token.Val) {
			p.next()
		} else {
			decl = token.Synthetic(token.Val, token.Val.String())
		}

		x := p.parseOperand()
		return &ir.AddressExpr{And: and, Decl: decl, X: x}
	}
	return p.parseOperand()
}

func (p *parser) parseOperand() ir.Expr {
	var expr ir.Expr
	if p.token.Is(token.Cast) {
		expr = p.parseCastExpr()
	} else if p.token.Is(token.Lenof) {
		expr = p.parseLenExpr()
	} else if p.token.Is(token.Lparen) {
		p.next()
		expr = p.parseExpr()
		p.expect1(token.Rparen)
	} else if p.token.Is(token.Lbrack) {
		expr = p.parseArrayLit()
	} else if p.token.Is(token.Ident) {
		ident := p.parseIdent()
		if !p.inCondition && p.token.Is(token.Lbrace) {
			expr = p.parseStructLit(ident)
		} else if p.token.Is(token.String) {
			expr = p.parseBasicLit(ident)
		} else {
			expr = ident
		}
	} else {
		expr = p.parseBasicLit(nil)
	}
	return p.parsePrimary(expr)
}

func (p *parser) parseCastExpr() *ir.CastExpr {
	cast := &ir.CastExpr{}
	cast.Cast = p.token
	p.next()
	cast.Lparen = p.token
	p.expect1(token.Lparen)
	cast.ToTyp = p.parseType(false)
	p.expect1(token.Comma)
	cast.X = p.parseExpr()
	cast.Rparen = p.token
	p.expect1(token.Rparen)
	return cast
}

func (p *parser) parseLenExpr() *ir.LenExpr {
	lenof := &ir.LenExpr{}
	lenof.Len = p.token
	p.next()
	lenof.Lparen = p.token
	p.expect1(token.Lparen)
	lenof.X = p.parseExpr()
	lenof.Rparen = p.token
	p.expect1(token.Rparen)
	return lenof
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
	p.expect1(token.Lbrack)

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
	p.expect1(token.Rbrack)

	if colon.IsValid() {
		return &ir.SliceExpr{X: expr, Lbrack: lbrack, Start: index1, Colon: colon, End: index2, Rbrack: rbrack}
	}

	res := &ir.IndexExpr{X: expr, Lbrack: lbrack, Index: index1, Rbrack: rbrack}
	return p.parsePrimary(res)
}

func (p *parser) parseIdent() *ir.Ident {
	tok := p.token
	p.expect1(token.Ident)
	return &ir.Ident{Name: tok}
}

func (p *parser) parseDotExpr(expr ir.Expr) ir.Expr {
	dot := p.token
	p.expect1(token.Dot)
	name := p.parseIdent()
	return &ir.DotExpr{X: expr, Dot: dot, Name: name}
}

func (p *parser) parseFuncCall(expr ir.Expr) ir.Expr {
	lparen := p.token
	p.expect1(token.Lparen)
	var args []ir.Expr
	if p.token.ID != token.Rparen {
		args = append(args, p.parseExpr())
		for p.token.ID != token.EOF && p.token.ID != token.Rparen {
			p.expect1(token.Comma)
			if p.token.Is(token.Rparen) {
				break
			}
			args = append(args, p.parseExpr())
		}
	}
	rparen := p.token
	p.expect1(token.Rparen)
	return &ir.FuncCall{X: expr, Lparen: lparen, Args: args, Rparen: rparen}
}

func (p *parser) parseBasicLit(prefix *ir.Ident) ir.Expr {
	switch p.token.ID {
	case token.Integer, token.Float, token.String, token.True, token.False, token.Null:
		tok := p.token
		p.next()

		var suffix *ir.Ident
		if tok.OneOf(token.Integer, token.Float) && p.token.Is(token.Ident) {
			suffix = p.parseIdent()
		}

		return &ir.BasicLit{Prefix: prefix, Value: tok, Suffix: suffix}
	default:
		tok := p.token
		p.error(tok, "got '%s', expected expression", tok.Quote())
		p.next()
		panic(parseError{tok})
	}
}

func (p *parser) parseKeyValue() *ir.KeyValue {
	key := p.token
	p.expect1(token.Ident)
	equal := p.token
	p.expect1(token.Assign)
	value := p.parseExpr()
	return &ir.KeyValue{Key: key, Equal: equal, Value: value}
}

func (p *parser) parseStructLit(name ir.Expr) ir.Expr {
	lbrace := p.token
	p.expect1(token.Lbrace)
	var inits []*ir.KeyValue
	if !p.token.Is(token.Rbrace) {
		inits = append(inits, p.parseKeyValue())
		for p.token.ID != token.EOF && p.token.ID != token.Rbrace {
			p.expect1(token.Comma)
			if p.token.Is(token.Rbrace) {
				break
			}
			inits = append(inits, p.parseKeyValue())
		}
	}
	rbrace := p.token
	p.expect1(token.Rbrace)
	return &ir.StructLit{Name: name, Lbrace: lbrace, Initializers: inits, Rbrace: rbrace}
}

func (p *parser) parseArrayLit() ir.Expr {
	lbrack := p.token
	p.expect1(token.Lbrack)
	var inits []ir.Expr
	if !p.token.Is(token.Rbrack) {
		inits = append(inits, p.parseExpr())
		for p.token.ID != token.EOF && p.token.ID != token.Rbrack {
			p.expect1(token.Comma)
			if p.token.Is(token.Rbrack) {
				break
			}
			inits = append(inits, p.parseExpr())
		}
	}
	rbrack := p.token
	p.expect1(token.Rbrack)
	return &ir.ArrayLit{Lbrack: lbrack, Initializers: inits, Rbrack: rbrack}
}
