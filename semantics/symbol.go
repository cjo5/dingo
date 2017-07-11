package semantics

import "github.com/jhnl/interpreter/token"
import "fmt"

type SymbolID int

const (
	VarSymbol SymbolID = iota
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
	File    *FileInfo // File where symbol was defined
	T       *TType
	Src     Decl // Node in ast where the symbol was defined
	Flags   int
	Address int

	dependencies []*Symbol
	color        GraphColor
}

// NewSymbol creates a new symbol of a given ID and name.
func NewSymbol(id SymbolID, name string, pos token.Position, file *FileInfo, src Decl, toplevel bool) *Symbol {
	flags := 0
	if toplevel {
		flags = SymFlagToplevel
	}
	return &Symbol{ID: id, Name: name, Pos: pos, File: file, Src: src, Flags: flags, color: GraphColorWhite}
}

func (s SymbolID) String() string {
	switch s {
	case VarSymbol:
		return "VarSymbol"
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
	path := ""
	if s.File != nil {
		path = s.File.Path
	}
	return fmt.Sprintf("%s:%s:%s", path, s.ID, s.Name)
}

func (s *Symbol) Func() (*FuncDecl, bool) {
	decl, ok := s.Src.(*FuncDecl)
	return decl, ok
}

func (s *Symbol) Var() (*VarDecl, bool) {
	decl, ok := s.Src.(*VarDecl)
	return decl, ok
}

func (s *Symbol) Toplevel() bool {
	return (s.Flags & SymFlagToplevel) != 0
}

func (s *Symbol) Constant() bool {
	return (s.Flags & SymFlagConstant) != 0
}
