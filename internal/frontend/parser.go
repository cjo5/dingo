package frontend

import (
	"bytes"
	"fmt"
	"io/ioutil"

	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

var anonID = 1

func parseFile(filename string) (*ir.File, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	p := newParser(buf, filename)

	mod := &ir.IncompleteModule{ParentIndex: 0}
	mod.Visibility = token.Private
	mod.Name = ir.NewIdent2(token.Ident, "")
	p.file.Modules = append(p.file.Modules, mod)

	p.parseModuleBody(mod, 0, false)
	p.file.Modules[0].Decls = append(p.file.Modules[0].Decls, p.anonDecls...)

	if p.errors.IsError() {
		return p.file, p.errors
	}

	return p.file, nil
}

type parseError int

type parser struct {
	lexer  lexer
	errors *common.ErrorList
	file   *ir.File

	prev    token.Token
	token   token.Token
	pos     token.Position
	literal string

	blockCount int
	funcName   string
	anonDecls  []*ir.TopDecl
}

func newParser(src []byte, filename string) *parser {
	p := &parser{}
	p.errors = &common.ErrorList{}
	p.file = &ir.File{Filename: filename}
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

func (p *parser) endPos() token.Position {
	pos := p.pos
	n := len(p.literal)
	pos.Column += n
	pos.Offset += n
	return pos
}

func (p *parser) sync() {
	lbrace := p.blockCount
	semi := false
	p.blockCount = 0
	p.next()
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
			panic(parseError(0))
		}

		return false
	}
	p.next()
	return true
}

func (p *parser) expect2(expected token.Token, sync bool) bool {
	return p.expect3(expected, nil, sync)
}

func (p *parser) expect(id token.Token, alts ...token.Token) bool {
	return p.expect3(id, alts, true)
}

func (p *parser) expectSemi1(sync bool) bool {
	if !p.token.OneOf(token.Rbrace, token.Rbrack) {
		if !p.token.OneOf(token.Semicolon, token.EOF) {
			p.error(p.pos, "expected semicolon or newline")
			if sync {
				panic(parseError(0))
			}
			return false
		}
		p.next()
	}
	return true
}

func (p *parser) expectSemi() bool {
	return p.expectSemi1(true)
}

func (p *parser) isSemi() bool {
	return p.token.OneOf(token.Semicolon, token.EOF)
}

func (p *parser) parseModuleBody(mod *ir.IncompleteModule, modIndex int, block bool) bool {
	ok := true
	for !(p.token.Is(token.EOF) || (p.token.Is(token.Rbrace) && block)) {
		sync := true
		if p.token.Is(token.Include) {
			include := p.parseInclude()
			if include != nil {
				mod.Includes = append(mod.Includes, include)
				sync = false
			}
		} else if p.isSemi() {
			p.next()
			sync = false
		} else {
			visibility := token.Private
			if p.token.OneOf(token.Public, token.Private) {
				visibility = p.token
				p.next()
			}
			if p.token.Is(token.Module) {
				if p.parseModule(modIndex, visibility) {
					sync = false
				}
			} else {
				decl := p.parseTopDecl(visibility)
				sync = false
				if decl != nil {
					mod.Decls = append(mod.Decls, decl)
				} else {
					ok = false
				}
			}
		}
		if sync {
			p.sync()
			ok = false
		}
	}
	return ok
}

func (p *parser) parseModule(parentIndex int, visibility token.Token) bool {
	p.next()
	var names []*ir.Ident
	for p.token.Is(token.Ident) {
		names = append(names, p.parseIdent())
		if p.token.Is(token.Dot) {
			p.next()
			names = append(names, p.parseIdent())
		}
	}

	if len(names) == 0 {
		p.expect2(token.Ident, false)
		return false
	}

	if !p.expect2(token.Lbrace, false) {
		return false
	}

	for i := 0; i < len(names)-1; i++ {
		mod := &ir.IncompleteModule{ParentIndex: parentIndex, Name: names[i]}
		mod.Visibility = token.Private
		p.file.Modules = append(p.file.Modules, mod)
		parentIndex = len(p.file.Modules) - 1
	}

	mod := &ir.IncompleteModule{ParentIndex: parentIndex, Name: names[len(names)-1]}
	mod.Visibility = visibility
	p.file.Modules = append(p.file.Modules, mod)
	modIndex := len(p.file.Modules) - 1

	ok := p.parseModuleBody(mod, modIndex, true)

	if p.token.Is(token.Rbrace) || !ok {
		if !p.expect2(token.Rbrace, false) {
			ok = false
		}
	}

	return ok
}

