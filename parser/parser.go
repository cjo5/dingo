package parser

import (
	"bytes"
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

	if p.errors.IsError() {
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

	file  *ir.File
	decls []ir.TopDecl

	token       token.Token
	literal     string
	prev        token.Token
	inCondition bool
	loopCount   int
	blockCount  int

	funcName        string
	funcAnonCount   int
	globalAnonCount int
}

func (p *parser) init(src []byte, filename string) {
	p.errors = &common.ErrorList{}
	p.funcName = ""
	p.lexer.init(src, filename, p.errors)
}

func (p *parser) next() {
	for {
		p.prev = p.token
		p.token, p.literal = p.lexer.lex()
		if p.token.ID != token.Comment && p.token.ID != token.MultiComment {
			break
		}
	}
}

func (p *parser) error(tok token.Token, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	p.errors.Add(tok.Pos, msg)
}

func (p *parser) sync() {
	lbrace := p.blockCount
	semi := false
	p.inCondition = false
	p.blockCount = 0
	p.loopCount = 0
	for {
		switch p.token.ID {
		case token.Public, token.Private,
			token.Var, token.Val, token.Func, token.Struct,
			token.Return, token.If, token.While, token.For, token.Break, token.Continue:
			if semi && lbrace == 0 {
				return
			}
		case token.Lbrace:
			lbrace++
		case token.Rbrace:
			if lbrace > 0 {
				lbrace--
			} else if lbrace == 0 {
				return
			}
		case token.EOF:
			return
		}
		semi = p.token.Is(token.Semicolon)
		p.next()
	}
}

func (p *parser) expect3(expected token.ID, alts []token.ID, sync bool) bool {
	tok := p.token
	p.next()
	if !tok.Is(expected) {
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("'%s'", expected))

		for i, alt := range alts {
			if (i + 1) < len(alts) {
				buf.WriteString(fmt.Sprintf(", '%s'", alt))
			} else {
				buf.WriteString(fmt.Sprintf(" or '%s'", alt))
			}
		}

		p.error(tok, "expected %s", buf.String())

		if sync {
			panic(parseError{tok})
		}

		return false
	}
	return true
}

func (p *parser) expect(id token.ID, alts ...token.ID) bool {
	return p.expect3(id, alts, true)
}

func (p *parser) expectSemi1(sync bool) bool {
	if !p.token.OneOf(token.Rbrace, token.Rbrack) {
		tok := p.token
		p.next()

		if !tok.OneOf(token.Semicolon, token.EOF) {
			p.error(tok, "expected semicolon or newline")
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

func (p *parser) expectSemi() bool {
	return p.expectSemi1(true)
}

func (p *parser) parseFile() (*ir.File, []ir.TopDecl) {
	p.file = &ir.File{Filename: p.lexer.filename}

	if p.token.Is(token.Module) {
		p.file.Decl = p.token
		p.next()
		p.file.ModName = p.parseModName()
		p.expectSemi1(false)
	} else {
		p.file.Decl = token.Synthetic(token.Module)
	}

	for p.token.Is(token.Require) {
		dep := p.parseRequire()
		if dep != nil {
			p.file.FileDeps = append(p.file.FileDeps, dep)
		}
	}

	for !p.token.Is(token.EOF) {
		directives := p.parseDirectives(nil)

		visibility := token.Synthetic(token.Private)
		if p.token.OneOf(token.Public, token.Private) {
			visibility = p.token
			p.next()
		}

		directives = p.parseDirectives(directives)

		if p.token.Is(token.Import) {
			dep := p.parseImport(directives, visibility)
			if dep != nil {
				p.file.ModDeps = append(p.file.ModDeps, dep)
			}
		} else {
			decl := p.parseTopDecl(directives, visibility)
			if decl != nil {
				p.decls = append(p.decls, decl)
			}
		}
	}

	return p.file, p.decls
}

func (p *parser) parseModName() ir.Expr {
	ident := p.parseIdent()
	if p.token.Is(token.Dot) {
		return p.parseDotExpr(ident)
	}
	return ident
}

func (p *parser) parseRequire() *ir.FileDependency {
	decl := p.token
	p.next()
	tok := p.token
	lit := p.literal
	if !p.expect3(token.String, nil, false) {
		return nil
	}
	p.expectSemi1(false)
	return &ir.FileDependency{Decl: decl, Literal: &ir.BasicLit{Tok: tok, Value: lit}}
}

func (p *parser) parseImport(directives []ir.Directive, visibility token.Token) (dep *ir.ModuleDependency) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(parseError); ok {
				p.sync()
				dep = nil
			} else {
				panic(r)
			}
		}
	}()

	decl := p.token
	p.next()

	var name ir.Expr
	name = p.parseIdent()
	if p.token.Is(token.Dot) {
		name = p.parseDotExpr(name)
	}

	p.expectSemi()
	dep = &ir.ModuleDependency{Directives: directives, Visibility: visibility, Decl: decl, ModName: name}

	return dep
}

