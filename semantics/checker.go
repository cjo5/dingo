package semantics

import (
	"fmt"
	"math/big"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"
)

var builtinScope = ir.NewScope(ir.RootScope, nil)

func addBuiltinType(t ir.Type) {
	sym := &ir.Symbol{}
	sym.ID = ir.TypeSymbol
	sym.T = t
	sym.Flags = ir.SymFlagCastable
	sym.Name = t.ID().String()
	sym.Pos = token.NoPosition
	builtinScope.Insert(sym)
}

func init() {
	addBuiltinType(ir.TBuiltinVoid)
	addBuiltinType(ir.TBuiltinBool)
	addBuiltinType(ir.TBuiltinString)
	addBuiltinType(ir.TBuiltinUInt64)
	addBuiltinType(ir.TBuiltinInt64)
	addBuiltinType(ir.TBuiltinUInt32)
	addBuiltinType(ir.TBuiltinInt32)
	addBuiltinType(ir.TBuiltinUInt16)
	addBuiltinType(ir.TBuiltinInt16)
	addBuiltinType(ir.TBuiltinUInt8)
	addBuiltinType(ir.TBuiltinInt8)
	addBuiltinType(ir.TBuiltinFloat64)
	addBuiltinType(ir.TBuiltinFloat32)
}

type checker struct {
	set    *ir.ModuleSet
	errors *common.ErrorList

	// State that changes when visiting nodes
	scope   *ir.Scope
	mod     *ir.Module
	fileCtx *ir.FileContext
	topDecl ir.TopDecl
}

func newChecker(set *ir.ModuleSet) *checker {
	c := &checker{set: set, scope: builtinScope}
	c.errors = &common.ErrorList{}
	return c
}

func (c *checker) resetWalkState() {
	c.mod = nil
	c.fileCtx = nil
	c.topDecl = nil
}

func (c *checker) openScope(id ir.ScopeID) {
	c.scope = ir.NewScope(id, c.scope)
}

func (c *checker) closeScope() {
	c.scope = c.scope.Outer
}

func setScope(c *checker, scope *ir.Scope) (*checker, *ir.Scope) {
	curr := c.scope
	c.scope = scope
	return c, curr
}

func (c *checker) visibilityScope(tok token.Token) *ir.Scope {
	var scope *ir.Scope
	if tok.Is(token.Public) {
		scope = c.mod.Public
	} else if tok.Is(token.Internal) {
		scope = c.mod.Internal
	} else if tok.Is(token.Private) {
		scope = c.fileScope()
	} else {
		panic(fmt.Sprintf("Unhandled visibility %s", tok))
	}
	return scope
}

func (c *checker) fileScope() *ir.Scope {
	if c.fileCtx != nil {
		return c.fileCtx.Scope
	}
	return nil
}

func (c *checker) setTopDecl(decl ir.TopDecl) {
	c.topDecl = decl
	c.fileCtx = decl.Context()
	c.scope = c.fileCtx.Scope
}

func (c *checker) error(pos token.Position, format string, args ...interface{}) {
	filename := ""
	if c.fileCtx != nil {
		filename = c.fileCtx.Path
	}
	c.errors.Add(filename, pos, format, args...)
}

func (c *checker) insert(scope *ir.Scope, id ir.SymbolID, name string, pos token.Position, src ir.Decl) *ir.Symbol {
	sym := ir.NewSymbol(id, scope.ID, c.mod.ID, name, pos, src)
	if existing := scope.Insert(sym); existing != nil {
		msg := fmt.Sprintf("redeclaration of '%s', previously declared at %s", name, existing.Pos)
		c.error(pos, msg)
		return nil
	}
	return sym
}

func (c *checker) lookup(name string) *ir.Symbol {
	if existing := c.scope.Lookup(name); existing != nil {
		return existing
	}
	return nil
}

func (c *checker) sortModules() {
	for _, mod := range c.set.Modules {
		mod.Color = ir.NodeColorWhite
	}

	var sortedModules []*ir.Module

	for _, mod := range c.set.Modules {
		if mod.Color == ir.NodeColorWhite {
			if !sortModuleDependencies(mod, &sortedModules) {
				// This shouldn't actually happen since cycles are checked when loading imports
				panic("Cycle detected")
			}
		}
	}

	c.set.Modules = sortedModules
}

