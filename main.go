package main

import (
	"fmt"
	"os"

	"github.com/jhnl/interpreter/codegen"
	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/parser"
	"github.com/jhnl/interpreter/semantics"
	"github.com/jhnl/interpreter/vm"
)

func exec(filename string) {
	mod, err := parser.ParseFile(filename)

	if err == nil {
		err = semantics.Check(mod)
	}

	if err != nil {
		if errList, ok := err.(common.ErrorList); ok {
			for idx, e := range errList {
				fmt.Println(fmt.Sprintf("[%d] %s", idx, e))
			}
		}
		return
	}

	fmt.Println(semantics.Print(mod))
	ip, code, mem := codegen.Compile(mod)

	fmt.Printf("Constants (%d):\n", len(mem.Constants))
	vm.DumpMemory(mem.Constants, os.Stdout)
	fmt.Printf("Globals (%d):\n", len(mem.Globals))
	vm.DumpMemory(mem.Globals, os.Stdout)
	fmt.Printf("\nCode (%d):\n", len(code))
	vm.Disasm(code, os.Stdout)
	fmt.Println()

	machine := vm.NewMachine(os.Stdout)
	machine.Exec(ip, code, mem)
	if machine.RuntimeError() {
		fmt.Println("Runtime error:", machine.Err)
	}
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
	vm.DumpMemory(mem.Constants, os.Stdout)
	fmt.Println("\nCode")
	vm.Disasm(code, os.Stdout)
	fmt.Println()

	machine.Exec(0, code, mem)
	if machine.RuntimeError() {
		fmt.Println("Runtime error:", machine.Err)
	}
}

func main() {
	exec("examples/test4.lang")
	//testVM()
}
