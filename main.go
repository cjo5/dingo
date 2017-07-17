package main

import (
	"fmt"

	"os"

	"github.com/jhnl/interpreter/common"
	"github.com/jhnl/interpreter/module"
	"github.com/jhnl/interpreter/semantics"
	"github.com/jhnl/interpreter/vm"
)

func showErrors(oldErrors common.ErrorList, newError error, onlyFatal bool) bool {
	if newError == nil {
		return false
	}
	if errList, ok := newError.(*common.ErrorList); ok {
		oldErrors.Merge(errList)
		if len(errList.Errors) == 0 || (onlyFatal && !oldErrors.IsFatal()) {
			return false
		}
		errList.Sort()
		errList.Filter()
		for _, e := range errList.Errors {
			fmt.Printf("%s\n", e)
		}
	} else {
		fmt.Printf("[error] %s\n", newError)
	}
	return true
}

func exec(path string) {
	var errors common.ErrorList
	prog, err := module.Load(path)

	if showErrors(errors, err, false) {
		return
	}

	for _, mod := range prog.Modules {
		fmt.Println("Module", mod.Name.Literal)
		for _, file := range mod.Files {
			fmt.Println("  File", file.Ctx.Path)
			for _, imp := range file.Imports {
				fmt.Println("    Import", imp.Literal.Literal)
			}
		}
	}

	fmt.Println("Parse done")
	fmt.Println(semantics.Print(prog))

	err = semantics.Check(prog)
	if showErrors(errors, err, false) {
		return
	}

	fmt.Println("Check done")
	fmt.Println(semantics.Print(prog))

	bytecode := vm.Compile(prog)
	vm.Disasm(bytecode, os.Stdout)
	vm.Run(bytecode, os.Stdout)
}

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("failed to get working directory:", err)
		return
	}
	common.Cwd = dir

	exec("examples/modules/a.lang")
}
