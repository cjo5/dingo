package semantics

import (
	"fmt"
	"strings"

	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
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
	sym := ir.NewSymbol(ir.ValSymbol, c.scope, CUID, modFQN, "__fqn__", token.NoPosition)
	sym.Key = c.nextSymKey()
	sym.Public = true
	sym.Flags = builtinSymFlags | ir.SymFlagConst
	sym.T = ir.NewSliceType(ir.TBuiltinInt8, true, true)
	lit := ir.NewStringLit(modFQN)
	lit.T = sym.T
	c.scope.Insert(sym.Name, sym)
	c.constMap[sym.Key] = lit
}

func (c *checker) newTopDeclSymbol(kind ir.SymbolKind, CUID int, modFQN string, abi string, public bool, name *ir.Ident, definition bool) *ir.Symbol {
	flags := ir.SymFlagTopDecl
	if definition {
		flags |= ir.SymFlagDefined
	}
	sym := ir.NewSymbol(kind, c.scope, CUID, modFQN, name.Literal, name.Pos())
	sym.Key = c.nextSymKey()
	sym.Public = public
	sym.ABI = abi
	sym.Flags = flags
	return sym
}

func (c *checker) setLocalTypeDeclSymbol(decl *ir.TypeDecl, CUID int, modFQN string) {
	sym := ir.NewSymbol(ir.TypeSymbol, c.scope, CUID, modFQN, decl.Name.Literal, decl.Name.Pos())
	sym.Key = c.nextSymKey()
	sym.Flags = ir.SymFlagDefined
	decl.Sym = sym
}

func (c *checker) setLocalValDeclSymbol(decl *ir.ValDecl, CUID int, modFQN string) {
	sym := ir.NewSymbol(ir.ValSymbol, c.scope, CUID, modFQN, decl.Name.Literal, decl.Name.Pos())
	sym.Key = c.nextSymKey()
	sym.Flags = ir.SymFlagDefined
	sym.Public = (decl.Flags & ir.AstFlagPublic) != 0
	decl.Sym = sym
}

func (c *checker) insertLocalValDeclSymbol(decl *ir.ValDecl, CUID int, modFQN string) {
	c.setLocalValDeclSymbol(decl, CUID, modFQN)
	decl.Sym = c.insertSymbol(c.scope, decl.Sym.Name, decl.Sym)
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
				c.error(decl.Name.Pos(), "module '%s' is private and cannot be re-exported as public", fqn)
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
	sym := ir.NewSymbol(ir.ModuleSymbol, c.scope, importedCUID, fqn, decl.Alias.Literal, decl.Alias.Pos())
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
		itemSym := ir.NewSymbol(ir.ImportSymbol, c.scope, importedCUID, fqn, item.Name.Literal, item.Alias.Pos())
		itemSym.Key = symKey
		itemSym.Public = item.Visibilty.Is(token.Public)
		itemSym.Flags = defaultFlags
		item.Name.Sym = c.insertSymbol(c.scope, item.Alias.Literal, itemSym)
	}
}
