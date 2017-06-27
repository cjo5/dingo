package semantics

import "github.com/jhnl/interpreter/token"

type SymbolID int

const (
	VarSymbol SymbolID = iota
	FuncSymbol
	TypeSymbol
)

type Symbol struct {
	ID       SymbolID
	Name     token.Token
	T        *TType
	Src      Node // Node in ast where the symbol was defined
	Address  int  // Variable's address in global array or local array
	Global   bool
	Constant bool

	// TODO: Add builtin bool
	// TODO: Replace bools with flags
}

// NewSymbol creates a new symbol of a given ID and name.
func NewSymbol(id SymbolID, name token.Token, src Node, global bool) *Symbol {
	return &Symbol{ID: id, Name: name, Src: src, Global: global}
}

func (s *Symbol) Pos() string {
	return s.Name.Pos.String()
}

func (s *Symbol) Func() (*FuncDecl, bool) {
	decl, ok := s.Src.(*FuncDecl)
	return decl, ok
}

func (s *Symbol) Var() (*VarDecl, bool) {
	decl, ok := s.Src.(*VarDecl)
	return decl, ok
}