// Returns false if cycle
func sortModuleDependencies(mod *ir.Module, sortedModules *[]*ir.Module) bool {
	if mod.Color == ir.NodeColorBlack {
		return true
	} else if mod.Color == ir.NodeColorGray {
		return false
	}
	mod.Color = ir.NodeColorGray
	for _, file := range mod.Files {
		for _, imp := range file.Imports {
			if !sortModuleDependencies(imp.Mod, sortedModules) {
				return false
			}
		}
	}
	mod.Color = ir.NodeColorBlack
	*sortedModules = append(*sortedModules, mod)
	return true
}

func (c *checker) sortDecls() {
	for _, mod := range c.set.Modules {
		for _, decl := range mod.Decls {
			decl.SetNodeColor(ir.NodeColorWhite)
		}
	}

	for _, mod := range c.set.Modules {
		var sortedDecls []ir.TopDecl
		for _, decl := range mod.Decls {
			sym := decl.Symbol()
			if sym == nil {
				continue
			}

			var cycleTrace []ir.TopDecl
			if !sortDeclDependencies(decl, &cycleTrace, &sortedDecls) {
				// Report most specific cycle
				i, j := 0, len(cycleTrace)-1
				for ; i < len(cycleTrace) && j >= 0; i, j = i+1, j-1 {
					if cycleTrace[i] == cycleTrace[j] {
						break
					}
				}

				if i < j {
					decl = cycleTrace[j]
					cycleTrace = cycleTrace[i:j]
				}

				sym.Flags |= ir.SymFlagDepCycle

				trace := common.NewTrace(fmt.Sprintf("%s uses:", sym.Name), nil)
				for i := len(cycleTrace) - 1; i >= 0; i-- {
					s := cycleTrace[i].Symbol()
					s.Flags |= ir.SymFlagDepCycle
					line := cycleTrace[i].Context().Path + ":" + s.Name
					trace.Lines = append(trace.Lines, line)
				}
				c.errors.AddTrace(decl.Context().Path, sym.Pos, trace, "initializer cycle detected")
			}
		}
		mod.Decls = sortedDecls
	}
}

// Returns false if cycle
func sortDeclDependencies(decl ir.TopDecl, trace *[]ir.TopDecl, sortedDecls *[]ir.TopDecl) bool {
	color := decl.NodeColor()
	if color == ir.NodeColorBlack {
		return true
	} else if color == ir.NodeColorGray {
		return false
	}

	sortOK := true
	decl.SetNodeColor(ir.NodeColorGray)
	for _, dep := range decl.Dependencies() {
		if !sortDeclDependencies(dep, trace, sortedDecls) {
			*trace = append(*trace, dep)
			sortOK = false
			break
		}
	}
	decl.SetNodeColor(ir.NodeColorBlack)
	*sortedDecls = append(*sortedDecls, decl)
	return sortOK
}

// Returns false if error
func (c *checker) tryCastLiteral(expr ir.Expr, target ir.Type) bool {
	if ir.IsNumericType(expr.Type()) && ir.IsNumericType(target) {
		lit, _ := expr.(*ir.BasicLit)
		if lit != nil {
			castResult := typeCastNumericLiteral(lit, target)

			if castResult == numericCastOK {
				return true
			}

			if castResult == numericCastOverflows {
				c.error(lit.Value.Pos, "constant expression %s overflows %s", lit.Value.Literal, target)
			} else if castResult == numericCastTruncated {
				c.error(lit.Value.Pos, "type mismatch: constant float expression %s not compatible with %s", lit.Value.Literal, target)
			} else {
				panic(fmt.Sprintf("Unhandled numeric cast result %d", castResult))
			}

			return false
		}
	}
	return true
}

// Returns false if error
func (c *checker) tryCoerceBigNumber(expr ir.Expr) bool {
	t := expr.Type()
	if t.ID() == ir.TBigInt {
		return c.tryCastLiteral(expr, ir.TBuiltinInt32)
	} else if t.ID() == ir.TBigFloat {
		return c.tryCastLiteral(expr, ir.TBuiltinFloat64)
	}
	return true
}

// Returns Ident that was declared as const. Nil otherwise.
func (c *checker) checkConstant(expr ir.Expr) *ir.Ident {
	return checkConstantRecursively(expr)
}

