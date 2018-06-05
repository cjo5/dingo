package ir

import "github.com/jhnl/dingo/token"
import "fmt"

// SymbolID identifies the type of symbol.
type SymbolID int

// Symbol IDs.
const (
	ValSymbol SymbolID = iota
	ConstSymbol
	FuncSymbol
	ModuleSymbol
	TypeSymbol
)

// Symbol flags.
const (
	SymFlagReadOnly = 1 << 1
	SymFlagDefined  = 1 << 2
)

// Symbol represents a unique symbol/identifier.
type Symbol struct {
	ID      SymbolID
	Parent  *Scope
	Public  bool
	Name    string
	DeclPos token.Position
	DefPos  token.Position // Different from DeclPos if symbol was declared before defined
	T       Type
	Flags   int
}

// NewSymbol creates a new symbol.
func NewSymbol(id SymbolID, parent *Scope, public bool, name string, pos token.Position) *Symbol {
	return &Symbol{ID: id, Parent: parent, Public: public, Name: name, DeclPos: pos, DefPos: pos, Flags: 0}
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
	return fmt.Sprintf("%s:%s:%s", s.ID, s.DeclPos, s.Name)
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

func (s *Symbol) ModFQN() string {
	return s.Parent.FQN
}
