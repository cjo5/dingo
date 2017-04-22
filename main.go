package main

import (
	"fmt"

	"github.com/jhnl/interpreter/ast"
	"github.com/jhnl/interpreter/parser"
	"github.com/jhnl/interpreter/scanner"
	"github.com/jhnl/interpreter/vm"
)

func testParse() {
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

func testVM() {
	var code vm.CodeMemory
	var mem vm.DataMemory

	code = append(code, vm.NewInstr1(vm.CPUSH, 1))
	code = append(code, vm.NewInstr0(vm.PRINT))
	code = append(code, vm.NewInstr1(vm.CPUSH, 0))
	code = append(code, vm.NewInstr0(vm.PRINT))

	code = append(code, vm.NewInstr1(vm.IPUSH, 5))
	code = append(code, vm.NewInstr1(vm.IPUSH, 2))
	code = append(code, vm.NewInstr0(vm.BINARY_SUB))
	code = append(code, vm.NewInstr0(vm.PRINT))
	code = append(code, vm.NewInstr1(vm.CPUSH, 0))
	code = append(code, vm.NewInstr0(vm.PRINT))

	mem.Constants = append(mem.Constants, "\n")
	mem.Constants = append(mem.Constants, "hello world")

	vm := vm.NewMachine()
	vm.Exec(code, mem)

	if vm.RuntimeError() {
		vm.PrintTrace()
	}
}

func main() {
	//testParse()
	testVM()
}
