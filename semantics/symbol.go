package semantics

import "github.com/jhnl/interpreter/token"
import "fmt"

type SymbolID int

const (
	ValSymbol SymbolID = iota
	FuncSymbol
	BuiltinSymbol
	ModuleSymbol
	StructSymbol
)

// Symbol flags.
const (
	SymFlagDepCycle = 1 << 0
	SymFlagConstant = 1 << 1
	SymFlagType     = 1 << 2
	SymFlagCastable = 1 << 3
)

type Symbol struct {
	ID       SymbolID
	ModuleID int
	Name     string
	Pos      token.Position
	T        *TType
	Src      Decl // Node in ast where the symbol was defined
	Flags    int
	Address  int
}

// NewSymbol creates a new symbol of a given ID and name.
func NewSymbol(id SymbolID, moduleID int, name string, pos token.Position, src Decl) *Symbol {
	return &Symbol{ID: id, ModuleID: moduleID, Name: name, Pos: pos, Src: src, Flags: 0}
}

func (s SymbolID) String() string {
	switch s {
	case ValSymbol:
		return "ValSymbol"
	case FuncSymbol:
		return "FuncSymbol"
	case BuiltinSymbol:
		return "BuiltinSymbol"
	case ModuleSymbol:
		return "ModuleSymbol"
	case StructSymbol:
		return "StructSymbol"
	default:
		return "Symbol " + string(s)
	}
}

func (s *Symbol) String() string {
	return fmt.Sprintf("%s:%s:%s", s.ID, s.Pos, s.Name)
}

func (s *Symbol) Func() (*FuncDecl, bool) {
	decl, ok := s.Src.(*FuncDecl)
	return decl, ok
}

func (s *Symbol) Constant() bool {
	return (s.Flags & SymFlagConstant) != 0
}

func (s *Symbol) Type() bool {
	return (s.Flags & SymFlagType) != 0
}

func (s *Symbol) Castable() bool {
	return (s.Flags & SymFlagCastable) != 0
}
