package ast

type SymbolID int

const (
	Invalid = iota
	Var
	Func
)

type Scope struct {
	Outer   *Scope
	Symbols map[string]*Symbol
}

type Symbol struct {
	ID   SymbolID
	Name string
}

// NewScope creates a new scope nested in the outer scope.
func NewScope(outer *Scope) *Scope {
	const n = 4 // initial scope capacity
	return &Scope{outer, make(map[string]*Symbol, n)}
}

// NewSymbol creates a new symbol of a given ID and name.
func NewSymbol(id SymbolID, name string) *Symbol {
	return &Symbol{ID: id, Name: name}
}
