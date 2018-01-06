package main

import (
	"fmt"

	"github.com/jhnl/dingo/ir"
	"github.com/jhnl/dingo/token"

	"flag"
	"os"

	"github.com/jhnl/dingo/backend"
	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/module"
	"github.com/jhnl/dingo/semantics"
)

func main() {
	env := &common.BuildEnvironment{}
	env.Debug = true

	flag.Usage = func() {
		fmt.Printf("Usage of %s: [options] file\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.StringVar(&env.Exe, "exe", "dgexe", "Name of executable")
	flag.BoolVar(&env.Clean, "clean", true, "Clean up object files after build has finished")
	flag.Parse()

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	build(os.Args[1:2], env)
}

func build(files []string, env *common.BuildEnvironment) {
	errors := &common.ErrorList{}
	set := &ir.ModuleSet{}

	for _, file := range files {
		mod, err := module.Load(file)
		addError(err, errors)
		if mod != nil {
			set.Modules = append(set.Modules, mod)
		}
	}

	if errors.Count() > 0 {
		printErrors(errors)
		return
	}

	if env.Debug {
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
		printErrors(errors)
		return
	}

	err = backend.Build(set, env)
	addError(err, errors)
	printErrors(errors)
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

func printErrors(errList *common.ErrorList) {
	errList.Sort()
	for _, err := range errList.Errors {
		fmt.Printf("%s\n", err)
	}
}
