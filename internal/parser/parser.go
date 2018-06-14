package parser

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
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
	p := newParser(src, filepath)
	p.parseFile()

	if p.errors.IsError() {
		return p.file, p.decls, p.errors
	}

	return p.file, p.decls, nil
}

type parseError struct {
	tok token.Token
}

type parser struct {
	lexer  lexer
	errors *common.ErrorList

	file  *ir.File
	decls []ir.TopDecl

	prev    token.Token
	token   token.Token
	pos     token.Position
	literal string

	loopCount  int
	blockCount int

	funcName        string
	funcAnonCount   int
	globalAnonCount int
}

func newParser(src []byte, filename string) *parser {
	p := &parser{}
	p.errors = &common.ErrorList{}
	p.file = &ir.File{Filename: filename}
	p.funcName = ""
	p.lexer.init(src, filename, p.errors)
	p.next()
	return p
}

func (p *parser) next() {
	for {
		p.prev = p.token
		p.token, p.pos, p.literal = p.lexer.lex()
		if p.token.OneOf(token.Comment, token.MultiComment) {
			p.file.Comments = append(p.file.Comments, &ir.Comment{Tok: p.token, Pos: p.pos, Literal: p.literal})
		} else {
			break
		}
	}
}

func (p *parser) error(pos token.Position, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	p.errors.Add(pos, msg)
}

