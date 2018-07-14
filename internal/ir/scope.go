package ir

import "bytes"
import "fmt"

// ScopeKind identifies the kind of scope.
type ScopeKind int

// Scope kinds.
const (
	BuiltinScope ScopeKind = iota
	ModuleScope
	LocalScope
	FieldScope
)

type Scope struct {
	Kind    ScopeKind
	Parent  *Scope
	CUID    int
	Defer   bool
	Symbols map[string]*Symbol
}

// NewScope creates a new scope nested in the parent scope.
func NewScope(kind ScopeKind, parent *Scope, CUID int) *Scope {
	return &Scope{
		Parent:  parent,
		CUID:    CUID,
		Symbols: make(map[string]*Symbol),
	}
}

func (s ScopeKind) String() string {
	switch s {
	case BuiltinScope:
		return "BuiltinScope"
	case ModuleScope:
		return "ModuleScope"
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
	buf.WriteString(fmt.Sprintf("%s (%d)\n", s.Kind, len(s.Symbols)))
	for k, v := range s.Symbols {
		buf.WriteString(fmt.Sprintf("%s = %s", k, v))
		idx++
		if idx < len(s.Symbols) {
			buf.WriteString("\n")
		}
	}
	return buf.String()
}

func (s *Scope) Insert(alias string, sym *Symbol) *Symbol {
	var existing *Symbol
	if existing = s.Symbols[alias]; existing == nil {
		s.Symbols[alias] = sym
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