func (p *parser) parseTopDecl(directives []ir.Directive, visibility token.Token) (decl ir.TopDecl) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(parseError); ok {
				p.sync()
				decl = nil
			} else {
				panic(r)
			}
		}
	}()

	if p.token.OneOf(token.Var, token.Val) {
		decl = p.parseValTopDecl(visibility, directives)
		p.expectSemi()
	} else if p.token.Is(token.Func) {
		decl = p.parseFuncDecl(visibility, directives)
		p.expectSemi()
	} else if p.token.Is(token.Struct) {
		decl = p.parseStructDecl(visibility, directives)
		p.expectSemi()
	} else {
		p.error(p.token, "expected declaration")
		p.next()
		p.sync()
	}

	return decl
}

func (p *parser) parseDirectives(directives []ir.Directive) []ir.Directive {
	for p.token.Is(token.Directive) {
		dir := p.token
		p.next()
		name := p.parseIdent()
		directives = append(directives, ir.Directive{Directive: dir, Name: name})
	}
	return directives
}

func (p *parser) parseValTopDecl(visibility token.Token, directives []ir.Directive) *ir.ValTopDecl {
	decl := &ir.ValTopDecl{}
	decl.Directives = directives
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

	decl.Name = p.parseIdent()
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
		decl.Decl = token.Synthetic(defaultDecl)
	}

	tok := p.token
	lit := p.literal

	if p.token.Is(token.Underscore) {
		p.next()
		decl.Type = p.parseType(false)
	} else if decl.Decl.IsValid() {
		p.expect(token.Ident)
		decl.Type = p.parseType(false)
	} else {
		optional := false
		if p.token.Is(token.Ident) {
			p.next()
			optional = true
		}
		decl.Type = p.parseType(optional)
		if decl.Type == nil {
			decl.Type = &ir.Ident{Tok: tok, Literal: lit}
			tok = token.Synthetic(token.Underscore)
			lit = token.Underscore.String()
		}
	}

	decl.Name = ir.NewIdent2(tok, lit)

	return decl
}

func (p *parser) parseFuncDecl(visibility token.Token, directives []ir.Directive) *ir.FuncDecl {
	decl := &ir.FuncDecl{}
	decl.Directives = directives
	decl.Visibility = visibility
	decl.Decl = p.token
	p.next()

	if p.token.Is(token.Lbrack) {
		p.next()
		decl.ABI = p.parseIdent()
		p.expect(token.Rbrack)

		if !decl.Visibility.IsValid() {
			if decl.ABI.Literal == ir.CABI {
				// C functions have public as default visibility
				decl.Visibility = token.Synthetic(token.Public)
			}
		}
	}

	decl.Name = p.parseIdent()
	decl.Lparen, decl.Params, decl.Return, decl.Rparen = p.parseFuncSignature()

	if p.token.Is(token.Semicolon) {
		return decl
	}

	p.funcName = decl.Name.Literal
	p.funcAnonCount = 0
	decl.Body = p.parseBlock(false)
	p.funcName = ""

	return decl
}

func (p *parser) parseFuncSignature() (lparen token.Token, params []*ir.ValDecl, ret *ir.ValDecl, rparen token.Token) {
	lparen = p.token
	p.expect(token.Lparen)

	if !p.token.Is(token.Rparen) {
		flags := ir.AstFlagNoInit
		params = append(params, p.parseField(flags, token.Val))
		for !p.token.OneOf(token.EOF, token.Rparen) {
			p.expect(token.Comma)
			if p.token.Is(token.Rparen) {
				break
			}
			params = append(params, p.parseField(flags, token.Val))
		}
	}

	rparen = p.token
	p.expect(token.Rparen)

	ret = &ir.ValDecl{}
	ret.Decl = token.Synthetic(token.Val)
	ret.Name = ir.NewIdent2(token.Synthetic(token.Underscore), token.Underscore.String())

	ret.Type = p.parseType(true)
	if ret.Type == nil {
		ret.Type = ir.NewIdent2(token.Synthetic(token.Ident), ir.TVoid.String())
	}

	return
}

func (p *parser) parseStructDecl(visibility token.Token, directives []ir.Directive) *ir.StructDecl {
	decl := &ir.StructDecl{}
	decl.Directives = directives
	decl.Visibility = visibility
	decl.Decl = p.token
	p.next()
	decl.Name = p.parseIdent()

	decl.Lbrace = p.token
	p.expect(token.Lbrace)
	p.blockCount++

	flags := ir.AstFlagNoInit
	for !p.token.OneOf(token.EOF, token.Rbrace) {
		decl.Fields = append(decl.Fields, p.parseField(flags, token.Var))
		p.expectSemi()
	}

	decl.Rbrace = p.token
	p.expect(token.Rbrace)
	p.blockCount--

	return decl
}

