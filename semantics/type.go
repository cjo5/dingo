package semantics

type TypeID int

const (
	TInvalid TypeID = iota
	TVoid
	TBool
	TString
	TUInt32
	TInt32
)

var types = [...]string{
	TInvalid: "invalidType",
	TVoid:    "void",
	TBool:    "bool",
	TString:  "str",
	TUInt32:  "u32",
	TInt32:   "s32",
}

// TODO: Create const pointers for built-in types

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
