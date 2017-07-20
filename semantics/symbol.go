package semantics

import "github.com/jhnl/interpreter/token"
import "fmt"

// SymbolID identifies the type of symbol.
type SymbolID int

// Symbol IDs.
const (
	ValSymbol SymbolID = iota
	FuncSymbol
	ModuleSymbol
	TypeSymbol
)

// Symbol flags.
const (
	SymFlagDepCycle = 1 << 0
	SymFlagConstant = 1 << 1
	SymFlagCastable = 1 << 2
)

type Symbol struct {
	ID       SymbolID
	ScopeID  ScopeID
	ModuleID int
	Name     string
	Pos      token.Position
	Src      Decl // Node in ast that generated this symbol
	T        Type // Type of symbol
	Flags    int
	Address  int
}

// NewSymbol creates a new symbol of a given ID and name.
func NewSymbol(id SymbolID, scopeID ScopeID, moduleID int, name string, pos token.Position, src Decl) *Symbol {
	return &Symbol{ID: id, ScopeID: scopeID, ModuleID: moduleID, Name: name, Pos: pos, Src: src, Flags: 0}
}

func (s SymbolID) String() string {
	switch s {
	case ValSymbol:
		return "ValSymbol"
	case FuncSymbol:
		return "FuncSymbol"
	case ModuleSymbol:
		return "ModuleSymbol"
	case TypeSymbol:
		return "TypeSymbol"
	default:
		return "Symbol " + string(s)
	}
}

func (s *Symbol) String() string {
	return fmt.Sprintf("%s:%s:%s", s.ID, s.Pos, s.Name)
}

func (s *Symbol) DepCycle() bool {
	return (s.Flags & SymFlagDepCycle) != 0
}

func (s *Symbol) Constant() bool {
	return (s.Flags & SymFlagConstant) != 0
}

func (s *Symbol) Castable() bool {
	return (s.Flags & SymFlagCastable) != 0
}
