package token

import (
	"bytes"
	"fmt"
	"path/filepath"
)

type File struct {
	Filename string
	Src      []byte
}

// Position of token in a file.
type Position struct {
	Filename string
	Offset   int
	Line     int
	Column   int
}

func Abs(cwd string, filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join(cwd, filename)
}

func NewPosition1(filename string) Position {
	return Position{Filename: filename}
}

// NoPosition means it wasn't part of a file.
var NoPosition = Position{}

func (p Position) String() string {
	var buf bytes.Buffer
	if len(p.Filename) > 0 {
		buf.WriteString(p.Filename)
	}

	if p.Line > 0 {
		if buf.Len() > 0 {
			buf.WriteString(":")
		}
		buf.WriteString(fmt.Sprintf("%d:%d", p.Line, p.Column))
	}

	if buf.Len() > 0 {
		return buf.String()
	}

	return "-"
}

// IsValid returns true if it's a valid file position.
func (p Position) IsValid() bool {
	return p.Line > 0
}
