package semantics

import (
	"github.com/cjo5/dingo/internal/ir"
	"github.com/cjo5/dingo/internal/token"
)

const builtinSymFlags = ir.SymFlagBuiltin | ir.SymFlagDefined

func (c *checker) insertBuiltinType(t ir.Type) {
	c.insertBuiltinType2(t.Kind().String(), t)
}

func (c *checker) insertBuiltinType2(name string, t ir.Type) {
	key := c.nextSymKey()
	sym := ir.NewSymbol(ir.TypeSymbol, key, c.builtinScope.CUID, "", name, token.NoPosition)
	sym.Public = true
	sym.Flags |= builtinSymFlags
	sym.T = t
	c.builtinScope.Insert(name, sym)
}

func (c *checker) insertBuiltinAliasType(name string, t ir.Type) {
	alias := ir.NewAliasType(name, t)
	c.insertBuiltinType2(name, alias)
}

func (c *checker) initBuiltinScope() {
	c.builtinScope = ir.NewScope("builtin_types", nil, -1)

	c.insertBuiltinType(ir.TBuiltinVoid)
	c.insertBuiltinType(ir.TBuiltinBool)
	c.insertBuiltinType(ir.TBuiltinInt8)
	c.insertBuiltinType(ir.TBuiltinUInt8)
	c.insertBuiltinType(ir.TBuiltinInt16)
	c.insertBuiltinType(ir.TBuiltinUInt16)
	c.insertBuiltinType(ir.TBuiltinInt32)
	c.insertBuiltinType(ir.TBuiltinUInt32)
	c.insertBuiltinType(ir.TBuiltinInt64)
	c.insertBuiltinType(ir.TBuiltinUInt64)
	c.insertBuiltinType(ir.TBuiltinUSize)
	c.insertBuiltinType(ir.TBuiltinFloat32)
	c.insertBuiltinType(ir.TBuiltinFloat64)

	c.insertBuiltinType2("Byte", ir.TBuiltinByte)
	c.insertBuiltinType2("Int", ir.TBuiltinInt)
	c.insertBuiltinType2("UInt", ir.TBuiltinUInt)
	c.insertBuiltinType2("Float", ir.TBuiltinFloat)

	// TODO: Change to distinct types
	c.insertBuiltinAliasType("C_void", ir.TBuiltinVoid)
	c.insertBuiltinAliasType("C_char", ir.TBuiltinInt8)
	c.insertBuiltinAliasType("C_uchar", ir.TBuiltinUInt8)
	c.insertBuiltinAliasType("C_short", ir.TBuiltinInt16)
	c.insertBuiltinAliasType("C_ushort", ir.TBuiltinUInt16)
	c.insertBuiltinAliasType("C_int", ir.TBuiltinInt32)
	c.insertBuiltinAliasType("C_uint", ir.TBuiltinUInt32)
	c.insertBuiltinAliasType("C_longlong", ir.TBuiltinInt64)
	c.insertBuiltinAliasType("C_ulonglong", ir.TBuiltinUInt64)
	c.insertBuiltinAliasType("C_usize", ir.TBuiltinUSize)
	c.insertBuiltinAliasType("C_float", ir.TBuiltinFloat32)
	c.insertBuiltinAliasType("C_double", ir.TBuiltinFloat64)
}

func (c *checker) insertSymbol(scope *ir.Scope, alias string, sym *ir.Symbol) *ir.Symbol {
	if alias == token.Placeholder.String() {
		return sym
	}
	if existing := scope.Insert(alias, sym); existing != nil {
		if existing.IsBuiltin() {
			c.error(sym.Pos, "redefinition of builtin '%s'", alias)
			return nil
		}
		if sym.Kind != existing.Kind || (sym.IsDefined() && existing.IsDefined()) {
			c.error(sym.Pos, "redefinition of '%s' (different definition is at %s)", sym.Name, existing.Pos)
			return nil
		}
		if sym.CUID != existing.CUID || sym.FQN() != existing.FQN() ||
			sym.Public != existing.Public || sym.ABI != existing.ABI {
			c.error(sym.Pos, "redeclaration of '%s' (different declaration is at %s)", sym.Name, existing.Pos)
			return nil
		}
		if sym.IsDefined() {
			// Existing symbol is a declaration consistent with the definition
			existing.Key = sym.Key
			existing.Flags |= ir.SymFlagDefined
			existing.Pos = sym.Pos
		}
		return existing
	}
	return sym
}

func (c *checker) insertBuiltinModuleSymbols(CUID int, modFQN string) {
	key := c.nextSymKey()
	sym := ir.NewSymbol(ir.ValSymbol, key, CUID, modFQN, "__fqn__", token.NoPosition)
	sym.Public = true
	sym.Flags = builtinSymFlags | ir.SymFlagConst
	sym.T = ir.NewSliceType(ir.TBuiltinByte, true, true)
	lit := ir.NewStringLit(modFQN)
	lit.T = sym.T
	c.scope.Insert(sym.Name, sym)
	c.constMap[sym.Key] = lit
}

func (c *checker) newTopDeclSymbol(kind ir.SymbolKind, CUID int, modFQN string, abi string, public bool, name string, pos token.Position, definition bool) *ir.Symbol {
	flags := ir.SymFlagTopDecl
	if definition {
		flags |= ir.SymFlagDefined
	}
	key := c.nextSymKey()
	sym := ir.NewSymbol(kind, key, CUID, modFQN, name, pos)
	sym.Public = public
	sym.ABI = abi
	sym.Flags = flags
	return sym
}

