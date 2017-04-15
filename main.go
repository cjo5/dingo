package main

import (
	"fmt"

	"github.com/jhnl/interpreter/ast"
	"github.com/jhnl/interpreter/parser"
)

func main() {
	//src := []byte("-6*2+2;")

	tree, err := parser.ParseFile("scripts/test1.nerd")
	if err != nil {
		fmt.Println(err)
		return
	}

	s := ast.Print(tree)
	fmt.Println(s)
}
