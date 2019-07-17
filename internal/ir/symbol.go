package ir

import "github.com/cjo5/dingo/internal/token"
import "fmt"

// CABI name.
const CABI = "c"

// DGABI name.
const DGABI = "dg"

// SymbolKind identifies the kind of symbol.
type SymbolKind int

// Symbol kinds.
const (
	ValSymbol SymbolKind = iota
	FuncSymbol
	ModuleSymbol
	TypeSymbol
	ImportSymbol
)

// Symbol flags.
const (
	SymFlagReadOnly = 1 << 1
	SymFlagConst    = 1 << 2
	SymFlagDefined  = 1 << 3
	SymFlagTopDecl  = 1 << 4
	SymFlagBuiltin  = 1 << 5
	SymFlagMethod   = 1 << 6
)

// Symbol represents any kind of symbol/identifier in the source code.
type Symbol struct {
	Kind   SymbolKind
	Parent *Scope
	CUID   int
	Key    int
	Public bool
	ABI    string
	ModFQN string
	Name   string
	Pos    token.Position
	T      Type
	Flags  int
}

// NewSymbol creates a new symbol.
func NewSymbol(kind SymbolKind, parent *Scope, CUID int, modFQN string, name string, pos token.Position) *Symbol {
	return &Symbol{
		Kind:   kind,
		Parent: parent,
		CUID:   CUID,
		ABI:    DGABI,
		ModFQN: modFQN,
		Name:   name,
		Pos:    pos,
		T:      TBuiltinUnknown,
	}
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

func (s SymbolKind) String() string {
	switch s {
	case ValSymbol:
		return "val"
	case FuncSymbol:
		return "fun"
	case ModuleSymbol:
		return "module"
	case TypeSymbol:
		return "type"
	case ImportSymbol:
		return "import"
	default:
		return "symbol " + string(s)
	}
}

func (s *Symbol) String() string {
	return fmt.Sprintf("%s:%s:%s", s.Kind, s.Pos, s.Name)
}

func (s *Symbol) IsReadOnly() bool {
	return (s.Flags & SymFlagReadOnly) != 0
}

func (s *Symbol) IsConst() bool {
	return (s.Flags & SymFlagConst) != 0
}

func (s *Symbol) IsDefined() bool {
	return (s.Flags & SymFlagDefined) != 0
}

func (s *Symbol) IsTopDecl() bool {
	return (s.Flags & SymFlagTopDecl) != 0
}

func (s *Symbol) IsBuiltin() bool {
	return (s.Flags & SymFlagBuiltin) != 0
}

func (s *Symbol) IsMethod() bool {
	return (s.Flags & SymFlagMethod) != 0
}

func (s *Symbol) FQN() string {
	if s.Kind == ModuleSymbol {
		return s.ModFQN
	}
	if len(s.ModFQN) > 0 {
		return fmt.Sprintf("%s.%s", s.ModFQN, s.Name)
	}
	return s.Name
}

func (s *Symbol) ParentCUID() int {
	return s.Parent.CUID
}
