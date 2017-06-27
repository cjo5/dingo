package semantics

type Scope struct {
	Outer   *Scope
	Symbols map[string]*Symbol
}

// NewScope creates a new scope nested in the outer scope.
func NewScope(outer *Scope) *Scope {
	const n = 4 // initial scope capacity
	return &Scope{outer, make(map[string]*Symbol, n)}
}

func (s *Scope) Insert(sym *Symbol) *Symbol {
	var existing *Symbol
	if existing = s.Symbols[sym.Name.Literal]; existing == nil {
		s.Symbols[sym.Name.Literal] = sym
	}
	return existing
}

func (s *Scope) Lookup(name string) *Symbol {
	return doLookup(s, name)
}

func (s *Scope) LookupFuncDecl(name string) *FuncDecl {
	sym := s.Lookup(name)
	if sym == nil {
		return nil
	}

	decl, _ := sym.Func()
	return decl
}

func doLookup(s *Scope, name string) *Symbol {
	if s == nil {
		return nil
	}
	if res := s.Symbols[name]; res != nil {
		return res
	}
	return doLookup(s.Outer, name)
}