func (p *parser) parseInclude() *ir.BasicLit {
	p.next()
	if !p.token.Is(token.String) {
		p.expect2(token.String, false)
		return nil
	}
	include := &ir.BasicLit{Tok: p.token, Value: p.literal}
	include.SetRange(p.pos, p.endPos())
	p.next()
	return include
}

func (p *parser) parseTopDecl(visibility token.Token) (topDecl *ir.TopDecl) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(parseError); ok {
				p.sync()
				topDecl = nil
			} else {
				panic(r)
			}
		}
	}()

	var abi *ir.Ident
	var decl ir.Decl

	if p.token.Is(token.Extern) {
		abi = p.parseExtern()
		if p.token.OneOf(token.Var, token.Val) {
			decl = p.parseValDecl()
			p.expectSemi()
		} else if p.token.Is(token.Func) {
			decl = p.parseFuncDecl()
			p.expectSemi()
		} else {
			p.error(p.pos, "expected '%s', '%s' or '%s'", token.Var, token.Val, token.Func)
			panic(parseError(0))
		}
	} else if p.token.Is(token.Func) {
		decl = p.parseFuncDecl()
		p.expectSemi()
	} else if p.token.Is(token.Struct) {
		decl = p.parseStructDecl()
		p.expectSemi()
	} else {
		decl = p.parseDecl(true)
		p.expectSemi()
	}

	if decl != nil {
		return ir.NewTopDecl(abi, visibility, decl)
	}

	return nil
}

func (p *parser) parseExtern() *ir.Ident {
	var abi *ir.Ident
	if p.token.Is(token.Extern) {
		pos := p.pos
		p.next()
		if p.token.Is(token.Lparen) {
			p.next()
			abi = p.parseIdent()
			p.expect(token.Rparen)
		} else {
			abi = ir.NewIdent2(token.Ident, ir.CABI)
			abi.SetRange(pos, pos)
		}
	}
	return abi
}

func (p *parser) parseStructDecl() *ir.StructDecl {
	decl := &ir.StructDecl{}
	decl.SetPos(p.pos)
	p.next()
	decl.Name = p.parseIdent()
	decl.SetEndPos(decl.Name.EndPos())
	if p.isSemi() {
		decl.Opaque = true
	} else {
		decl.Opaque = false
		p.expect(token.Lbrace)
		p.blockCount++
		flags := ir.AstFlagNoInit | ir.AstFlagPublic
		for !p.token.OneOf(token.EOF, token.Rbrace) {
			decl.Fields = append(decl.Fields, p.parseField(flags, token.Var))
			p.expectSemi()
		}
		p.expect(token.Rbrace)
		p.blockCount--
	}
	return decl
}

func (p *parser) parseFuncDecl() *ir.FuncDecl {
	decl := &ir.FuncDecl{}
	decl.SetPos(p.pos)
	p.next()
	decl.Name = p.parseIdent()
	decl.Params, decl.Return = p.parseFuncSignature()
	decl.SetEndPos(decl.Return.EndPos())
	if p.isSemi() {
		return decl
	}
	decl.Body = p.parseBlock(false)
	return decl
}

func (p *parser) parseDecl(topDecl bool) ir.Decl {
	var decl ir.Decl
	if p.token.Is(token.Import) {
		decl = p.parseImportDecl(topDecl)
	} else if p.token.Is(token.AliasType) {
		decl = p.parseTypeDecl()
	} else if p.token.OneOf(token.Var, token.Val) {
		decl = p.parseValDecl()
	} else {
		p.error(p.pos, "expected declaration")
		panic(parseError(0))
	}
	return decl
}

func (p *parser) parseImportDecl(topDecl bool) *ir.ImportDecl {
	decl := &ir.ImportDecl{}
	decl.Decl = p.token
	decl.SetPos(p.pos)
	p.next()
	decl.Alias, decl.Name = p.parseImportName()
	decl.SetEndPos(decl.Name.EndPos())
	if p.token.Is(token.Colon) {
		p.next()
		paren := false
		if p.token.Is(token.Lparen) {
			p.next()
			paren = true
		}
		decl.Items = append(decl.Items, p.parseImportItem(topDecl))
		for p.token.Is(token.Comma) {
			p.next()
			if p.token.OneOf(token.EOF, token.Semicolon) || (p.token.Is(token.Rparen) && paren) {
				break
			}
			decl.Items = append(decl.Items, p.parseImportItem(topDecl))
		}
		if paren {
			p.expect(token.Rparen)
		}
	}
	return decl
}

