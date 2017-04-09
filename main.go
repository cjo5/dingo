package main

import (
	"fmt"

	"github.com/jhnl/interpreter/ast"
	"github.com/jhnl/interpreter/parser"
)

func main() {
	src := []byte("-5+2*3")

	tree, _ := parser.Parse(src)
	s := ast.Print(tree)

	fmt.Println(s)
}
