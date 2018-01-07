package main

import (
	"fmt"
	"os"

	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"

	"flag"

	"github.com/jhnl/dingo/backend"
	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/module"
	"github.com/jhnl/dingo/semantics"
)

func main() {
	env := &common.BuildEnvironment{}

	flag.Usage = func() {
		fmt.Printf("Usage of %s: [options] file\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.StringVar(&env.Exe, "exe", "dgexe", "Name of executable")
	flag.BoolVar(&env.Verbose, "verbose", false, "Print compilation info")
	flag.BoolVar(&env.LLVMIR, "dump-llvm-ir", false, "Print LLVM IR")
	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Printf("error: no input files\n")
		os.Exit(0)
	}

	errors := &common.ErrorList{}
	build(flag.Args()[0:1], env, errors)

	errors.Sort()
	for _, err := range errors.Errors {
		fmt.Printf("%s\n", err)
	}
}

func build(files []string, env *common.BuildEnvironment, errors *common.ErrorList) {
	set := &ir.ModuleSet{}

	for _, file := range files {
		mod, err := module.Load(file)
		addError(err, errors)
		if mod != nil {
			set.Modules = append(set.Modules, mod)
		}
	}

	if errors.Count() > 0 {
		return
	}

	if env.Verbose {
		for _, mod := range set.Modules {
			fmt.Println("Module", mod.FQN)
			for _, file := range mod.Files {
				fmt.Println("  File", file.Ctx.Path)
				for _, dep := range file.Deps {
					fmt.Println("    Include", dep.Literal.Literal)
				}
			}
		}
	}

	err := semantics.Check(set)
	addError(err, errors)
	if errors.Count() > 0 {
		return
	}

	err = backend.Build(set, env)
	addError(err, errors)
}

func addError(newError error, errList *common.ErrorList) {
	if newError == nil {
		return
	}

	switch t := newError.(type) {
	case *common.ErrorList:
		errList.Merge(t)
	case *common.Error:
		errList.Errors = append(errList.Errors, t)
	default:
		errList.AddGeneric("", token.NoPosition, newError)
	}
}
