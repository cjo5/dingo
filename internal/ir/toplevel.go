package ir

import (
	"github.com/cjo5/dingo/internal/token"
)

const ParentModName = "up"

type FileMatrix []FileList
type FileList []*File

type File struct {
	Filename     string
	ParentIndex1 int
	ParentIndex2 int
	Comments     []*Comment
	Modules      []*IncompleteModule
}

type Comment struct {
	Tok     token.Token
	Pos     token.Position
	Literal string
}

type IncompleteModule struct {
	ParentIndex int
	Name        *Ident
	Visibility  token.Token
	Includes    []*BasicLit
	Decls       []*TopDecl
}

type TopDecl struct {
	D          Decl
	ABI        *Ident
	Visibility token.Token
}

func (d *TopDecl) declNode() {}

func (d *TopDecl) Symbol() *Symbol {
	return d.D.Symbol()
}

func NewTopDecl(abi *Ident, visibility token.Token, decl Decl) *TopDecl {
	return &TopDecl{
		ABI:        abi,
		Visibility: visibility,
		D:          decl,
	}
}

type DeclMatrix []*DeclList

type DeclList struct {
	Filename string
	CUID     int
	Decls    []Decl
	Syms     map[SymbolKey]*Symbol
}
