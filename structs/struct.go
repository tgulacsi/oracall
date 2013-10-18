package structs

var Functions = make(map[string]*Function)

type Function struct {
	Owner, Package, Name string
	Args                 []Argument
}

func (f Function) String() string {
	return f.Owner + "." + f.Package + "." + f.Name
}

const (
	IN  = 0
	OUT = 1
)

type Argument struct {
	Name                    string
	Level                   uint8
	Position                uint8
	InOut                   uint8
	Type, PlsType, TypeName string
	Precision               uint8
	Scale                   uint8
	Charset                 string
	Charlength              uint
}