func (p *parser) sync() {
	lbrace := p.blockCount
	semi := false
	p.blockCount = 0
	p.loopCount = 0
	for {
		switch p.token {
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

func (p *parser) expect3(expected token.Token, alts []token.Token, sync bool) bool {
	ok := true
	if !p.token.Is(expected) {
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("'%s'", expected))

		for i, alt := range alts {
			if (i + 1) < len(alts) {
				buf.WriteString(fmt.Sprintf(", '%s'", alt))
			} else {
				buf.WriteString(fmt.Sprintf(" or '%s'", alt))
			}
		}

		p.error(p.pos, "expected %s", buf.String())

		if sync {
			panic(parseError{p.token})
		}

		ok = false
	}
	p.next()
	return ok
}

func (p *parser) expect(id token.Token, alts ...token.Token) bool {
	return p.expect3(id, alts, true)
}

func (p *parser) expectSemi1(sync bool) bool {
	ok := true
	if !p.token.OneOf(token.Rbrace, token.Rbrack) {
		if !p.token.OneOf(token.Semicolon, token.EOF) {
			p.error(p.pos, "expected semicolon or newline")
			if sync {
				panic(parseError{p.token})
			}
			ok = false
		}
		p.next()
		for p.token.Is(token.Semicolon) {
			p.next()
		}
	}
	return ok
}

func (p *parser) expectSemi() bool {
	return p.expectSemi1(true)
}

func (p *parser) parseFile() {
	p.file.SetRange(token.NoPosition, token.NoPosition)
	if p.token.Is(token.Module) {
		p.file.SetPos(p.pos)
		p.next()
		p.file.ModName = p.parseName()
		p.file.SetEndPos(p.pos)
		p.expectSemi1(false)
	}

	for p.token.Is(token.Include) {
		dep := p.parseInclude()
		if dep != nil {
			p.file.FileDeps = append(p.file.FileDeps, dep)
		}
	}

	for !p.token.Is(token.EOF) {
		directives := p.parseDirectives(nil)

		visibilityPos := token.NoPosition
		visibility := token.Private
		if p.token.OneOf(token.Public, token.Private) {
			visibilityPos = p.pos
			visibility = p.token
			p.next()
		}

		directives = p.parseDirectives(directives)

		if p.token.Is(token.Semicolon) {
			p.next()
		} else if p.token.Is(token.Import) {
			dep := p.parseImport(directives, visibility)
			if dep != nil {
				p.file.ModDeps = append(p.file.ModDeps, dep)
			}
		} else {
			decl := p.parseTopDecl(visibilityPos, visibility, directives)
			if decl != nil {
				p.decls = append(p.decls, decl)
			}
		}
	}
}

func (p *parser) parseInclude() *ir.FileDependency {
	dep := &ir.FileDependency{}
	dep.SetPos(p.pos)
	p.next()
	dep.Literal = &ir.BasicLit{Tok: p.token, Value: p.literal}
	if !p.expect3(token.String, nil, false) {
		return nil
	}
	dep.SetEndPos(p.pos)
	p.expectSemi1(false)
	return dep
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

	dep = &ir.ModuleDependency{Directives: directives, Visibility: visibility}

	dep.SetPos(p.pos)
	p.next()

	dep.ModName = p.parseIdent()
	if p.token.Is(token.Dot) {
		dep.ModName = p.parseDotExpr(dep.ModName)
	}

	if p.token.Is(token.As) {
		p.next()
		dep.Alias = p.parseIdent()
	} else {
		parts := strings.Split(ir.ExprNameToText(dep.ModName), ".")
		dep.Alias = ir.NewIdent2(token.Ident, parts[len(parts)-1])
	}

	dep.SetEndPos(p.pos)
	p.expectSemi()

	return dep
}

func (p *parser) parseTopDecl(visibilityPos token.Position, visibility token.Token, directives []ir.Directive) (decl ir.TopDecl) {
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

	if p.token.OneOf(token.Const, token.Var, token.Val) {
		decl = p.parseValTopDecl(visibilityPos, visibility, directives)
		p.expectSemi()
	} else if p.token.Is(token.Func) {
		decl = p.parseFuncDecl(visibilityPos, visibility, directives)
		p.expectSemi()
	} else if p.token.Is(token.Struct) {
		decl = p.parseStructDecl(visibilityPos, visibility, directives)
		p.expectSemi()
	} else {
		p.error(p.pos, "expected declaration")
		p.next()
		p.sync()
	}

	return decl
}

func (p *parser) parseDirectives(directives []ir.Directive) []ir.Directive {
	for p.token.Is(token.Directive) {
		p.next()
		name := p.parseIdent()
		directives = append(directives, ir.Directive{Name: name})
	}
	return directives
}

func (p *parser) parseValTopDecl(visibilityPos token.Position, visibility token.Token, directives []ir.Directive) *ir.ValTopDecl {
	decl := &ir.ValTopDecl{}
	if visibilityPos.IsValid() {
		decl.SetPos(visibilityPos)
	} else {
		decl.SetPos(p.pos)
	}
	decl.Visibility = visibility
	decl.Directives = directives
	decl.ValDeclSpec = p.parseValDeclSpec()
	decl.SetEndPos(p.pos)
	return decl
}

func (p *parser) parseValDecl() *ir.ValDecl {
	decl := &ir.ValDecl{}
	decl.SetPos(p.pos)
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
	} else if decl.Type == nil {
		p.error(p.pos, "expected type or assignment")
		p.next()
		panic(parseError{p.token})
	}

	return decl
}

func (p *parser) parseField(flags int, defaultDecl token.Token) *ir.ValDecl {
	decl := &ir.ValDecl{}
	decl.SetPos(p.pos)
	decl.Flags = flags

	isdecl := false
	if p.token.OneOf(token.Val, token.Var) {
		isdecl = true
		decl.Decl = p.token
		p.next()
	} else {
		decl.Decl = defaultDecl
	}

	tok := p.token
	lit := p.literal

	if p.token.Is(token.Underscore) {
		p.next()
		decl.Type = p.parseType(false)
	} else if isdecl {
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
			tok = token.Underscore
			lit = token.Underscore.String()
		}
	}

	decl.Name = ir.NewIdent2(tok, lit)

	return decl
}

func (p *parser) parseFuncDecl(visibilityPos token.Position, visibility token.Token, directives []ir.Directive) *ir.FuncDecl {
	decl := &ir.FuncDecl{}
	if visibilityPos.IsValid() {
		decl.SetPos(visibilityPos)
	} else {
		decl.SetPos(p.pos)
	}
	decl.Visibility = visibility
	decl.Directives = directives
	p.next()

	if p.token.Is(token.Lbrack) {
		p.next()
		decl.ABI = p.parseIdent()
		p.expect(token.Rbrack)

		if !visibilityPos.IsValid() {
			if decl.ABI.Literal == ir.CABI {
				// C functions have public as default visibility
				decl.Visibility = token.Public
			}
		}
	}

	decl.Name = p.parseIdent()
	decl.Params, decl.Return = p.parseFuncSignature()
	decl.SetEndPos(p.pos)

	if p.token.Is(token.Semicolon) {
		return decl
	}

	p.funcName = decl.Name.Literal
	p.funcAnonCount = 0
	decl.Body = p.parseBlock(false)
	p.funcName = ""

	return decl
}

