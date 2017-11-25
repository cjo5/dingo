package main

import (
	"fmt"

	"flag"
	"os"

	"github.com/jhnl/dingo/common"
	"github.com/jhnl/dingo/llvm"
	"github.com/jhnl/dingo/module"
	"github.com/jhnl/dingo/semantics"
)

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage of %s: [options] file\n", os.Args[0])
		flag.PrintDefaults()
	}

	out := flag.String("exe", "", "Name of executable")
	flag.Parse()

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	driver := newDriver()
	driver.infile = os.Args[1]
	driver.outfile = *out

	driver.build()
}

type driver struct {
	infile  string
	outfile string
}

func newDriver() *driver {
	return &driver{}
}

func (d *driver) build() {
	var errors common.ErrorList

	modules, err := module.Load(d.infile)
	if printErrors(errors, err, false) {
		return
	}

	for _, mod := range modules.Modules {
		fmt.Println("Module", mod.Name.Literal)
		for _, file := range mod.Files {
			fmt.Println("  File", file.Ctx.Path)
			for _, inc := range file.Includes {
				fmt.Println("    Include", inc.Literal.Literal)
			}
		}
	}

	//fmt.Println("Parse done")
	//fmt.Println(semantics.PrintTree(modules))

	err = semantics.Check(modules)
	if printErrors(errors, err, false) {
		return
	}

	//fmt.Println("Check done")
	//fmt.Println(semantics.PrintTree(modules))

	llvm.Build(modules, d.outfile)
}

func printErrors(oldErrors common.ErrorList, newError error, onlyFatal bool) bool {
	if newError == nil {
		return false
	}

	if errList, ok := newError.(*common.ErrorList); ok {
		oldErrors.Merge(errList)
		if len(errList.Errors) == 0 || (onlyFatal && !oldErrors.IsFatal()) {
			return false
		}
		errList.Sort()
		for _, e := range errList.Errors {
			fmt.Printf("%s\n", e)
		}
	} else {
		fmt.Printf("%s\n", newError)
	}

	return true
}
