package main

import (
	"fmt"

	"github.com/jhnl/interpreter/ast"
	"github.com/jhnl/interpreter/parser"
	"github.com/jhnl/interpreter/scanner"
)

func main() {
	//src := []byte("-6*2+2;")

	tree, err := parser.ParseFile("scripts/test1.nerd")
	if err != nil {
		printed := false
		if errList, ok := err.(scanner.ErrorList); ok {
			if len(errList) > 1 {
				fmt.Println("Errors:")
				for idx, e := range errList {
					fmt.Println(fmt.Sprintf("[%d] %s", idx, e))
				}
				printed = true
			}
		}

		if !printed {
			fmt.Println("Error:", err)
		}

		return
	}

	s := ast.Print(tree)
	fmt.Println(s)
}
