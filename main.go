package main

import (
	"fmt"
	"os"

	"github.com/jhnl/interpreter/ast"
	"github.com/jhnl/interpreter/parser"
	"github.com/jhnl/interpreter/scanner"
	"github.com/jhnl/interpreter/vm"
)

func testParse() {
	//src := []byte("-6*2+2;")

	tree, err := parser.ParseFile("examples/test1.lang")
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

	code = append(code, vm.NewInstr1(vm.Iload, 0))
	code = append(code, vm.NewInstr1(vm.Gstore, loopVarAddress))
	code = append(code, vm.NewInstr1(vm.Goto, 11))
	code = append(code, vm.NewInstr1(vm.Gload, loopVarAddress)) // Address of loop_start
	code = append(code, vm.NewInstr0(vm.Print))
	code = append(code, vm.NewInstr1(vm.Cload, 0))
	code = append(code, vm.NewInstr0(vm.Print))
	code = append(code, vm.NewInstr1(vm.Gload, loopVarAddress))
	code = append(code, vm.NewInstr1(vm.Iload, 1))
	code = append(code, vm.NewInstr0(vm.BinaryAdd))
	code = append(code, vm.NewInstr1(vm.Gstore, loopVarAddress))
	code = append(code, vm.NewInstr1(vm.Gload, loopVarAddress)) // Address of loop_end
	code = append(code, vm.NewInstr1(vm.Iload, iterCount))
	code = append(code, vm.NewInstr1(vm.CmpLt, 3))

	mem.Globals = make([]interface{}, 2)
	mem.Constants = append(mem.Constants, "\n")

	machine := vm.NewMachine(os.Stdout)

	fmt.Println("Constants")
	vm.DumpMemory(mem, os.Stdout)
	fmt.Println("\nCode")
	vm.Disasm(code, os.Stdout)
	fmt.Println()

	machine.Exec(code, mem)
	if machine.RuntimeError() {
		fmt.Println("Runtime error:", machine.Err)
	}
}

func main() {
	testParse()
	//testVM()
}