func (p *parser) parseStmt() (stmt ir.Stmt, sync bool) {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(parseError); ok {
				p.sync()
				stmt = &ir.BadStmt{From: err.tok, To: p.prev}
				sync = true
			} else {
				panic(r)
			}
		}
	}()

	sync = false

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
		if p.loopCount == 0 {
			// TODO: This should be done in type checker
			p.error(p.token, "%s can only be used in a loop", token.Quote(p.literal))
		}

		tok := p.token
		p.next()
		stmt = &ir.BranchStmt{Tok: tok}
	} else {
		stmt = p.parseExprOrAssignStmt()
	}
	p.expectSemi()
	return stmt, sync
}

func (p *parser) parseBlockStmt() *ir.BlockStmt {
	return p.parseBlock(true)
}

func (p *parser) parseBlock(incBody bool) *ir.BlockStmt {
	block := &ir.BlockStmt{}

	block.Lbrace = p.token
	p.expect(token.Lbrace)

	if incBody {
		p.blockCount++
	}

	didSync := false

	for p.token.ID != token.Rbrace && p.token.ID != token.EOF {
		stmt, sync := p.parseStmt()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		}
		if sync {
			didSync = true
		}
	}

	if !p.token.Is(token.Rbrace) && didSync {
		block.Rbrace = token.Synthetic(token.Rbrace)
	} else {
		block.Rbrace = p.token
		p.expect(token.Rbrace)
	}

	if incBody {
		p.blockCount--
	}

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
	p.loopCount++
	s.Body = p.parseBlockStmt()
	p.loopCount--
	return s
}

func (p *parser) parseForStmt() *ir.ForStmt {
	s := &ir.ForStmt{}
	s.For = p.token
	p.next()

	if p.token.ID != token.Semicolon {
		s.Init = &ir.ValDecl{}
		s.Init.Decl = token.Synthetic(token.Var)

		s.Init.Name = p.parseIdent()
		s.Init.Type = p.parseType(true)

		p.expect(token.Assign)
		s.Init.Initializer = p.parseExpr()
	}

	p.expectSemi()

	if p.token.ID != token.Semicolon {
		p.inCondition = true
		s.Cond = p.parseExpr()
		p.inCondition = false
	}

	p.expectSemi()

	if p.token.ID != token.Lbrace {
		s.Inc = p.parseExprOrAssignStmt()
	}

	p.loopCount++
	s.Body = p.parseBlockStmt()
	p.loopCount--

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
			right = &ir.BasicLit{Tok: token.Synthetic(token.Integer), Value: "1"}
			assign.ID = token.AddAssign
		} else if assign.Is(token.Dec) {
			right = &ir.BasicLit{Tok: token.Synthetic(token.Integer), Value: "1"}
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
	} else if p.token.OneOf(token.Func) {
		return p.parseFuncType()
	} else if p.token.Is(token.Ident) {
		var expr ir.Expr = p.parseIdent()
		for p.token.Is(token.Dot) {
			expr = p.parseDotExpr(expr)
		}
		return expr
	} else if p.token.Is(token.Lparen) {
		p.next()
		t := p.parseType(optional)
		if t != nil {
			p.expect(token.Rparen)
		}
		return t
	} else if optional {
		return nil
	}
	p.error(p.token, "expected type")
	p.next()
	panic(parseError{p.token})
}

func (p *parser) parsePointerType() ir.Expr {
	tok := p.token
	p.expect(token.Mul)
	decl := p.token
	if p.token.OneOf(token.Var, token.Val) {
		p.next()
	} else {
		decl = token.Synthetic(token.Val)
	}
	x := p.parseType(false)
	return &ir.PointerTypeExpr{Pointer: tok, Decl: decl, X: x}
}

func (p *parser) parseArrayType() ir.Expr {
	array := &ir.ArrayTypeExpr{}
	array.Lbrack = p.token
	p.expect(token.Lbrack)

	array.X = p.parseType(false)

	if p.token.Is(token.Colon) {
		p.next()
		array.Size = p.parseExpr()
	}

	array.Rbrack = p.token
	p.expect(token.Rbrack)
	return array
}

