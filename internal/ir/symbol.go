package ir

import "github.com/jhnl/dingo/internal/token"
import "fmt"

// CABI name.
const CABI = "c"

// DGABI name.
const DGABI = "dg"

// SymbolID identifies the type of symbol.
type SymbolID int

// Symbol IDs.
const (
	ValSymbol SymbolID = iota
	ConstSymbol
	FuncSymbol
	ModuleSymbol
	StructSymbol
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
	ABI     string
	Name    string
	DeclPos token.Position
	DefPos  token.Position // Different from DeclPos if symbol was declared before defined
	T       Type
	Flags   int
}

// NewSymbol creates a new symbol.
func NewSymbol(id SymbolID, parent *Scope, public bool, abi string, name string, pos token.Position) *Symbol {
	return &Symbol{ID: id, Parent: parent, Public: public, ABI: abi, Name: name, DeclPos: pos, DefPos: pos, Flags: 0}
}

func IsValidABI(abi string) bool {
	switch abi {
	case CABI:
	case DGABI:
	default:
		return false
	}
	return true
}

func (s SymbolID) String() string {
	switch s {
	case ValSymbol:
		return "val"
	case ConstSymbol:
		return "const"
	case FuncSymbol:
		return "fun"
	case ModuleSymbol:
		return "module"
	case StructSymbol:
		return "struct"
	case TypeSymbol:
		return "type"
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

func (s *Symbol) IsDefined() bool {
	return (s.Flags & SymFlagDefined) != 0
}

func (s *Symbol) IsUntyped() bool {
	if s.T == nil || IsUntyped(s.T) {
		return true
	}
	return false
}

func (s *Symbol) IsType() bool {
	switch s.ID {
	case StructSymbol:
		return true
	case TypeSymbol:
		return true
	}
	return false
}

func (s *Symbol) ModFQN() string {
	return s.Parent.FQN
}

func (s *Symbol) FQN() string {
	if s.ID == ModuleSymbol {
		return s.ModFQN()
	}
	return fmt.Sprintf("%s.%s", s.ModFQN(), s.Name)
}