func (p *parser) parseImportItem(topDecl bool) *ir.ImportItem {
	item := &ir.ImportItem{}
	item.Visibilty = token.Private
	if topDecl && p.token.OneOf(token.Private, token.Public) {
		item.Visibilty = p.token
		p.next()
	}
	item.Name = p.parseIdent()
	if p.token.Is(token.Assign) {
		item.Alias = item.Name
		p.next()
		item.Name = p.parseIdent()
	}
	return item
}

func (p *parser) parseImportName() (alias *ir.Ident, name ir.Expr) {
	if p.token.OneOf(token.RootMod, token.ParentMod, token.SelfMod) {
		name = p.parseScopeName()
		return
	}
	var ident *ir.Ident
	if p.token.Is(token.Underscore) {
		alias = ir.NewIdent2(p.token, p.literal)
		alias.SetRange(p.pos, p.endPos())
		p.next()
	} else {
		ident = p.parseIdent()
	}
	if p.token.Is(token.Assign) {
		if alias == nil {
			alias = ident
		}
		p.next()
		name = p.parseScopeName()
	} else if alias != nil {
		p.expect(token.Assign)
		return nil, nil
	} else {
		name = ident
		for p.token.Is(token.Dot) {
			name = p.parseDotExpr(name)
		}
	}
	return
}

func (p *parser) parseScopeName() ir.Expr {
	var scopePart ir.Expr
	var namePart *ir.Ident
	if p.token.Is(token.RootMod) {
		scopePart = ir.NewIdent2(p.token, ir.RootModuleName)
		scopePart.SetRange(p.pos, p.endPos())
		p.next()
		namePart = p.parseIdent()
	} else if p.token.Is(token.ParentMod) {
		scopePart = ir.NewIdent2(p.token, ir.ParentModuleName)
		scopePart.SetRange(p.pos, p.endPos())
		p.next()
		for p.token.Is(token.Dot) && namePart == nil {
			p.next()
			if p.token == token.ParentMod {
				name := ir.NewIdent2(p.token, ir.ParentModuleName)
				name.SetRange(p.pos, p.endPos())
				p.next()
				dot := &ir.DotExpr{X: scopePart, Name: name}
				dot.SetRange(scopePart.Pos(), name.EndPos())
				scopePart = dot
			} else {
				namePart = p.parseIdent()
			}
		}
	} else if p.token.Is(token.SelfMod) {
		scopePart = ir.NewIdent2(p.token, ir.SelfModuleName)
		scopePart.SetRange(p.pos, p.endPos())
		p.next()
	} else {
		namePart = p.parseIdent()
	}
	var expr ir.Expr
	if namePart != nil {
		if scopePart != nil {
			expr = &ir.DotExpr{X: scopePart, Name: namePart}
			expr.SetRange(scopePart.Pos(), namePart.EndPos())
		} else {
			expr = namePart
		}
	} else {
		expr = scopePart
	}
	for p.token.Is(token.Dot) {
		expr = p.parseDotExpr(expr)
	}
	return expr
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
	ident.SetRange(p.pos, p.endPos())
	ident.Tok = p.token
	ident.Literal = p.literal
	p.expect(token.Ident)
	return ident
}

func (p *parser) parseTypeDecl() *ir.TypeDecl {
	decl := &ir.TypeDecl{}
	decl.Decl = p.token
	decl.SetPos(p.pos)
	p.next()
	decl.Name = p.parseIdent()
	p.expect(token.Assign)
	decl.Type = p.parseType(true)
	decl.SetEndPos(decl.Type.EndPos())
	return decl
}

func (p *parser) parseValDecl() *ir.ValDecl {
	decl := &ir.ValDecl{}
	decl.Decl = p.token
	decl.SetPos(p.pos)
	p.next()
	decl.Name = p.parseIdent()
	decl.Type = p.parseType(true)
	if p.token.Is(token.Assign) {
		p.next()
		decl.Initializer = p.parseExpr()
		decl.SetEndPos(decl.Initializer.EndPos())
	} else if decl.Type != nil {
		decl.SetEndPos(decl.Type.EndPos())
	} else {
		p.error(p.pos, "expected type or assignment")
		panic(parseError(0))
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

	tok, lit := p.token, p.literal
	startPos, endPos := p.pos, p.endPos()

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
			decl.SetRange(startPos, endPos)
			tok, lit = token.Underscore, token.Underscore.String()
			startPos, endPos = token.NoPosition, token.NoPosition
		}
	}

	decl.Name = ir.NewIdent2(tok, lit)
	decl.Name.SetRange(startPos, endPos)

	decl.SetEndPos(decl.Type.EndPos())

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
	endPos := p.pos
	p.expect(token.Rparen)
	ret = &ir.ValDecl{}
	ret.SetPos(p.pos)
	ret.Decl = token.Val
	ret.Name = ir.NewIdent2(token.Underscore, token.Underscore.String())
	ret.Type = p.parseType(true)
	if ret.Type == nil {
		ret.Type = ir.NewIdent2(token.Ident, ir.TVoid.String())
		ret.SetRange(endPos, endPos)
	}
	return
}

