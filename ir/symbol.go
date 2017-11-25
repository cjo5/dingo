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
	SymFlagConstant = 1 << 1
	SymFlagCastable = 1 << 2
	SymFlagDefined  = 1 << 3
)

type Symbol struct {
	ID       SymbolID
	ScopeID  ScopeID
	ModuleID int // ID of module where symbol was defined.
	Name     string
	Pos      token.Position
	Src      Decl // Node in ast that created this symbol. Nil if builtin symbol.
	T        Type // The symbol's type.
	Flags    int
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

func (s *Symbol) Defined() bool {
	return (s.Flags & SymFlagDefined) != 0
}

func (s *Symbol) Untyped() bool {
	if s.T == nil || IsUntyped(s.T) {
		return true
	}
	return false
}
