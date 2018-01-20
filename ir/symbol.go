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

// Position for symbol.
type Position struct {
	Filename string
	Pos      token.Position
}

// Symbol represents a unique symbol/identifier.
type Symbol struct {
	ID      SymbolID
	ScopeID ScopeID
	Public  bool
	Name    string
	DeclPos Position
	DefPos  Position // Different from DeclPos if symbol was declared before defined
	T       Type
	Flags   int
}

// NewPosition creates a new position.
func NewPosition(filename string, pos token.Position) Position {
	return Position{Pos: pos, Filename: filename}
}

// NewSymbol creates a new symbol.
func NewSymbol(id SymbolID, scopeID ScopeID, public bool, name string, pos Position) *Symbol {
	return &Symbol{ID: id, ScopeID: scopeID, Public: public, Name: name, DeclPos: pos, DefPos: pos, Flags: 0}
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

func (p Position) String() string {
	if len(p.Filename) > 0 {
		return fmt.Sprintf("%s:%s", p.Filename, p.Pos)
	}
	return p.Pos.String()
}

func (s *Symbol) String() string {
	return fmt.Sprintf("%s:%s:%s", s.ID, s.DeclPos, s.Name)
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
