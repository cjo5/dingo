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
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	ctx := common.NewBuildContext(cwd)

	flag.Usage = func() {
		fmt.Printf("Usage of %s: [options] files\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.StringVar(&ctx.Exe, "exe", "dgexe", "Name of executable")
	flag.BoolVar(&ctx.Verbose, "verbose", false, "Print compilation info")
	flag.BoolVar(&ctx.LLVMIR, "dump-llvm-ir", false, "Print LLVM IR")
	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Printf("%s: no input files\n", common.BoldRed(common.ErrorMsg.String()))
		os.Exit(0)
	}

	build(ctx, flag.Args())
}

func build(ctx *common.BuildContext, filenames []string) {
	if fileMatrix, ok := frontend.Load(ctx, filenames); ok {
		target := backend.NewLLVMTarget()
		if declMatrix, ok := semantics.Check(ctx, target, fileMatrix); ok {
			backend.BuildLLVM(ctx, target, declMatrix)
		}
	}
	ctx.FormatErrors()
	for _, warn := range ctx.Errors.Warnings {
		fmt.Printf("%s\n", warn)
	}
	for _, err := range ctx.Errors.Errors {
		fmt.Printf("%s\n", err)
	}
	if ctx.Errors.IsError() {
		os.Exit(1)
	}
}
