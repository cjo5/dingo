package ir

import (
	"github.com/jhnl/dingo/token"
)

// A ModuleSet is a collection of modules that make up the program.
type ModuleSet struct {
	Modules []*Module
}

func (m *ModuleSet) Pos() token.Position { return token.NoPosition }

func (m *ModuleSet) FindModule(fqn string) *Module {
	for _, mod := range m.Modules {
		if mod.FQN == fqn {
			return mod
		}
	}
	return nil
}

func (m *ModuleSet) ResetDeclColors() {
	for _, mod := range m.Modules {
		mod.ResetDeclColors()
	}
}

// A Module is a collection of files sharing the same namespace.
type Module struct {
	ID    int
	Path  string // To root file
	FQN   string
	Scope *Scope
	Files []*File
	Decls []TopDecl
}

func (m *Module) Pos() token.Position { return token.NoPosition }

func (m *Module) FindFuncSymbol(name string) *Symbol {
	for _, decl := range m.Decls {
		sym := decl.Symbol()
		if sym != nil && sym.ID == FuncSymbol {
			if sym.Name == name {
				return sym
			}
		}
	}
	return nil
}

func (m *Module) ResetDeclColors() {
	for _, decl := range m.Decls {
		decl.SetColor(WhiteColor)
	}
}
