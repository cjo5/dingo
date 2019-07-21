package semantics

import (
	"github.com/cjo5/dingo/internal/ir"
	"github.com/cjo5/dingo/internal/token"
)

const builtinSymFlags = ir.SymFlagBuiltin | ir.SymFlagDefined

func (c *checker) insertBuiltinType(t ir.Type) {
	c.insertBuiltinAliasType(t.Kind().String(), t)
}

func (c *checker) insertBuiltinAliasType(name string, t ir.Type) {
	key := c.nextSymKey()
	sym := ir.NewSymbol(ir.TypeSymbol, key, c.builtinScope.CUID, "", name, token.NoPosition)
	sym.Public = true
	sym.Flags |= builtinSymFlags
	sym.T = t
	c.builtinScope.Insert(name, sym)
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

	// TODO: Change to distinct types
	c.insertBuiltinAliasType("c_void", ir.TBuiltinVoid)
	c.insertBuiltinAliasType("c_char", ir.TBuiltinInt8)
	c.insertBuiltinAliasType("c_uchar", ir.TBuiltinUInt8)
	c.insertBuiltinAliasType("c_short", ir.TBuiltinInt16)
	c.insertBuiltinAliasType("c_ushort", ir.TBuiltinUInt16)
	c.insertBuiltinAliasType("c_int", ir.TBuiltinInt32)
	c.insertBuiltinAliasType("c_uint", ir.TBuiltinUInt32)
	c.insertBuiltinAliasType("c_longlong", ir.TBuiltinInt64)
	c.insertBuiltinAliasType("c_ulonglong", ir.TBuiltinUInt64)
	c.insertBuiltinAliasType("c_usize", ir.TBuiltinUSize)
	c.insertBuiltinAliasType("c_float", ir.TBuiltinFloat32)
	c.insertBuiltinAliasType("c_double", ir.TBuiltinFloat64)
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
	sym.T = ir.NewSliceType(ir.TBuiltinInt8, true, true)
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

func (c *checker) insertImportSymbol(decl *ir.ImportDecl, modFQN string, public bool) {
	importedCUID := -1
	timportedMod := ir.TBuiltinInvalid
	fqn := decl.Name.FQN(modFQN)
	if mod, ok := c.importMap[fqn]; ok {
		importedCUID = mod.CUID
		timportedMod = mod.T
	} else {
		c.error(decl.Name.Pos(), "undefined module '%s'", fqn)
	}
	if isUntyped(timportedMod) {
		return
	}
	sym := c.newTopDeclSymbol(ir.ModuleSymbol, importedCUID, fqn, ir.DGABI, public, decl.Alias.Literal, decl.Alias.Pos(), false)
	sym.T = timportedMod
	sym.Flags |= ir.SymFlagReadOnly
	decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
}

func (c *checker) setUseSymbol(decl *ir.UseDecl, CUID int, modFQN string, public bool, topDecl bool) {
	key := c.nextSymKey()
	sym := ir.NewSymbol(ir.UnknownSymbol, key, CUID, modFQN, decl.Alias.Literal, decl.Alias.Pos())
	sym.Public = public
	sym.ABI = ir.DGABI
	sym.Flags |= ir.SymFlagDefined
	if topDecl {
		sym.Flags |= ir.SymFlagTopDecl
	}
	decl.Sym = sym
}

func (c *checker) insertUseSymbol(decl *ir.UseDecl, CUID int, modFQN string, public bool, topDecl bool) {
	c.setUseSymbol(decl, CUID, modFQN, public, topDecl)
	decl.Sym = c.insertSymbol(c.scope, decl.Sym.Name, decl.Sym)
}

func (c *checker) setLocalTypeDeclSymbol(decl *ir.TypeDecl, CUID int, modFQN string) {
	key := c.nextSymKey()
	sym := ir.NewSymbol(ir.TypeSymbol, key, CUID, modFQN, decl.Name.Literal, decl.Name.Pos())
	sym.Flags = ir.SymFlagDefined
	decl.Sym = sym
}

func (c *checker) setLocalValDeclSymbol(decl *ir.ValDecl, CUID int, modFQN string) {
	key := c.nextSymKey()
	sym := ir.NewSymbol(ir.ValSymbol, key, CUID, modFQN, decl.Name.Literal, decl.Name.Pos())
	sym.Flags = ir.SymFlagDefined
	if (decl.Flags & ir.AstFlagField) != 0 {
		sym.Flags |= ir.SymFlagField
	}
	sym.Public = (decl.Flags & ir.AstFlagPublic) != 0
	decl.Sym = sym
}

func (c *checker) insertLocalValDeclSymbol(decl *ir.ValDecl, CUID int, modFQN string) {
	c.setLocalValDeclSymbol(decl, CUID, modFQN)
	decl.Sym = c.insertSymbol(c.scope, decl.Sym.Name, decl.Sym)
}

func (c *checker) insertFunDeclSignature(decl *ir.FuncDecl, parentScope *ir.Scope) {
	sym := decl.Sym
	decl.Name.Sym = sym
	decl.Scope = ir.NewScope("fun", parentScope, sym.CUID)
	defer c.setScope(c.setScope(decl.Scope))
	for _, param := range decl.Params {
		c.insertLocalValDeclSymbol(param, sym.CUID, sym.ModFQN)
	}
	c.insertLocalValDeclSymbol(decl.Return, sym.CUID, sym.ModFQN)
	if decl.Body != nil {
		decl.Body.Scope = decl.Scope
	}
}

func (c *checker) insertStructDeclBody(decl *ir.StructDecl, methodScope *ir.Scope) {
	sym := decl.Sym
	defer c.setScope(c.setScope(decl.Scope))
	var fields []ir.Field
	for _, field := range decl.Fields {
		c.insertLocalValDeclSymbol(field, sym.CUID, sym.ModFQN)
		if field.Sym != nil {
			fields = append(fields, ir.Field{Name: field.Sym.Name, T: ir.TBuiltinUnknown})
		}
	}
	for _, method := range decl.Methods {
		c.patchSelf(method, sym.Name)
		pubField := (method.Flags & ir.AstFlagPublic) != 0
		name := "dg." + sym.Name + "." + method.Name.Literal
		sym := c.newTopDeclSymbol(ir.FuncSymbol, sym.CUID, sym.ModFQN, sym.ABI, pubField, name, method.Name.Pos(), !method.SignatureOnly())
		method.Sym = c.insertSymbol(c.scope, method.Name.Literal, sym)
		if method.Sym != nil {
			c.insertFunDeclSignature(method, methodScope)
			method.Sym.Flags |= ir.SymFlagMethod
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

func (c *checker) patchSelf(decl *ir.FuncDecl, structName string) {
	if len(decl.Params) > 0 {
		param := decl.Params[0]
		if param.Name.Tok == token.Placeholder {
			param.Name.Tok = token.Ident
			param.Name.Literal = "self"
			if !param.Name.Pos().IsValid() {
				param.Name.SetRange(param.Type.Pos(), param.Type.EndPos())
			}
		}
	}
}
