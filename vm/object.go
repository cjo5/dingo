package vm

import (
	"fmt"
	"os"
	"strconv"
)

// InstructionList represents a functions's instructions.
type InstructionList []Instruction

// MemoryRegion represents a data memory region.
type MemoryRegion []interface{}

type BytecodeProgram struct {
	Modules []*ModuleObject
}

type ModuleObject struct {
	Name      string
	Path      string
	Constants MemoryRegion
	Globals   MemoryRegion
	Functions []*FunctionObject
}

type FunctionObject struct {
	ReturnValue bool
	ArgCount    int
	LocalCount  int
	Name        string
	Code        InstructionList
}

type StructObject struct {
	Name   string
	Fields MemoryRegion
}

func (s *StructObject) Get(index int) interface{} {
	return s.Fields[index]
}

func (s *StructObject) Put(index int, value interface{}) {
	s.Fields[index] = value
}

type StructDescriptor struct {
	Name       string
	FieldCount int
}

func Disasm(code *BytecodeProgram, output *os.File) {
	for idx, mod := range code.Modules {
		mod.Disasm(idx, output)
		output.WriteString("\n")
	}
}

func (m *ModuleObject) Disasm(address int, output *os.File) {
	output.WriteString(fmt.Sprintf("%04x: %s\n", address, m))
	output.WriteString(fmt.Sprintf("constants (%d)\n", len(m.Constants)))
	writeMemory(m.Constants, output)
	output.WriteString(fmt.Sprintf("globals (%d)\n", len(m.Globals)))
	writeMemory(m.Globals, output)
	output.WriteString(fmt.Sprintf("functions (%d)\n", len(m.Functions)))
	for idx, f := range m.Functions {
		f.Disasm(idx, output)
	}
}

func (f *FunctionObject) Disasm(address int, output *os.File) {
	output.WriteString(fmt.Sprintf("%04x: %s\n", address, f))
	writeCode(f.Code, output)
}

func writeMemory(mem MemoryRegion, output *os.File) {
	for i, c := range mem {
		s := ""
		if tmp, ok := c.(string); ok {
			s = strconv.Quote(tmp)
		} else {
			s = fmt.Sprintf("%v", c)
		}
		output.WriteString(fmt.Sprintf("%04x: %s <%T>\n", i, s, c))
	}
}

func writeCode(code InstructionList, output *os.File) {
	for i, c := range code {
		output.WriteString(fmt.Sprintf("%04x: %s\n", i, c))
	}
}

func (m *ModuleObject) String() string {
	return fmt.Sprintf(".module(name=%s, path=\"%s\")", m.Name, m.Path)
}

func (f *FunctionObject) String() string {
	return fmt.Sprintf(".function(name=%s, instructionCount=%d, argCount=%d, returnValue=%v, localCount=%d)", f.Name, len(f.Code), f.ArgCount, f.ReturnValue, f.LocalCount)
}

func (s *StructObject) String() string {
	return fmt.Sprintf(".struct(name=%s, fieldCount=%d)", s.Name, len(s.Fields))
}

func (s *StructDescriptor) String() string {
	return fmt.Sprintf(".structdescriptor(name=%s, fieldCount=%d)", s.Name, s.FieldCount)
}
