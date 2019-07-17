package semantics

import (
	"fmt"
	"strings"

	"github.com/cjo5/dingo/internal/ir"
	"github.com/cjo5/dingo/internal/token"
)

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

func (c *checker) insertBuiltinModuleFieldSymbols(CUID int, modFQN string) {
	sym := ir.NewSymbol(ir.ValSymbol, CUID, CUID, modFQN, "modfqn_", token.NoPosition)
	sym.Key = c.nextSymKey()
	sym.Public = true
	sym.Flags = builtinSymFlags | ir.SymFlagConst
	sym.T = ir.NewSliceType(ir.TBuiltinInt8, true, true)
	lit := ir.NewStringLit(modFQN)
	lit.T = sym.T
	c.scope.Insert(sym.Name, sym)
	c.constMap[sym.Key] = lit
}

func (c *checker) newTopDeclSymbol(kind ir.SymbolKind, defCUID int, CUID int, modFQN string, abi string, public bool, name string, pos token.Position, definition bool) *ir.Symbol {
	flags := ir.SymFlagTopDecl
	if definition {
		flags |= ir.SymFlagDefined
	}
	sym := ir.NewSymbol(kind, defCUID, CUID, modFQN, name, pos)
	sym.Key = c.nextSymKey()
	sym.Public = public
	sym.ABI = abi
	sym.Flags = flags
	return sym
}

func (c *checker) setLocalTypeDeclSymbol(decl *ir.TypeDecl, CUID int, modFQN string) {
	sym := ir.NewSymbol(ir.TypeSymbol, CUID, CUID, modFQN, decl.Name.Literal, decl.Name.Pos())
	sym.Key = c.nextSymKey()
	sym.Flags = ir.SymFlagDefined
	decl.Sym = sym
}

func (c *checker) setLocalValDeclSymbol(decl *ir.ValDecl, CUID int, modFQN string) {
	sym := ir.NewSymbol(ir.ValSymbol, CUID, CUID, modFQN, decl.Name.Literal, decl.Name.Pos())
	sym.Key = c.nextSymKey()
	sym.Flags = ir.SymFlagDefined
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
	decl.Scope = ir.NewScope(ir.LocalScope, parentScope, sym.CUID)
	prevScope := c.setScope(decl.Scope)
	for _, param := range decl.Params {
		c.insertLocalValDeclSymbol(param, sym.CUID, sym.ModFQN)
	}
	c.insertLocalValDeclSymbol(decl.Return, sym.CUID, sym.ModFQN)
	if decl.Body != nil {
		decl.Body.Scope = decl.Scope
	}
	c.setScope(prevScope)
}

func (c *checker) insertStructDeclBody(decl *ir.StructDecl, methodScope *ir.Scope) {
	sym := decl.Sym
	prevScope := c.setScope(decl.Scope)
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
		sym := c.newTopDeclSymbol(ir.FuncSymbol, sym.CUID, sym.CUID, sym.ModFQN, sym.ABI, pubField, name, method.Name.Pos(), !method.SignatureOnly())
		method.Sym = c.insertSymbol(c.scope, method.Name.Literal, sym)
		if method.Sym != nil {
			c.insertFunDeclSignature(method, methodScope)
			method.Sym.Flags |= ir.SymFlagMethod
			if method.SignatureOnly() {
				c.error(method.Pos(), "method must have a body")
			}
		}
	}
	c.setScope(prevScope)
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

func (c *checker) insertImportSymbols(decl *ir.ImportDecl, CUID int, modFQN string, topDecl bool, public bool) {
	nameParts := strings.Split(ir.ExprToFQN(decl.Name), ".")
	var scopeParts []string
	// Find relative import components (mroot, msuper, mself) and replace them with the corresponding fqn
	for _, part := range nameParts {
		if part == ir.RootModuleName || part == ir.ParentModuleName || part == ir.SelfModuleName {
			scopeParts = append(scopeParts, part)
		} else {
			break
		}
	}
	if len(scopeParts) > 0 {
		nameParts = nameParts[len(scopeParts):]
	} else {
		scopeParts = append(scopeParts, ir.SelfModuleName)
	}
	relativeSym := c.scope.Lookup(scopeParts[0])
	for i := 1; i < len(scopeParts); i++ {
		scope := relativeSym.T.(*ir.ModuleType).Scope
		relativeSym = scope.Lookup(scopeParts[i])
	}
	fqn := strings.Join(nameParts, ".")
	if len(relativeSym.ModFQN) > 0 && len(fqn) > 0 {
		fqn = fmt.Sprintf("%s.%s", relativeSym.ModFQN, fqn)
	} else if len(relativeSym.ModFQN) > 0 {
		fqn = relativeSym.ModFQN
	}
	if decl.Alias == nil {
		pos := token.NoPosition
		switch name := decl.Name.(type) {
		case *ir.Ident:
			pos = name.Pos()
		case *ir.DotExpr:
			pos = name.Name.Pos()
		}
		if len(nameParts) > 0 {
			decl.Alias = ir.NewIdent2(token.Ident, nameParts[len(nameParts)-1])
		} else {
			decl.Alias = ir.NewIdent1(token.Placeholder)
		}
		decl.Alias.SetRange(pos, pos)
	}
	importedCUID := -1
	importedTMod := ir.TBuiltinInvalid
	if decl.Decl == token.Import {
		if mod, ok := c.importMap[fqn]; ok {
			importedCUID = mod.CUID
			importedTMod = mod.T
		} else {
			c.error(decl.Name.Pos(), "undefined public module '%s'", fqn)
		}
	} else {
		if mod, ok := c.objectList.importMap[fqn]; ok {
			importedCUID = mod.CUID
			importedTMod = mod.T
			if public && !mod.Public {
				public = false
				c.error(decl.Name.Pos(), "private module '%s' cannot be re-exported as public", fqn)
			}
		} else {
			c.error(decl.Name.Pos(), "undefined local module '%s'", fqn)
		}
	}
	if isUntyped(importedTMod) {
		return
	}
	defaultFlags := 0
	if topDecl {
		defaultFlags |= ir.SymFlagTopDecl
	} else {
		defaultFlags |= ir.SymFlagDefined
	}
	symKey := c.nextSymKey()
	sym := ir.NewSymbol(ir.ModuleSymbol, c.scope.CUID, importedCUID, fqn, decl.Alias.Literal, decl.Alias.Pos())
	sym.Key = symKey
	sym.T = importedTMod
	sym.Public = public
	sym.Flags = ir.SymFlagReadOnly | defaultFlags
	decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
	for _, item := range decl.Items {
		if item.Alias == nil {
			item.Alias = ir.NewIdent2(token.Ident, item.Name.Literal)
			item.Alias.SetRange(item.Name.Pos(), item.Name.EndPos())
		}
		itemSym := ir.NewSymbol(ir.ImportSymbol, c.scope.CUID, importedCUID, fqn, item.Name.Literal, item.Alias.Pos())
		itemSym.Key = symKey
		itemSym.Public = item.Visibilty.Is(token.Public)
		itemSym.Flags = defaultFlags
		item.Name.Sym = c.insertSymbol(c.scope, item.Alias.Literal, itemSym)
	}
}