func (p *parser) parseFuncSignature() (params []*ir.ValDecl, ret *ir.ValDecl) {
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
	p.expect(token.Rparen)
	ret = &ir.ValDecl{}
	ret.SetPos(p.pos)
	ret.Decl = token.Val
	ret.Name = ir.NewIdent2(token.Underscore, token.Underscore.String())
	ret.Type = p.parseType(true)
	if ret.Type == nil {
		ret.Type = ir.NewIdent2(token.Ident, ir.TVoid.String())
	}
	ret.SetPos(p.pos)
	return
}

func (p *parser) parseStructDecl(visibilityPos token.Position, visibility token.Token, directives []ir.Directive) *ir.StructDecl {
	decl := &ir.StructDecl{}
	if visibilityPos.IsValid() {
		decl.SetPos(visibilityPos)
	} else {
		decl.SetPos(p.pos)
	}
	decl.Visibility = visibility
	decl.Directives = directives
	p.next()
	decl.Name = p.parseIdent()
	decl.SetEndPos(p.pos)
	p.expect(token.Lbrace)
	p.blockCount++
	flags := ir.AstFlagNoInit
	for !p.token.OneOf(token.EOF, token.Rbrace) {
		decl.Fields = append(decl.Fields, p.parseField(flags, token.Var))
		p.expectSemi()
	}
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

	if p.token.Is(token.Semicolon) {
		stmt = nil
	} else if p.token.Is(token.Lbrace) {
		stmt = p.parseBlockStmt()
	} else if p.token.OneOf(token.Const, token.Var, token.Val) {
		stmt = p.parseValDeclStmt()
	} else if p.token.Is(token.If) {
		stmt = p.parseIfStmt()
	} else if p.token.Is(token.While) {
		stmt = p.parseWhileStmt()
	} else if p.token.Is(token.For) {
		stmt = p.parseForStmt()
	} else if p.token.Is(token.Return) {
		stmt = p.parseReturnStmt()
	} else if p.token.OneOf(token.Break, token.Continue) {
		if p.loopCount == 0 {
			// TODO: This should be done in type checker
			p.error(p.pos, "%s can only be used in a loop", token.Quote(p.literal))
		}

		stmt = &ir.BranchStmt{Tok: p.token}
		stmt.SetPos(p.pos)
		p.next()
	} else if !p.token.Is(token.Semicolon) {
		stmt = p.parseExprOrAssignStmt()
	}
	stmt.SetEndPos(p.pos)
	p.expectSemi()
	return stmt, sync
}

func (p *parser) parseBlockStmt() *ir.BlockStmt {
	return p.parseBlock(true)
}

func (p *parser) parseBlock(incBody bool) *ir.BlockStmt {
	block := &ir.BlockStmt{}
	block.SetPos(p.pos)

	p.expect(token.Lbrace)

	if incBody {
		p.blockCount++
	}

	didSync := false

	for p.token != token.Rbrace && p.token != token.EOF {
		stmt, sync := p.parseStmt()
		if stmt != nil {
			block.Stmts = append(block.Stmts, stmt)
		}
		if sync {
			didSync = true
		}
	}

	if p.token.Is(token.Rbrace) || !didSync {
		p.expect(token.Rbrace)
	}

	block.SetEndPos(p.pos)

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
	s.SetPos(p.pos)
	p.next()
	s.Cond = p.parseCondition()
	s.Body = p.parseBlockStmt()
	if p.token == token.Elif {
		s.Else = p.parseIfStmt()
	} else if p.token == token.Else {
		p.next() // We might wanna save this token...
		s.Else = p.parseBlockStmt()
	}
	return s
}

func (p *parser) parseWhileStmt() *ir.ForStmt {
	s := &ir.ForStmt{}
	s.SetPos(p.pos)
	p.next()
	s.Cond = p.parseCondition()
	p.loopCount++
	s.Body = p.parseBlockStmt()
	p.loopCount--
	return s
}

