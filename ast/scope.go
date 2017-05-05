package ast

import (
	"fmt"

	"github.com/jhnl/interpreter/token"
)

type SymbolID int

const (
	Invalid = iota
	VarSymbol
	FuncSymbol
)

type Scope struct {
	Outer   *Scope
	Symbols map[string]*Symbol
}

type Symbol struct {
	ID   SymbolID
	Name token.Token
	Decl Stmt
}

// NewScope creates a new scope nested in the outer scope.
func NewScope(outer *Scope) *Scope {
	const n = 4 // initial scope capacity
	return &Scope{outer, make(map[string]*Symbol, n)}
}

// NewSymbol creates a new symbol of a given ID and name.
func NewSymbol(id SymbolID, name token.Token, decl Stmt) *Symbol {
	return &Symbol{ID: id, Name: name, Decl: decl}
}

func (s *Scope) Insert(sym *Symbol) *Symbol {
	var existing *Symbol
	if existing = s.Symbols[sym.Name.Literal]; existing == nil {
		s.Symbols[sym.Name.Literal] = sym
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
	return doLookup(s.Outer, name)
}

func (s *Symbol) Pos() string {
	return fmt.Sprintf("%d:%d", s.Name.Line, s.Name.Column)
}
