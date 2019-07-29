package semantics

import (
	"fmt"

	"github.com/cjo5/dingo/internal/ir"
	"github.com/cjo5/dingo/internal/token"
)

type color int

const (
	whiteColor color = iota
	grayColor  color = iota
	blackColor color = iota
)

type objectList struct {
	filename  string
	CUID      int
	rootScope *ir.Scope
	objects   []*object
}

type object struct {
	d           ir.Decl
	parentScope *ir.Scope
	definition  bool
	deps        map[*object][]*depEdge
	checked     bool
	incomplete  bool
	color       color
}

type depEdge struct {
	pos            token.Position
	isIndirectType bool
}

func newObjectList(filename string, CUID int, rootScope *ir.Scope) *objectList {
	return &objectList{
		filename:  filename,
		CUID:      CUID,
		rootScope: rootScope,
	}
}

func resetColors(matrix []*objectList) {
	for _, list := range matrix {
		list.resetColors()
	}
}

func (l *objectList) resetColors() {
	for _, obj := range l.objects {
		obj.color = whiteColor
	}
}

func newObject(d ir.Decl, parentScope *ir.Scope, definition bool) *object {
	return &object{
		d:           d,
		parentScope: parentScope,
		definition:  definition,
		deps:        make(map[*object][]*depEdge),
		color:       whiteColor,
	}
}

func (d *object) sym() *ir.Symbol {
	return d.d.Symbol()
}

func (d *object) modFQN() string {
	return d.sym().ModFQN
}

func (d *object) CUID() int {
	return d.sym().CUID
}

func (d *object) uniqKey() int {
	return d.sym().UniqKey
}

func (d *object) addEdge(to *object, edge *depEdge) {
	var edges []*depEdge
	if existing, ok := d.deps[to]; ok {
		edges = existing
	}
	edges = append(edges, edge)
	d.deps[to] = edges
}

func (c *checker) initObjectMatrix(modMatrix moduleMatrix) {
	for CUID, modList := range modMatrix {
		root := modList.importMap[""]
		rootScope := root.T.(*ir.ModuleType).Scope()
		c.objectList = newObjectList(modList.filename, CUID, rootScope)
		for _, mod := range modList.mods {
			c.scope = mod.builtinScope
			c.insertBuiltinModuleSymbols(CUID, mod.fqn)
			c.scope = mod.scope
			for _, decl := range mod.decls {
				objects := c.createObjects(decl, CUID, mod.fqn)
				if objects != nil {
					c.objectList.objects = append(c.objectList.objects, objects...)
				}
			}
			for _, obj := range c.objectList.objects {
				if obj.definition {
					c.objectMap[obj.uniqKey()] = obj
				} else if _, ok := c.objectMap[obj.uniqKey()]; !ok {
					c.objectMap[obj.uniqKey()] = obj
				}
			}
		}
		c.objectMatrix = append(c.objectMatrix, c.objectList)
	}
}

func (c *checker) createObjects(decl *ir.TopDecl, CUID int, modFQN string) []*object {
	abi := ir.DGABI
	if decl.ABI != nil {
		abiLit := decl.ABI.Literal
		if ir.IsValidABI(abiLit) {
			abi = abiLit
		} else {
			c.error(decl.ABI.Pos(), "unknown abi '%s'", abiLit)
		}
	}
	public := decl.Visibility.Is(token.Public)
	var objects []*object
	switch decl := decl.D.(type) {
	case *ir.ImportDecl:
		c.insertImportSymbol(decl, CUID, modFQN, public)
		if decl.Sym != nil {
			objects = append(objects, newObject(decl, c.scope, false))
		}
	case *ir.UseDecl:
		c.insertUseSymbol(decl, CUID, modFQN, public, public)
		if decl.Sym != nil {
			objects = append(objects, newObject(decl, c.scope, false))
		}
	case *ir.TypeDecl:
		sym := c.newTopDeclSymbol(ir.TypeSymbol, CUID, modFQN, abi, public, decl.Name.Literal, decl.Name.Pos(), true)
		decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
		if decl.Sym != nil {
			objects = append(objects, newObject(decl, c.scope, true))
		}
	case *ir.ValDecl:
		sym := c.newTopDeclSymbol(ir.ValSymbol, CUID, modFQN, abi, public, decl.Name.Literal, decl.Name.Pos(), true)
		decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
		if decl.Sym != nil {
			objects = append(objects, newObject(decl, c.scope, true))
		}
	case *ir.FuncDecl:
		def := !decl.SignatureOnly()
		sym := c.newTopDeclSymbol(ir.FuncSymbol, CUID, modFQN, abi, public, decl.Name.Literal, decl.Name.Pos(), def)
		decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
		if decl.Sym != nil {
			c.insertFunDeclSignature(decl, c.scope)
			objects = append(objects, newObject(decl, c.scope, true))
		}
	case *ir.StructDecl:
		def := !decl.Opaque
		sym := c.newTopDeclSymbol(ir.TypeSymbol, CUID, modFQN, abi, public, decl.Name.Literal, decl.Name.Pos(), def)
		decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
		if decl.Sym != nil {
			decl.Scope = ir.NewScope("struct_fields", nil, sym.CUID)
			if decl.Opaque {
				if decl.Sym.T.Kind() == ir.TUnknown {
					tstruct := ir.NewStructType(decl.Sym, decl.Scope)
					tstruct.SetBody(nil, true)
					decl.Sym.T = tstruct
					objects = append(objects, newObject(decl, c.scope, def))
				}
			} else {
				selfType := &ir.TypeDecl{
					Name: ir.NewIdent2(token.Ident, ir.SelfType),
					Type: ir.NewIdent2(token.Ident, decl.Name.Literal),
				}

				selfScope := ir.NewScope("struct_self", c.scope, CUID)
				selfType.Sym = c.newTopDeclSymbol(ir.TypeSymbol, CUID, modFQN, abi, false, selfType.Name.Literal, token.NoPosition, true)
				c.insertSymbol(selfScope, selfType.Name.Literal, selfType.Sym)
				c.insertStructDeclBody(decl, selfScope)
				objects = append(objects, newObject(decl, c.scope, def))
				objects = append(objects, newObject(selfType, selfScope, true))
				for _, method := range decl.Methods {
					if method.Sym != nil {
						objects = append(objects, newObject(method, c.scope, !method.SignatureOnly()))
					}
				}
				decl.Methods = nil
			}
		}
	default:
		panic(fmt.Sprintf("Unhandled decl %T", decl))
	}
	return objects
}