func (p *parser) parseStmt() (stmt ir.Stmt, sync bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(parseError); ok {
				p.sync()
				sync = true
			} else {
				panic(r)
			}
		}
	}()

	sync = false

	if p.isSemi() {
		stmt = nil
	} else if p.token.Is(token.Lbrace) {
		stmt = p.parseBlockStmt()
	} else if p.token.OneOf(token.Import, token.AliasType, token.Var, token.Val) {
		d := p.parseDecl(false)
		stmt = &ir.DeclStmt{D: d}
		stmt.SetRange(d.Pos(), d.EndPos())
	} else if p.token.Is(token.If) {
		stmt = p.parseIfStmt()
	} else if p.token.Is(token.While) {
		stmt = p.parseWhileStmt()
	} else if p.token.Is(token.For) {
		stmt = p.parseForStmt()
	} else if p.token.Is(token.Return) {
		stmt = p.parseReturnStmt()
	} else if p.token.Is(token.Defer) {
		stmt = p.parseDeferStmt()
	} else if p.token.OneOf(token.Break, token.Continue) {
		stmt = &ir.BranchStmt{Tok: p.token}
		stmt.SetPos(p.pos)
		p.next()
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
	block.SetRange(p.pos, p.pos)

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

	block.SetEndPos(p.pos)

	if p.token.Is(token.Rbrace) || !didSync {
		p.expect(token.Rbrace)
	}

	if incBody && !didSync {
		p.blockCount--
	}

	return block
}

func (p *parser) parseIfStmt() *ir.IfStmt {
	s := &ir.IfStmt{}
	s.Tok = p.token
	s.SetPos(p.pos)
	p.next()
	s.Cond = p.parseCondition()
	s.Body = p.parseBlockStmt()
	if p.token == token.Elif {
		s.Else = p.parseIfStmt()
	} else if p.token == token.Else {
		p.next()
		s.Else = p.parseBlockStmt()
	}
	if s.Else != nil {
		s.SetEndPos(s.Else.EndPos())
	} else {
		s.SetEndPos(s.Body.EndPos())
	}
	return s
}

func (p *parser) parseWhileStmt() *ir.ForStmt {
	s := &ir.ForStmt{}
	s.Tok = p.token
	s.SetPos(p.pos)
	p.next()
	s.Cond = p.parseCondition()
	s.Body = p.parseBlockStmt()
	s.SetEndPos(s.Body.EndPos())
	return s
}

func (p *parser) parseForStmt() *ir.ForStmt {
	s := &ir.ForStmt{}
	s.Tok = p.token
	s.SetPos(p.pos)
	p.next()
	if p.token != token.Semicolon {
		decl := &ir.ValDecl{}
		s.SetPos(p.pos)
		decl.Decl = token.Var
		decl.Name = p.parseIdent()
		decl.Type = p.parseType(true)
		p.expect(token.Assign)
		decl.Initializer = p.parseExpr()
		s.Init = &ir.DeclStmt{D: decl}
		s.Init.SetPos(decl.Pos())
	}
	p.expectSemi()
	if p.token != token.Semicolon {
		s.Cond = p.parseCondition()
	}
	p.expectSemi()
	if p.token != token.Lbrace {
		s.Inc = p.parseExprOrAssignStmt()
	}
	s.Body = p.parseBlockStmt()
	s.SetEndPos(s.Body.EndPos())
	return s
}

func (p *parser) parseReturnStmt() *ir.ReturnStmt {
	s := &ir.ReturnStmt{}
	s.SetRange(p.pos, p.pos)
	p.next()
	if p.token != token.Semicolon {
		s.X = p.parseExpr()
	}
	return s
}

