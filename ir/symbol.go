package ir

import "github.com/jhnl/dingo/token"
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
	SymFlagReadOnly = 1 << 1
	SymFlagDefined  = 1 << 2
)

// Symbol represents a unique symbol/identifier in the program.
type Symbol struct {
	ID      SymbolID
	ScopeID ScopeID
	Name    string
	Pos     token.Position
	T       Type
	Flags   int
}

// NewSymbol creates a new symbol of a given ID and name.
func NewSymbol(id SymbolID, scopeID ScopeID, name string, pos token.Position) *Symbol {
	return &Symbol{ID: id, ScopeID: scopeID, Name: name, Pos: pos, Flags: 0}
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

func (s *Symbol) ReadOnly() bool {
	return (s.Flags & SymFlagReadOnly) != 0
}

func (s *Symbol) Defined() bool {
	return (s.Flags & SymFlagDefined) != 0
}

func (s *Symbol) Untyped() bool {
	if s.T == nil || IsUntyped(s.T) {
		return true
	}
	return false
}
