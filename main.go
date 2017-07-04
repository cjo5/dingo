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

	fmt.Println(semantics.Print(mod))
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

func main() {
	exec("examples/test2.lang")
	//testVM()
}