func checkConstantRecursively(expr ir.Expr) *ir.Ident {
	switch t := expr.(type) {
	case *ir.Ident:
		if t.Sym != nil && t.Sym.Constant() {
			return t
		}
	case *ir.DotExpr:
		if t.Name.Sym != nil {
			if t.Name.Sym.Constant() {
				return t.Name
			}
		} else {
			return nil
		}
		return checkConstantRecursively(t.X)
	}
	return nil
}

func compatibleTypes(from ir.Type, to ir.Type) bool {
	switch {
	case from.IsEqual(to):
		return true
	case ir.IsNumericType(from):
		switch {
		case ir.IsNumericType(to):
			return true
		default:
			return false
		}
	default:
		return false
	}
}

type numericCastResult int

const (
	numericCastOK numericCastResult = iota
	numericCastFails
	numericCastOverflows
	numericCastTruncated
)

func toBigFloat(val *big.Int) *big.Float {
	res := big.NewFloat(0)
	res.SetInt(val)
	return res
}

func toBigInt(val *big.Float) *big.Int {
	if !val.IsInt() {
		return nil
	}
	res := big.NewInt(0)
	val.Int(res)
	return res
}

func integerOverflows(val *big.Int, t ir.TypeID) bool {
	fits := true

	switch t {
	case ir.TBigInt:
		// OK
	case ir.TUInt64:
		fits = 0 <= val.Cmp(ir.BigIntZero) && val.Cmp(ir.MaxU64) <= 0
	case ir.TUInt32:
		fits = 0 <= val.Cmp(ir.BigIntZero) && val.Cmp(ir.MaxU32) <= 0
	case ir.TUInt16:
		fits = 0 <= val.Cmp(ir.BigIntZero) && val.Cmp(ir.MaxU16) <= 0
	case ir.TUInt8:
		fits = 0 <= val.Cmp(ir.BigIntZero) && val.Cmp(ir.MaxU8) <= 0
	case ir.TInt64:
		fits = 0 <= val.Cmp(ir.MinI64) && val.Cmp(ir.MaxI64) <= 0
	case ir.TInt32:
		fits = 0 <= val.Cmp(ir.MinI32) && val.Cmp(ir.MaxI32) <= 0
	case ir.TInt16:
		fits = 0 <= val.Cmp(ir.MinI16) && val.Cmp(ir.MaxI16) <= 0
	case ir.TInt8:
		fits = 0 <= val.Cmp(ir.MinI8) && val.Cmp(ir.MaxI8) <= 0
	}

	return !fits
}

func floatOverflows(val *big.Float, t ir.TypeID) bool {
	fits := true

	switch t {
	case ir.TBigFloat:
		// OK
	case ir.TFloat64:
		fits = 0 <= val.Cmp(ir.MinF64) && val.Cmp(ir.MaxF64) <= 0
	case ir.TFloat32:
		fits = 0 <= val.Cmp(ir.MinF32) && val.Cmp(ir.MaxF32) <= 0
	}

	return !fits
}

func typeCastNumericLiteral(lit *ir.BasicLit, target ir.Type) numericCastResult {
	res := numericCastOK
	id := target.ID()

	switch t := lit.Raw.(type) {
	case *big.Int:
		switch id {
		case ir.TBigInt, ir.TUInt64, ir.TUInt32, ir.TUInt16, ir.TUInt8, ir.TInt64, ir.TInt32, ir.TInt16, ir.TInt8:
			if integerOverflows(t, id) {
				res = numericCastOverflows
			}
		case ir.TBigFloat, ir.TFloat64, ir.TFloat32:
			fval := toBigFloat(t)
			if floatOverflows(fval, id) {
				res = numericCastOverflows
			} else {
				lit.Raw = fval
			}
		default:
			return numericCastFails
		}
	case *big.Float:
		switch id {
		case ir.TBigInt, ir.TUInt64, ir.TUInt32, ir.TUInt16, ir.TUInt8, ir.TInt64, ir.TInt32, ir.TInt16, ir.TInt8:
			if ival := toBigInt(t); ival != nil {
				if integerOverflows(ival, id) {
					res = numericCastOverflows
				} else {
					lit.Raw = ival
				}
			} else {
				res = numericCastTruncated
			}
		case ir.TBigFloat, ir.TFloat64, ir.TFloat32:
			if floatOverflows(t, id) {
				res = numericCastOverflows
			}
		default:
			return numericCastFails
		}
	default:
		return numericCastFails
	}

	if res == numericCastOK {
		lit.T = ir.NewBasicType(id)
	}

	return res
}