func (p *parser) parseForStmt() *ir.ForStmt {
	s := &ir.ForStmt{}
	s.SetPos(p.pos)
	p.next()

	if p.token != token.Semicolon {
		s.Init = &ir.ValDecl{}
		s.SetPos(p.pos)
		s.Init.Decl = token.Var

		s.Init.Name = p.parseIdent()
		s.Init.Type = p.parseType(true)

		p.expect(token.Assign)
		s.Init.Initializer = p.parseExpr()
		s.SetEndPos(p.pos)
	}

	p.expectSemi()

	if p.token != token.Semicolon {
		s.Cond = p.parseCondition()
	}

	p.expectSemi()

	if p.token != token.Lbrace {
		s.Inc = p.parseExprOrAssignStmt()
	}

	p.loopCount++
	s.Body = p.parseBlockStmt()
	p.loopCount--

	return s
}

func (p *parser) parseReturnStmt() *ir.ReturnStmt {
	s := &ir.ReturnStmt{}
	s.SetPos(p.pos)
	p.next()
	if p.token != token.Semicolon {
		s.X = p.parseExpr()
	}
	return s
}

func (p *parser) parseExprOrAssignStmt() ir.Stmt {
	var stmt ir.Stmt
	pos := p.pos
	expr := p.parseExpr()
	if p.token.IsAssignOp() || p.token.OneOf(token.Inc, token.Dec) {
		assign := p.token
		p.next()
		var right ir.Expr
		if assign.Is(token.Inc) {
			right = &ir.BasicLit{Tok: token.Integer, Value: "1"}
			assign = token.AddAssign
		} else if assign.Is(token.Dec) {
			right = &ir.BasicLit{Tok: token.Integer, Value: "1"}
			assign = token.SubAssign
		} else {
			right = p.parseExpr()
		}
		stmt = &ir.AssignStmt{Left: expr, Assign: assign, Right: right}
	} else {
		stmt = &ir.ExprStmt{X: expr}
	}
	stmt.SetPos(pos)
	return stmt
}

func (p *parser) parseType(optional bool) ir.Expr {
	if p.token.Is(token.Lparen) {
		pos := p.pos
		p.next()
		t := p.parseType(optional)
		if t != nil {
			p.expect(token.Rparen)
			t.SetRange(pos, p.pos)
		}
		return t
	} else if p.token.Is(token.Pointer) {
		return p.parsePointerType()
	} else if p.token.Is(token.Lbrack) {
		return p.parseArrayType()
	} else if p.token.OneOf(token.Func) {
		return p.parseFuncType()
	} else if p.token.Is(token.Ident) {
		return p.parseName()
	} else if optional {
		return nil
	}
	p.error(p.pos, "expected type")
	p.next()
	panic(parseError{p.token})
}

func (p *parser) parsePointerType() ir.Expr {
	pointer := &ir.PointerTypeExpr{}
	pointer.SetPos(p.pos)
	p.expect(token.Pointer)
	pointer.Decl = token.Val
	if p.token.OneOf(token.Var, token.Val) {
		pointer.Decl = p.token
		p.next()
	}
	pointer.X = p.parseType(false)
	pointer.SetEndPos(p.pos)
	return pointer
}

func (p *parser) parseArrayType() ir.Expr {
	array := &ir.ArrayTypeExpr{}
	array.SetPos(p.pos)
	p.expect(token.Lbrack)
	array.X = p.parseType(false)
	if p.token.Is(token.Colon) {
		p.next()
		array.Size = p.parseExpr()
	}
	p.expect(token.Rbrack)
	array.SetEndPos(p.pos)
	return array
}

func (p *parser) parseFuncType() ir.Expr {
	fun := &ir.FuncTypeExpr{}
	fun.SetPos(p.pos)
	p.expect(token.Func)
	if p.token.Is(token.Lbrack) {
		p.next()
		fun.ABI = p.parseIdent()
		p.expect(token.Rbrack)
	}
	fun.Params, fun.Return = p.parseFuncSignature()
	fun.SetPos(p.pos)
	return fun
}

func (p *parser) parseCondition() ir.Expr {
	return p.parseBinaryExpr(true, ir.LowestPrec)
}

func (p *parser) parseExpr() ir.Expr {
	return p.parseBinaryExpr(false, ir.LowestPrec)
}

