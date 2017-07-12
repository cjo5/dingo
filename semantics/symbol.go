package semantics

import "github.com/jhnl/interpreter/token"
import "fmt"

type SymbolID int

const (
	ValSymbol SymbolID = iota
	FuncSymbol
	TypeSymbol
	ModuleSymbol
)

// Symbol flags.
const (
	SymFlagDepCycle = 1 << 1
	SymFlagToplevel = 1 << 2
	SymFlagConstant = 1 << 3
)

type Symbol struct {
	ID      SymbolID
	Name    string
	Pos     token.Position
	T       *TType
	Src     Decl // Node in ast where the symbol was defined
	Flags   int
	Address int
}

// NewSymbol creates a new symbol of a given ID and name.
func NewSymbol(id SymbolID, name string, pos token.Position, src Decl, toplevel bool) *Symbol {
	flags := 0
	if toplevel {
		flags = SymFlagToplevel
	}
	return &Symbol{ID: id, Name: name, Pos: pos, Src: src, Flags: flags}
}

func (s SymbolID) String() string {
	switch s {
	case ValSymbol:
		return "ValSymbol"
	case FuncSymbol:
		return "FuncSymbol"
	case TypeSymbol:
		return "TypeSymbol"
	case ModuleSymbol:
		return "ModuleSymbol"
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

func (s *Symbol) Toplevel() bool {
	return (s.Flags & SymFlagToplevel) != 0
}

func (s *Symbol) Constant() bool {
	return (s.Flags & SymFlagConstant) != 0
}
