package main

import (
	"fmt"
	"os"

	"flag"

	"github.com/jhnl/dingo/internal/backend"
	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/module"
	"github.com/jhnl/dingo/internal/semantics"
)

func main() {
	config := common.NewBuildConfig()

	flag.Usage = func() {
		fmt.Printf("Usage of %s: [options] file\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.StringVar(&config.Exe, "exe", "dgexe", "Name of executable")
	flag.BoolVar(&config.Verbose, "verbose", false, "Print compilation info")
	flag.BoolVar(&config.LLVMIR, "dump-llvm-ir", false, "Print LLVM IR")
	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Printf("%s: no input files\n", common.BoldRed(common.ErrorMsg.String()))
		os.Exit(0)
	}

	errors := &common.ErrorList{}
	build(flag.Args(), config, errors)
	printErrors(errors)
}

func printErrors(errors *common.ErrorList) {
	errors.Sort()
	errors.LoadContext()

	for _, warn := range errors.Warnings {
		fmt.Printf("%s\n", warn)
	}

	for _, err := range errors.Errors {
		fmt.Printf("%s\n", err)
	}

	if errors.IsError() {
		os.Exit(1)
	}
}

func build(filenames []string, config *common.BuildConfig, errors *common.ErrorList) {
	set, err := module.Load(filenames)
	addError(err, errors)

	if errors.IsError() {
		return
	}

	if config.Verbose {
		for _, mod := range set.Modules {
			fmt.Println("Module", mod.FQN)
			for _, file := range mod.Files {
				fmt.Println("  File", file.Filename)
				for _, dep := range file.FileDeps {
					fmt.Println("    Include", dep.Literal)
				}
			}
		}
	}

	target := backend.NewLLVMTarget()

	err = semantics.Check(set, target)
	addError(err, errors)

	if errors.IsError() {
		return
	}

	err = backend.BuildLLVM(set, target, config)
	addError(err, errors)
}

func addError(newError error, errors *common.ErrorList) {
	if newError == nil {
		return
	}
	errors.AddGeneric1(newError)
}