func (p *parser) parseBinaryExpr(condition bool, prec int) ir.Expr {
	var expr ir.Expr
	pos := p.pos

	if p.token.OneOf(token.Sub, token.Lnot, token.Mul) {
		op := p.token
		p.next()
		expr = p.parseOperand(condition)
		expr = &ir.UnaryExpr{Op: op, X: expr}
		expr.SetRange(pos, p.pos)
	} else if p.token.Is(token.And) {
		p.next()
		decl := token.Val
		if p.token.OneOf(token.Var, token.Val) {
			decl = p.token
			p.next()
		}
		expr = p.parseOperand(condition)
		expr = &ir.AddressExpr{Decl: decl, X: expr}
		expr.SetRange(pos, p.pos)
	} else {
		expr = p.parseOperand(condition)
	}

	expr = p.parseAsExpr(expr)

	for p.token.IsBinaryOp() {
		op := p.token
		opPrec := ir.BinaryPrec(op)
		if prec < opPrec {
			break
		}
		p.next()
		right := p.parseBinaryExpr(condition, opPrec-1)
		bin := &ir.BinaryExpr{Left: expr, Op: op, Right: right}
		bin.SetRange(bin.Left.Pos(), bin.Right.EndPos())
		expr = bin
	}

	return expr
}

func (p *parser) parseAsExpr(expr ir.Expr) ir.Expr {
	if p.token.Is(token.As) {
		cast := &ir.CastExpr{}
		cast.X = expr
		p.next()
		cast.ToType = p.parseType(false)
		cast.SetRange(expr.Pos(), p.pos)
		return cast
	}
	return expr
}

func (p *parser) parseOperand(condition bool) ir.Expr {
	var expr ir.Expr
	if p.token.Is(token.Lparen) {
		pos := p.pos
		p.next()
		expr = p.parseExpr()
		expr.SetPos(pos)
		p.expect(token.Rparen)
		expr.SetEndPos(p.pos)
	} else if p.token.Is(token.Lenof) {
		expr = p.parseLenExpr()
	} else if p.token.Is(token.Sizeof) {
		expr = p.parseSizeExpr()
	} else if p.token.Is(token.Ident) {
		expr = p.parseName()
		if p.token.Is(token.String) {
			expr = p.parseBasicLit(expr)
		} else if p.token.Is(token.Lbrace) && !condition {
			expr = p.parseStructLit(expr)
		}
	} else if p.token.Is(token.Lbrack) {
		expr = p.parseArrayLit()
	} else if p.token.Is(token.Func) {
		expr = p.parseFuncLit()
	} else {
		expr = p.parseBasicLit(nil)
	}
	return p.parsePrimary(expr)
}

func (p *parser) parseLenExpr() *ir.LenExpr {
	lenof := &ir.LenExpr{}
	lenof.SetPos(p.pos)
	p.next()
	p.expect(token.Lparen)
	lenof.X = p.parseExpr()
	p.expect(token.Rparen)
	lenof.SetEndPos(p.pos)
	return lenof
}

func (p *parser) parseSizeExpr() *ir.SizeExpr {
	sizeof := &ir.SizeExpr{}
	sizeof.SetPos(p.pos)
	p.next()
	p.expect(token.Lparen)
	sizeof.X = p.parseType(false)
	p.expect(token.Rparen)
	sizeof.SetEndPos(p.pos)
	return sizeof
}

func (p *parser) parseArgExpr(stop token.Token) *ir.ArgExpr {
	arg := &ir.ArgExpr{}
	arg.SetPos(p.pos)
	expr := p.parseExpr()
	if p.token.Is(token.Colon) {
		if ident, ok := expr.(*ir.Ident); ok {
			p.next()
			arg.Name = ident
			arg.Value = p.parseExpr()
		} else {
			// Trigger an error
			p.expect(token.Comma, stop)
		}
	} else {
		arg.Value = expr
	}
	arg.SetEndPos(p.pos)
	return arg
}

func (p *parser) parseArgumentList(stop token.Token) []*ir.ArgExpr {
	var args []*ir.ArgExpr
	if !p.token.Is(stop) {
		args = append(args, p.parseArgExpr(stop))
		for p.token != token.EOF && p.token != stop {
			p.expect(token.Comma, stop)
			if p.token.Is(stop) {
				break
			}
			args = append(args, p.parseArgExpr(stop))
		}
	}
	return args
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
	colon := token.Invalid
	pos := p.pos

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

	p.expect(token.Rbrack)

	if colon != token.Invalid {
		slice := &ir.SliceExpr{X: expr, Start: index1, End: index2}
		slice.SetRange(pos, p.pos)
		return slice
	}

	res := &ir.IndexExpr{X: expr, Index: index1}
	res.SetRange(pos, p.pos)
	return p.parsePrimary(res)
}

