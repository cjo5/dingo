package ir

import "bytes"
import "fmt"

// ScopeID identifies the type of scope.
type ScopeID int

// Scope IDs.
const (
	RootScope ScopeID = iota
	TopScope
	LocalScope
	FieldScope
)

type Scope struct {
	ID      ScopeID
	FQN     string
	Parent  *Scope
	Symbols map[string]*Symbol
}

// NewScope creates a new scope nested in the parent scope.
func NewScope(id ScopeID, fqn string, parent *Scope) *Scope {
	const n = 4 // Initial scope capacity
	return &Scope{id, fqn, parent, make(map[string]*Symbol, n)}
}

func (s ScopeID) String() string {
	switch s {
	case RootScope:
		return "RootScope"
	case TopScope:
		return "TopScope"
	case LocalScope:
		return "LocalScope"
	case FieldScope:
		return "FieldScope"
	default:
		return "Scope " + string(s)
	}
}

func (s *Scope) String() string {
	var buf bytes.Buffer
	idx := 0
	buf.WriteString(fmt.Sprintf("%s %s (%d)\n", s.FQN, s.ID, len(s.Symbols)))
	for k, v := range s.Symbols {
		buf.WriteString(fmt.Sprintf("%s = %s", k, v))
		idx++
		if idx < len(s.Symbols) {
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

func (s *Scope) Insert(sym *Symbol) *Symbol {
	var existing *Symbol
	if existing = s.Symbols[sym.Name]; existing == nil {
		s.Symbols[sym.Name] = sym
	}
	return existing
}

func (s *Scope) Lookup(name string) *Symbol {
	return doLookup(s, name)
}

func doLookup(s *Scope, name string) *Symbol {
	if s == nil {
		return nil
	}
	if res := s.Symbols[name]; res != nil {
		return res
	}
	return doLookup(s.Parent, name)
}
