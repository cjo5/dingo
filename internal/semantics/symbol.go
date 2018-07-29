package semantics

import (
	"fmt"
	"strings"

	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

func (c *checker) insertSymbol(scope *ir.Scope, alias string, sym *ir.Symbol) *ir.Symbol {
	if existing := scope.Insert(alias, sym); existing != nil {
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

func (c *checker) insertLocalDeclSymbol(decl ir.Decl, CUID int, modFQN string) {
	switch decl := decl.(type) {
	case *ir.ImportDecl:
		c.insertImportSymbols(decl, CUID, modFQN, false, false)
	case *ir.TypeDecl:
		sym := ir.NewSymbol(ir.TypeSymbol, c.scope, CUID, modFQN, decl.Name.Literal, decl.Name.Pos())
		sym.Key = c.nextSymKey()
		sym.Flags = ir.SymFlagDefined
		decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
	case *ir.ValDecl:
		sym := ir.NewSymbol(ir.ValSymbol, c.scope, CUID, modFQN, decl.Name.Literal, decl.Name.Pos())
		sym.Key = c.nextSymKey()
		sym.Flags = ir.SymFlagDefined
		sym.Public = (decl.Flags & ir.AstFlagPublic) != 0
		if decl.Name.Tok != token.Underscore {
			sym = c.insertSymbol(c.scope, sym.Name, sym)
		}
		decl.Sym = sym
	default:
		panic(fmt.Sprintf("Unhandled decl %T", decl))
	}
}

func (c *checker) insertImportSymbols(decl *ir.ImportDecl, CUID int, modFQN string, topDecl bool, public bool) {
	fqn := ir.ExprToFQN(decl.Name)
	nameParts := strings.Split(fqn, ".")
	var scopeParts []string
	// Find relative import components (mroot, msuper, mself) and replace them with the corresponding fqn
	for _, part := range nameParts {
		if part == ir.RootModuleName || part == ir.SelfModuleName {
			scopeParts = append(scopeParts, part)
			break
		} else if part == ir.ParentModuleName {
			scopeParts = append(scopeParts, part)
		} else {
			break
		}
	}
	if len(scopeParts) > 0 {
		nameParts = nameParts[len(scopeParts):]
		sym := c.scope.Lookup(scopeParts[0])
		for i := 1; i < len(scopeParts); i++ {
			scope := sym.T.(*ir.ModuleType).Scope
			sym = scope.Lookup(scopeParts[i])
		}
		nameFQN := strings.Join(nameParts, ".")
		if len(sym.ModFQN) > 0 && len(nameFQN) > 0 {
			fqn = fmt.Sprintf("%s.%s", sym.ModFQN, nameFQN)
		} else if len(sym.ModFQN) > 0 {
			fqn = sym.ModFQN
		} else {
			fqn = nameFQN
		}
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
			decl.Alias = ir.NewIdent1(token.Underscore)
		}
		decl.Alias.SetRange(pos, pos)
	}
	importedCUID := -1
	importedTMod := ir.TBuiltinInvalid
	if mod, ok := c.importMap[fqn]; ok {
		importedCUID = mod.CUID
		importedTMod = mod.T
	} else {
		c.error(decl.Name.Pos(), "undefined public module '%s'", fqn)
	}
	defaultFlags := 0
	if topDecl {
		defaultFlags |= ir.SymFlagTopDecl
	}
	symKey := c.nextSymKey()
	sym := ir.NewSymbol(ir.ModuleSymbol, c.scope, importedCUID, fqn, decl.Alias.Literal, decl.Alias.Pos())
	sym.Key = symKey
	sym.T = importedTMod
	sym.Public = public
	sym.Flags = ir.SymFlagReadOnly | defaultFlags
	if decl.Alias.Tok != token.Underscore {
		sym = c.insertSymbol(c.scope, sym.Name, sym)
	}
	decl.Sym = sym
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
