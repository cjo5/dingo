package llvm

import (
	"github.com/jhnl/interpreter/semantics"
	"llvm.org/llvm/bindings/go/llvm"
)

type compiler struct {
	semantics.BaseVisitor
	b llvm.Builder
}

func Compile(prog *semantics.Program) {
	c := &compiler{b: llvm.NewBuilder()}
	c.doStuff()
}

func (c *compiler) doStuff() {

}
