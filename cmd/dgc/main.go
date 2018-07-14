package main

import (
	"fmt"
	"os"

	"flag"

	"github.com/jhnl/dingo/internal/backend"
	"github.com/jhnl/dingo/internal/common"
	"github.com/jhnl/dingo/internal/frontend"
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
	fileList1, err := frontend.Load(filenames)
	addError(err, errors)

	if errors.IsError() {
		return
	}

	target := backend.NewLLVMTarget()

	cunitSet, err := semantics.Check(fileList1, target)
	addError(err, errors)

	if errors.IsError() {
		return
	}

	err = backend.BuildLLVM(cunitSet, target, config)
	addError(err, errors)
}

func addError(newError error, errors *common.ErrorList) {
	if newError == nil {
		return
	}
	errors.AddGeneric1(newError)
}
