package semantics

type TypeID int

// Type IDs
const (
	TUntyped TypeID = iota
	TVoid
	TBool
	TString
	TUInt32
	TInt32
)

var types = [...]string{
	TUntyped: "untyped",
	TVoid:    "void",
	TBool:    "bool",
	TString:  "str",
	TUInt32:  "u32",
	TInt32:   "s32",
}

// Built-in types
var (
	TBuiltinUntyped = NewType(TUntyped)
	TBuiltinVoid    = NewType(TVoid)
	TBuiltinBool    = NewType(TBool)
	TBuiltinString  = NewType(TString)
	TBuiltinUInt32  = NewType(TUInt32)
	TBuiltinInt32   = NewType(TInt32)
)

type TType struct {
	ID TypeID
}

func NewType(id TypeID) *TType {
	return &TType{ID: id}
}

func (t *TType) String() string {
	return t.ID.String()
}

func (id TypeID) String() string {
	s := ""
	if 0 <= id && id < TypeID(len(types)) {
		s = types[id]
	} else {
		s = "unknown"
	}
	return s
}
