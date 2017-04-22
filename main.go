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

	loopVarAddress := 0
	iterCount := 9

	code = append(code, vm.NewInstr1(vm.ILOAD, 0))
	code = append(code, vm.NewInstr1(vm.GSTORE, loopVarAddress))
	code = append(code, vm.NewInstr1(vm.GOTO, 11))
	code = append(code, vm.NewInstr1(vm.GLOAD, loopVarAddress)) // Address of loop_start
	code = append(code, vm.NewInstr0(vm.PRINT))
	code = append(code, vm.NewInstr1(vm.CLOAD, 0))
	code = append(code, vm.NewInstr0(vm.PRINT))
	code = append(code, vm.NewInstr1(vm.GLOAD, loopVarAddress))
	code = append(code, vm.NewInstr1(vm.ILOAD, 1))
	code = append(code, vm.NewInstr0(vm.BINARY_ADD))
	code = append(code, vm.NewInstr1(vm.GSTORE, loopVarAddress))
	code = append(code, vm.NewInstr1(vm.GLOAD, loopVarAddress)) // Address of loop_end
	code = append(code, vm.NewInstr1(vm.ILOAD, iterCount))
	code = append(code, vm.NewInstr1(vm.CMP_LT, 3))

	mem.Globals = make([]interface{}, 2)
	mem.Constants = append(mem.Constants, "\n")

	vm := vm.NewMachine()

	vm.Disasm(code, mem)
	vm.Exec(code, mem)

	if vm.RuntimeError() {
		vm.PrintTrace()
	}
}

func main() {
	//testParse()
	testVM()
}