func (p *parser) parseFuncType() ir.Expr {
	fun := &ir.FuncTypeExpr{}

	fun.Fun = p.token
	p.expect(token.Func)

	if p.token.Is(token.Lbrack) {
		p.next()
		fun.ABI = p.parseIdent()
		p.expect(token.Rbrack)
	}

	fun.Lparen, fun.Params, fun.Return, fun.Rparen = p.parseFuncSignature()
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
		p.expect(token.And)

		decl := p.token
		if p.token.OneOf(token.Var, token.Val) {
			p.next()
		} else {
			decl = token.Synthetic(token.Val)
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
		p.expect(token.Rparen)
	} else if p.token.Is(token.Lbrack) {
		expr = p.parseArrayLit()
	} else if p.token.Is(token.Func) {
		expr = p.parseFuncLit()
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
	p.expect(token.Lparen)
	cast.ToTyp = p.parseType(false)
	p.expect(token.Comma)
	cast.X = p.parseExpr()
	cast.Rparen = p.token
	p.expect(token.Rparen)
	return cast
}

func (p *parser) parseLenExpr() *ir.LenExpr {
	lenof := &ir.LenExpr{}
	lenof.Len = p.token
	p.next()
	lenof.Lparen = p.token
	p.expect(token.Lparen)
	lenof.X = p.parseExpr()
	lenof.Rparen = p.token
	p.expect(token.Rparen)
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
	colon := token.Synthetic(token.Invalid)

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
	lit := p.literal
	p.expect(token.Ident)
	return ir.NewIdent2(tok, lit)
}

func (p *parser) parseDotExpr(expr ir.Expr) ir.Expr {
	dot := p.token
	p.expect(token.Dot)
	name := p.parseIdent()
	return &ir.DotExpr{X: expr, Dot: dot, Name: name}
}

func (p *parser) parseFuncCall(expr ir.Expr) ir.Expr {
	lparen := p.token
	p.expect(token.Lparen)
	var args []ir.Expr
	if p.token.ID != token.Rparen {
		args = append(args, p.parseExpr())
		for p.token.ID != token.EOF && p.token.ID != token.Rparen {
			p.expect(token.Comma)
			if p.token.Is(token.Rparen) {
				break
			}
			args = append(args, p.parseExpr())
		}
	}
	rparen := p.token
	p.expect(token.Rparen)
	return &ir.FuncCall{X: expr, Lparen: lparen, Args: args, Rparen: rparen}
}

func (p *parser) parseBasicLit(prefix *ir.Ident) ir.Expr {
	switch p.token.ID {
	case token.Integer, token.Float, token.String, token.True, token.False, token.Null:
		tok := p.token
		lit := p.literal
		p.next()

		var suffix *ir.Ident
		if tok.OneOf(token.Integer, token.Float) && p.token.Is(token.Ident) {
			suffix = p.parseIdent()
		}

		return &ir.BasicLit{Prefix: prefix, Tok: tok, Value: lit, Suffix: suffix}
	default:
		tok := p.token
		p.error(tok, "expected expression")
		p.next()
		panic(parseError{tok})
	}
}

func (p *parser) parseKeyValue() *ir.KeyValue {
	key := p.parseIdent()
	equal := p.token
	p.expect(token.Assign)
	value := p.parseExpr()
	return &ir.KeyValue{Key: key, Equal: equal, Value: value}
}

func (p *parser) parseStructLit(name ir.Expr) ir.Expr {
	lbrace := p.token
	p.expect(token.Lbrace)
	p.blockCount++
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
	p.blockCount--
	return &ir.StructLit{Name: name, Lbrace: lbrace, Initializers: inits, Rbrace: rbrace}
}

func (p *parser) parseArrayLit() ir.Expr {
	lbrack := p.token
	p.expect(token.Lbrack)
	var inits []ir.Expr
	if !p.token.Is(token.Rbrack) {
		inits = append(inits, p.parseExpr())
		for p.token.ID != token.EOF && p.token.ID != token.Rbrack {
			p.expect(token.Comma, token.Rbrack)
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

func (p *parser) parseFuncLit() ir.Expr {
	decl := &ir.FuncDecl{}
	decl.Flags = ir.AstFlagAnon

	name := ""
	if len(p.funcName) > 0 {
		name = fmt.Sprintf("$%s_anon%d", p.funcName, p.funcAnonCount)
		p.funcAnonCount++
	} else {
		name = fmt.Sprintf("$anon%d", p.globalAnonCount)
		p.globalAnonCount++
	}

	decl.Name = ir.NewIdent2(token.Synthetic(token.Ident), name)

	decl.Visibility = token.Synthetic(token.Private)
	decl.Decl = p.token
	p.expect(token.Func)
	if p.token.Is(token.Lbrack) {
		p.next()
		decl.ABI = p.parseIdent()
		p.expect(token.Rbrack)
	}

	decl.Lparen, decl.Params, decl.Return, decl.Rparen = p.parseFuncSignature()
	decl.Body = p.parseBlockStmt()

	p.decls = append(p.decls, decl)

	return decl.Name
}
