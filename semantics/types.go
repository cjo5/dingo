package semantics

type TypeID int

const (
	UInt32 TypeID = iota
	Int32
	Real32
	String
	Char
	Bool
)

var builtinTypes = map[string]TypeID{
	"u32":    UInt32,
	"i32":    Int32,
	"r32":    Real32,
	"string": String,
	"char":   Char,
}