func (c *checker) insertImportSymbol(decl *ir.ImportDecl, CUID int, modFQN string, public bool) {
	timportedMod := ir.TBuiltinInvalid
	fqn := decl.Name.FQN(modFQN)
	if mod, ok := c.importMap[fqn]; ok {
		timportedMod = mod.T
	} else {
		c.error(decl.Name.Pos(), "unknown public module '%s'", fqn)
	}
	if isUntyped(timportedMod) {
		return
	}
	sym := c.newTopDeclSymbol(ir.ModuleSymbol, CUID, fqn, ir.DGABI, public, decl.Alias.Literal, decl.Alias.Pos(), false)
	sym.T = timportedMod
	sym.Flags |= ir.SymFlagReadOnly
	decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
}

func (c *checker) newUseSymbol(decl *ir.UseDecl, CUID int, modFQN string, public bool, topDecl bool) *ir.Symbol {
	alias := ir.FQN(modFQN, decl.Alias.Literal)
	name := decl.Name.FQN(modFQN)
	if name == alias {
		c.nodeError(decl, "use cannot refer to itself")
		return nil
	}

	key := c.nextSymKey()
	sym := ir.NewSymbol(ir.UnknownSymbol, key, CUID, modFQN, decl.Alias.Literal, decl.Alias.Pos())
	sym.Public = public
	sym.ABI = ir.DGABI
	if topDecl {
		sym.Flags |= ir.SymFlagTopDecl
	}
	return sym
}

func (c *checker) insertUseSymbol(decl *ir.UseDecl, CUID int, modFQN string, public bool, topDecl bool) {
	decl.Sym = c.newUseSymbol(decl, CUID, modFQN, public, topDecl)
	if decl.Sym != nil {
		decl.Sym = c.insertSymbol(c.scope, decl.Sym.Name, decl.Sym)
	}
}

func (c *checker) newLocalTypeDeclSymbol(decl *ir.TypeDecl, CUID int, modFQN string) *ir.Symbol {
	key := c.nextSymKey()
	sym := ir.NewSymbol(ir.TypeSymbol, key, CUID, modFQN, decl.Name.Literal, decl.Name.Pos())
	sym.Flags = ir.SymFlagDefined
	return sym
}

func (c *checker) newLocalValDeclSymbol(decl *ir.ValDecl, CUID int, modFQN string) *ir.Symbol {
	key := c.nextSymKey()
	sym := ir.NewSymbol(ir.ValSymbol, key, CUID, modFQN, decl.Name.Literal, decl.Name.Pos())
	sym.Flags = ir.SymFlagDefined
	if (decl.Flags & ir.AstFlagField) != 0 {
		sym.Flags |= ir.SymFlagField
	}
	sym.Public = (decl.Flags & ir.AstFlagPublic) != 0
	return sym
}

func (c *checker) insertLocalValDeclSymbol(decl *ir.ValDecl, CUID int, modFQN string) {
	decl.Sym = c.newLocalValDeclSymbol(decl, CUID, modFQN)
	if decl.Sym != nil {
		decl.Sym = c.insertSymbol(c.scope, decl.Sym.Name, decl.Sym)
	}
}

func (c *checker) insertStructDeclBody(decl *ir.StructDecl, methodScope *ir.Scope) {
	sym := decl.Sym
	defer c.setScope(c.setScope(decl.Scope))
	var fields []ir.Field
	for _, field := range decl.Fields {
		c.insertLocalValDeclSymbol(field, sym.CUID, sym.ModFQN)
		if field.Sym != nil {
			fields = append(fields, ir.Field{Name: field.Name.Literal, T: ir.TBuiltinUnknown})
		}
	}
	for _, method := range decl.Methods {
		c.patchSelf(method, sym)
		pubField := (method.Flags & ir.AstFlagPublic) != 0
		name := "dg." + sym.Name + "." + method.Name.Literal
		sym := c.newTopDeclSymbol(ir.FuncSymbol, sym.CUID, sym.ModFQN, sym.ABI, pubField, name, method.Name.Pos(), !method.SignatureOnly())
		sym = c.insertSymbol(c.scope, method.Name.Literal, sym)
		method.Sym = sym
		method.Name.Sym = sym
		if sym != nil {
			sym.Flags |= ir.SymFlagMethod
			method.Scope = ir.NewScope("method", methodScope, sym.CUID)
			if method.Body != nil {
				method.Body.Scope = decl.Scope
			}
			if method.SignatureOnly() {
				c.error(method.Pos(), "method must have a body")
			}
		}
	}
	tstruct := ir.NewStructType(decl.Sym, decl.Scope)
	tstruct.SetBody(fields, false) // Set untyped fields
	decl.Sym.T = tstruct
	decl.Name.Sym = decl.Sym
}

func (c *checker) patchSelf(decl *ir.FuncDecl, structSym *ir.Symbol) {
	if len(decl.Params) == 0 {
		return
	}
	param := decl.Params[0]
	if param.Name.Tok != token.Placeholder {
		return
	}
	typeExpr := param.Type
	if ty2, ok := param.Type.(*ir.PointerTypeExpr); ok {
		typeExpr = ty2.X
	}
	modFQN := structSym.ModFQN
	tyName := ""
	switch typeExpr := typeExpr.(type) {
	case *ir.Ident:
		tyName = ir.FQN(modFQN, typeExpr.Literal)
	case *ir.ScopeLookup:
		tyName = typeExpr.FQN(structSym.ModFQN)
	}
	if len(tyName) > 0 {
		selfTyName := ir.FQN(modFQN, ir.SelfType)
		if tyName == selfTyName {
			param.Name.Tok = token.Ident
			param.Name.Literal = ir.Self
			if !param.Name.Pos().IsValid() {
				param.Name.SetRange(param.Type.Pos(), param.Type.EndPos())
			}
		}
	}
}
