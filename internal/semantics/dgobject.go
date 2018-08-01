package semantics

import (
	"fmt"

	"github.com/jhnl/dingo/internal/ir"
	"github.com/jhnl/dingo/internal/token"
)

type color int

const (
	whiteColor color = iota
	grayColor  color = iota
	blackColor color = iota
)

type dgObjectList struct {
	filename string
	CUID     int
	objects  []*dgObject
}

type dgObject struct {
	d          ir.Decl
	definition bool
	deps       map[*dgObject][]*depEdge
	checked    bool
	incomplete bool
	color      color
}

type depEdge struct {
	pos            token.Position
	isIndirectType bool
}

func newDgObjectList(filename string, CUID int) *dgObjectList {
	return &dgObjectList{
		filename: filename,
		CUID:     CUID,
	}
}

func resetColors(matrix []*dgObjectList) {
	for _, list := range matrix {
		list.resetColors()
	}
}

func (l *dgObjectList) resetColors() {
	for _, obj := range l.objects {
		obj.color = whiteColor
	}
}

func newDgObject(d ir.Decl, definition bool) *dgObject {
	return &dgObject{
		d:          d,
		definition: definition,
		deps:       make(map[*dgObject][]*depEdge),
		color:      whiteColor,
	}
}

func (d *dgObject) sym() *ir.Symbol {
	return d.d.Symbol()
}

func (d *dgObject) modFQN() string {
	return d.sym().ModFQN
}

func (d *dgObject) CUID() int {
	return d.sym().CUID
}

func (d *dgObject) parentCUID() int {
	return d.sym().ParentCUID()
}

func (d *dgObject) key() int {
	return d.sym().Key
}

func (d *dgObject) addEdge(to *dgObject, edge *depEdge) {
	var edges []*depEdge
	if existing, ok := d.deps[to]; ok {
		edges = existing
	}
	edges = append(edges, edge)
	d.deps[to] = edges
}

func (c *checker) initDgObjectMatrix(modMatrix moduleMatrix) {
	for CUID, modList := range modMatrix {
		objList := newDgObjectList(modList.filename, CUID)
		for _, mod := range modList.mods {
			c.scope = mod.builtinScope
			c.insertBuiltinModuleFieldSymbols(CUID, mod.fqn)
			c.scope = mod.scope
			for _, decl := range mod.decls {
				obj := c.createDgObject(decl, CUID, mod.fqn)
				if obj != nil {
					objList.objects = append(objList.objects, obj)
				}
			}
			for _, obj := range objList.objects {
				if obj.definition {
					c.objectMap[obj.key()] = obj
				} else if _, ok := c.objectMap[obj.key()]; !ok {
					c.objectMap[obj.key()] = obj
				}
			}
		}
		c.objectMatrix = append(c.objectMatrix, objList)
	}
}

func (c *checker) createDgObject(decl *ir.TopDecl, CUID int, modFQN string) *dgObject {
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
	definition := false
	switch decl := decl.D.(type) {
	case *ir.ImportDecl:
		c.insertImportSymbols(decl, CUID, modFQN, true, public)
	case *ir.TypeDecl:
		definition = true
		sym := c.newTopDeclSymbol(ir.TypeSymbol, CUID, modFQN, abi, public, decl.Name, definition)
		decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
	case *ir.ValDecl:
		definition = true
		sym := c.newTopDeclSymbol(ir.ValSymbol, CUID, modFQN, abi, public, decl.Name, definition)
		decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
	case *ir.FuncDecl:
		definition = !decl.SignatureOnly()
		sym := c.newTopDeclSymbol(ir.FuncSymbol, CUID, modFQN, abi, public, decl.Name, definition)
		decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
		if decl.Sym != nil {
			decl.Name.Sym = decl.Sym
			c.openScope(ir.LocalScope)
			defer c.closeScope()
			decl.Scope = c.scope
			for _, param := range decl.Params {
				c.insertLocalValDeclSymbol(param, CUID, modFQN)
			}
			c.insertLocalValDeclSymbol(decl.Return, CUID, modFQN)
			if decl.Body != nil {
				decl.Body.Scope = decl.Scope
			}
		}
	case *ir.StructDecl:
		if !decl.Opaque && len(decl.Fields) == 0 {
			c.error(decl.Pos(), "struct must have at least 1 field")
			return nil
		}
		definition = !decl.Opaque
		sym := c.newTopDeclSymbol(ir.TypeSymbol, CUID, modFQN, abi, public, decl.Name, definition)
		decl.Sym = c.insertSymbol(c.scope, sym.Name, sym)
		if decl.Sym != nil {
			decl.Scope = ir.NewScope(ir.FieldScope, nil, CUID)
			if decl.Sym.T.Kind() == ir.TUnknown || !decl.Opaque {
				tstruct := ir.NewStructType(decl.Sym, decl.Scope)
				if decl.Opaque {
					tstruct.SetBody(nil, true)
				} else {
					var fields []ir.Field
					prevScope := c.setScope(decl.Scope)
					for _, field := range decl.Fields {
						c.insertLocalValDeclSymbol(field, CUID, modFQN)
						if field.Sym != nil {
							fields = append(fields, ir.Field{Name: field.Sym.Name, T: ir.TBuiltinUnknown})
						}
					}
					c.setScope(prevScope)
					tstruct.SetBody(fields, false) // Set untyped fields
				}
				decl.Sym.T = tstruct
			}
			decl.Name.Sym = decl.Sym
		}
	default:
		panic(fmt.Sprintf("Unhandled decl %T", decl))
	}
	if decl.D.Symbol() == nil {
		return nil
	}
	return newDgObject(decl.D, definition)
}

func (c *checker) createDeclMatrix() ir.DeclMatrix {
	var declMatrix ir.DeclMatrix
out:
	for _, objList := range c.objectMatrix {
		resetColors(c.objectMatrix)
		var sortedDecls []ir.Decl
		for _, obj := range objList.objects {
			var cycleTrace []*dgObject
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

				c.errors.AddContext(sym.Pos, lines, "cycle detected")
				break out
			}
		}
		declList := &ir.DeclList{
			Filename: objList.filename,
			CUID:     objList.CUID,
			Decls:    sortedDecls,
		}
		declMatrix = append(declMatrix, declList)
	}
	return declMatrix
}

func sortDeps(obj *dgObject, trace *[]*dgObject, sorted *[]ir.Decl) bool {
	if obj.color == blackColor {
		return true
	} else if obj.color == grayColor {
		return false
	}

	sortOK := true
	obj.color = grayColor

	var weak []*dgObject

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

func isWeakDep(from *dgObject, edges []*depEdge, to *dgObject) bool {
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