func (p *parser) parseDeferStmt() *ir.DeferStmt {
	s := &ir.DeferStmt{}
	s.SetRange(p.pos, p.pos)
	p.next()
	s.S = p.parseExprOrAssignStmt()
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
			right = &ir.BasicLit{Tok: token.Integer, Value: "1"}
			assign = token.AddAssign
		} else if assign.Is(token.Dec) {
			right = &ir.BasicLit{Tok: token.Integer, Value: "1"}
			assign = token.SubAssign
		} else {
			right = p.parseExpr()
		}
		stmt = &ir.AssignStmt{Left: expr, Assign: assign, Right: right}
		stmt.SetRange(expr.Pos(), right.EndPos())
	} else {
		stmt = &ir.ExprStmt{X: expr}
		stmt.SetRange(expr.Pos(), expr.EndPos())
	}
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
	} else if p.token.OneOf(token.And, token.Land) {
		return p.parsePointerType()
	} else if p.token.Is(token.Lbrack) {
		return p.parseArrayType()
	} else if p.token.OneOf(token.Extern, token.Func) {
		return p.parseFuncType()
	} else if p.token.Is(token.Ident) {
		return p.parseScopeName()
	} else if optional {
		return nil
	}
	p.error(p.pos, "expected type")
	panic(parseError(0))
}

func (p *parser) parsePointerType() ir.Expr {
	pointer := &ir.PointerTypeExpr{}
	tok, pos := p.token, p.pos
	p.next()
	pointer.Decl = token.Val
	if p.token.OneOf(token.Var, token.Val) {
		pointer.Decl = p.token
		p.next()
	}
	pointer.X = p.parseType(false)
	pointer.SetRange(pos, p.pos)
	if tok.Is(token.Land) {
		// && is a single token so double pointer needs to be handled separately
		inner := &ir.PointerTypeExpr{}
		inner.Decl = pointer.Decl
		inner.X = pointer.X
		pos.Offset++
		pos.Column++
		inner.SetRange(pos, p.pos)
		pointer.Decl = token.Val
		pointer.X = inner
	}
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
	array.SetEndPos(p.endPos())
	p.expect(token.Rbrack)
	return array
}

func (p *parser) parseFuncType() ir.Expr {
	fun := &ir.FuncTypeExpr{}
	fun.ABI = p.parseExtern()
	fun.SetPos(p.pos)
	p.expect(token.Func)
	if p.token.Is(token.Lbrack) {
		p.next()
		fun.ABI = p.parseIdent()
		p.expect(token.Rbrack)
	}
	fun.Params, fun.Return = p.parseFuncSignature()
	fun.SetPos(fun.Return.EndPos())
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

	if p.token.OneOf(token.Sub, token.Lnot, token.Deref, token.Addr) {
		op := p.token
		p.next()
		decl := token.Invalid
		if op.Is(token.Addr) {
			decl = token.Val
			if p.token.OneOf(token.Var, token.Val) {
				decl = p.token
				p.next()
			}
		}
		expr = p.parseOperand(condition)
		endPos := expr.EndPos()
		expr = &ir.UnaryExpr{Op: op, Decl: decl, X: expr}
		expr.SetRange(pos, endPos)
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
		cast.SetRange(expr.Pos(), cast.ToType.EndPos())
		return cast
	}
	return expr
}

func (p *parser) tryParseStructLit(expr ir.Expr, condition bool) ir.Expr {
	if p.token.Is(token.Lbrace) && !condition {
		expr = p.parseStructLit(expr)
	}
	return expr
}

