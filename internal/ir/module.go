package ir

import (
	"github.com/jhnl/dingo/internal/token"
)

// A ModuleSet is a collection of modules that make up the program.
type ModuleSet struct {
	Modules map[string]*Module
}

func NewModuleSet() *ModuleSet {
	set := &ModuleSet{}
	set.Modules = make(map[string]*Module)
	return set
}

func (m *ModuleSet) Pos() token.Position { return token.NoPosition }

func (m *ModuleSet) FindModule(fqn string) *Module {
	return m.Modules[fqn]
}

func (m *ModuleSet) ResetDeclColors() {
	for _, mod := range m.Modules {
		mod.ResetDeclColors()
	}
}

// A Module is a collection of files sharing the same namespace.
type Module struct {
	Path  token.Position
	FQN   string
	Scope *Scope
	Files []*File
	Decls []TopDecl
}

func (m *Module) Pos() token.Position { return m.Path }

func (m *Module) FindFileWithFQN(fqn string) *File {
	for _, file := range m.Files {
		fileFQN := ExprNameToText(file.ModName)
		if fileFQN == fqn {
			return file
		}
	}
	return nil
}

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
