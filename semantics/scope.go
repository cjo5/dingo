package semantics

import "github.com/jhnl/interpreter/token"

type SymbolID int

const (
	VarSymbol SymbolID = iota
	FuncSymbol
)

type Scope struct {
	Outer   *Scope
	Symbols map[string]*Symbol
}

type Symbol struct {
	ID      SymbolID
	Name    token.Token
	Decl    Node
	Address int // Variable's address in global array or local array
}

// NewScope creates a new scope nested in the outer scope.
func NewScope(outer *Scope) *Scope {
	const n = 4 // initial scope capacity
	return &Scope{outer, make(map[string]*Symbol, n)}
}

// NewSymbol creates a new symbol of a given ID and name.
func NewSymbol(id SymbolID, name token.Token, decl Node) *Symbol {
	return &Symbol{ID: id, Name: name, Decl: decl}
}

func (s *Scope) Insert(sym *Symbol) *Symbol {
	var existing *Symbol
	if existing = s.Symbols[sym.Name.Literal]; existing == nil {
		s.Symbols[sym.Name.Literal] = sym
	}
	return existing
}

func (s *Scope) Lookup(name string) (*Symbol, bool) {
	return doLookup(s, name)
}

func doLookup(s *Scope, name string) (*Symbol, bool) {
	if s == nil {
		return nil, false
	}
	if res := s.Symbols[name]; res != nil {
		return res, s.IsGlobal()
	}
	return doLookup(s.Outer, name)
}

// Address of name. Returns true if global scope.
func (s *Scope) Address(name string) (int, bool) {
	sym, global := doLookup(s, name)
	if sym != nil {
		return sym.Address, global
	}
	return 0, false
}

// IsGlobal returns true if it's the global scope.
func (s *Scope) IsGlobal() bool {
	return s.Outer == nil
}

func (s *Symbol) Pos() string {
	return s.Name.Pos.String()
}