func (p *parser) parseName() ir.Expr {
	var name ir.Expr
	name = p.parseIdent()
	for p.token.Is(token.Dot) {
		name = p.parseDotExpr(name)
	}
	return name
}

func (p *parser) parseIdent() *ir.Ident {
	ident := &ir.Ident{}
	ident.SetPos(p.pos)
	ident.Tok = p.token
	ident.Literal = p.literal
	p.expect(token.Ident)
	ident.SetEndPos(p.pos)
	return ident
}

func (p *parser) parseDotExpr(expr ir.Expr) ir.Expr {
	dot := &ir.DotExpr{}
	dot.SetPos(expr.Pos())
	dot.X = expr
	p.expect(token.Dot)
	dot.Name = p.parseIdent()
	dot.SetEndPos(p.pos)
	return dot
}

func (p *parser) parseFuncCall(expr ir.Expr) ir.Expr {
	call := &ir.FuncCall{}
	call.SetPos(expr.Pos())
	call.X = expr
	p.expect(token.Lparen)
	call.Args = p.parseArgumentList(token.Rparen)
	p.expect(token.Rparen)
	call.SetEndPos(p.pos)
	return call
}

func (p *parser) parseBasicLit(prefix ir.Expr) ir.Expr {
	switch p.token {
	case token.Integer, token.Float, token.Char, token.String, token.True, token.False, token.Null:
		lit := &ir.BasicLit{Prefix: prefix}
		lit.Tok = p.token
		lit.Value = p.literal

		if prefix != nil {
			lit.SetPos(prefix.Pos())
		} else {
			lit.SetPos(p.pos)
		}

		p.next()

		if p.token.OneOf(token.Integer, token.Float) && p.token.Is(token.Ident) {
			lit.Suffix = p.parseName()
		}

		lit.SetEndPos(p.pos)
		return lit
	default:
		p.error(p.pos, "expected expression")
		p.next()
		panic(parseError{p.token})
	}
}

func (p *parser) parseStructLit(name ir.Expr) ir.Expr {
	lit := &ir.StructLit{Name: name}
	lit.SetPos(name.Pos())
	p.expect(token.Lbrace)
	p.blockCount++
	lit.Args = p.parseArgumentList(token.Rbrace)
	p.expect(token.Rbrace)
	p.blockCount--
	lit.SetEndPos(p.pos)
	return lit
}

func (p *parser) parseArrayLit() ir.Expr {
	lit := &ir.ArrayLit{}
	lit.SetPos(p.pos)
	p.expect(token.Lbrack)
	var inits []ir.Expr
	if !p.token.Is(token.Rbrack) {
		inits = append(inits, p.parseExpr())
		for p.token != token.EOF && p.token != token.Rbrack {
			p.expect(token.Comma, token.Rbrack)
			if p.token.Is(token.Rbrack) {
				break
			}
			inits = append(inits, p.parseExpr())
		}
	}
	p.expect(token.Rbrack)
	lit.Initializers = inits
	lit.SetEndPos(p.pos)
	return lit
}

func (p *parser) parseFuncLit() ir.Expr {
	decl := &ir.FuncDecl{}
	decl.SetPos(p.pos)
	decl.Flags = ir.AstFlagAnon

	name := ""
	if len(p.funcName) > 0 {
		name = fmt.Sprintf("$%s_anon%d", p.funcName, p.funcAnonCount)
		p.funcAnonCount++
	} else {
		name = fmt.Sprintf("$anon%d", p.globalAnonCount)
		p.globalAnonCount++
	}

	decl.Name = ir.NewIdent2(token.Ident, name)

	decl.Visibility = token.Private
	p.expect(token.Func)
	if p.token.Is(token.Lbrack) {
		p.next()
		decl.ABI = p.parseIdent()
		p.expect(token.Rbrack)
	}

	decl.Params, decl.Return = p.parseFuncSignature()
	decl.SetEndPos(p.pos)
	decl.Body = p.parseBlockStmt()

	p.decls = append(p.decls, decl)

	return decl.Name
}