func (p *parser) parseOperand(condition bool) ir.Expr {
	var expr ir.Expr
	if p.token.Is(token.Lparen) {
		pos := p.pos
		p.next()
		expr = p.parseExpr()
		expr.SetRange(pos, p.endPos())
		p.expect(token.Rparen)
	} else if p.token.Is(token.Lenof) {
		expr = p.parseLenExpr()
	} else if p.token.Is(token.Sizeof) {
		expr = p.parseSizeExpr()
	} else if p.token.Is(token.Ident) {
		expr = p.parseName()
		if p.token.Is(token.String) {
			expr = p.parseBasicLit(expr)
		} else {
			expr = p.tryParseStructLit(expr, condition)
		}
	} else if p.token.OneOf(token.RootMod, token.ParentMod, token.SelfMod) {
		expr = p.parseScopeName()
		expr = p.tryParseStructLit(expr, condition)
	} else if p.token.Is(token.Lbrack) {
		expr = p.parseArrayLit()
	} else if p.token.OneOf(token.Func, token.Extern) {
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
	lenof.SetEndPos(p.endPos())
	p.expect(token.Rparen)
	return lenof
}

func (p *parser) parseSizeExpr() *ir.SizeExpr {
	sizeof := &ir.SizeExpr{}
	sizeof.SetPos(p.pos)
	p.next()
	p.expect(token.Lparen)
	sizeof.X = p.parseType(false)
	sizeof.SetEndPos(p.endPos())
	p.expect(token.Rparen)
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
	arg.SetEndPos(arg.Value.EndPos())
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

	endPos := p.endPos()
	p.expect(token.Rbrack)

	if colon != token.Invalid {
		slice := &ir.SliceExpr{X: expr, Start: index1, End: index2}
		slice.SetRange(pos, endPos)
		return slice
	}

	res := &ir.IndexExpr{X: expr, Index: index1}
	res.SetRange(pos, endPos)
	return p.parsePrimary(res)
}

func (p *parser) parseDotExpr(expr ir.Expr) *ir.DotExpr {
	dot := &ir.DotExpr{}
	dot.SetPos(expr.Pos())
	dot.X = expr
	p.expect(token.Dot)
	dot.Name = p.parseIdent()
	dot.SetEndPos(dot.Name.EndPos())
	return dot
}

func (p *parser) parseFuncCall(expr ir.Expr) ir.Expr {
	call := &ir.FuncCall{}
	call.SetPos(expr.Pos())
	call.X = expr
	p.expect(token.Lparen)
	call.Args = p.parseArgumentList(token.Rparen)
	call.SetEndPos(p.endPos())
	p.expect(token.Rparen)
	return call
}

func (p *parser) parseBasicLit(prefix ir.Expr) ir.Expr {
	switch p.token {
	case token.Integer, token.Float, token.Char, token.String, token.True, token.False, token.Null:
		lit := &ir.BasicLit{Prefix: prefix}
		lit.Tok = p.token
		lit.Value = p.literal

		if prefix != nil {
			lit.SetRange(prefix.Pos(), p.endPos())
		} else {
			lit.SetRange(p.pos, p.endPos())
		}

		p.next()

		if lit.Tok.OneOf(token.Integer, token.Float) && p.token.Is(token.Ident) {
			lit.Suffix = p.parseName()
			lit.SetEndPos(lit.Suffix.EndPos())
		}

		return lit
	default:
		p.error(p.pos, "expected expression")
		panic(parseError(0))
	}
}

func (p *parser) parseStructLit(name ir.Expr) ir.Expr {
	lit := &ir.StructLit{Name: name}
	lit.SetRange(name.Pos(), name.EndPos())
	p.expect(token.Lbrace)
	p.blockCount++
	lit.Args = p.parseArgumentList(token.Rbrace)
	p.expect(token.Rbrace)
	p.blockCount--
	return lit
}

func (p *parser) parseArrayLit() ir.Expr {
	lit := &ir.ArrayLit{}
	lit.SetPos(p.pos)
	p.expect(token.Lbrack)
	lit.Elem = p.parseType(false)
	if p.token.Is(token.Colon) {
		p.next()
		lit.Size = p.parseExpr()
	}
	p.expect(token.Rbrack)
	p.expect(token.Lbrace)
	var inits []ir.Expr
	if !p.token.Is(token.Rbrace) {
		inits = append(inits, p.parseExpr())
		for p.token != token.EOF && p.token != token.Rbrace {
			p.expect(token.Comma, token.Rbrace)
			if p.token.Is(token.Rbrack) {
				break
			}
			inits = append(inits, p.parseExpr())
		}
	}
	lit.SetEndPos(p.endPos())
	p.expect(token.Rbrace)
	lit.Initializers = inits
	return lit
}

func (p *parser) parseFuncLit() ir.Expr {
	decl := &ir.FuncDecl{}
	decl.Flags = ir.AstFlagAnon

	abi := p.parseExtern()
	name := fmt.Sprintf("$anon%d_lineno_%d", anonID, p.pos.Line)
	decl.Name = ir.NewIdent2(token.Ident, name)
	anonID++

	decl.SetPos(p.pos)
	decl.Name.SetRange(p.pos, p.pos)
	p.expect(token.Func)

	decl.Params, decl.Return = p.parseFuncSignature()
	decl.SetEndPos(decl.Return.EndPos())
	decl.Body = p.parseBlockStmt()

	p.anonDecls = append(p.anonDecls, ir.NewTopDecl(abi, token.Private, decl))

	return decl.Name
}