func (c *checker) createDeclMatrix() ir.DeclMatrix {
	var declMatrix ir.DeclMatrix
out:
	for _, objList := range c.objectMatrix {
		resetColors(c.objectMatrix)
		var sortedDecls []ir.Decl
		for _, obj := range objList.objects {
			var cycleTrace []*object
			if !sortDeps(obj, &cycleTrace, &sortedDecls) {
				cycleTrace = append(cycleTrace, obj)

				// Find most specific cycle
				j := len(cycleTrace) - 1
				for ; j >= 0; j = j - 1 {
					if cycleTrace[0] == cycleTrace[j] {
						break
					}
				}

				cycleTrace = cycleTrace[:j+1]
				sym := cycleTrace[0].d.Symbol()

				var lines []string
				for i, j := len(cycleTrace)-1, 0; i > 0; i, j = i-1, j+1 {
					next := j + 1
					if next == len(cycleTrace)-1 {
						next = 0
					}

					s := cycleTrace[i].d.Symbol()
					line := fmt.Sprintf("  >> [%d] %s:%s uses [%d]", j, s.Pos, s.Name, next)
					lines = append(lines, line)
				}

				c.ctx.Errors.AddContext(sym.Pos, lines, "cycle detected")
				break out
			}
		}
		declList := &ir.DeclList{
			Filename: objList.filename,
			CUID:     objList.CUID,
			Decls:    sortedDecls,
			Syms:     make(map[int]*ir.Symbol),
		}
		// TODO: fill Syms map when creating the decl list
		for _, decl := range declList.Decls {
			sym := decl.Symbol()
			declList.Syms[sym.UniqKey] = sym
		}
		declMatrix = append(declMatrix, declList)
	}
	return declMatrix
}

func sortDeps(obj *object, trace *[]*object, sorted *[]ir.Decl) bool {
	if obj.color == blackColor {
		return true
	} else if obj.color == grayColor {
		return false
	}

	sortOK := true
	obj.color = grayColor

	var weak []*object

	for dep, edges := range obj.deps {
		if isWeakDep(obj, edges, dep) {
			weak = append(weak, dep)
			continue
		}

		if !sortDeps(dep, trace, sorted) {
			*trace = append(*trace, dep)
			sortOK = false
			break
		}
	}

	obj.color = blackColor
	*sorted = append(*sorted, obj.d)

	if sortOK {
		for _, dep := range weak {
			if dep.color == whiteColor {
				// Ensure dependencies from other modules are included
				if !sortDeps(dep, trace, sorted) {
					*trace = append(*trace, dep)
					sortOK = false
					break
				}
			}
		}
	}

	return sortOK
}

func isWeakDep(from *object, edges []*depEdge, to *object) bool {
	switch from.d.(type) {
	case *ir.FuncDecl:
		return to.d.Symbol().Kind == ir.FuncSymbol
	case *ir.StructDecl:
		weakCount := 0
		for _, edge := range edges {
			if edge.isIndirectType {
				weakCount++
			}
		}
		if weakCount == len(edges) {
			return true
		}
	}
	return false
}
