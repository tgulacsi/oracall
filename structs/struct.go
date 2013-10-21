package structs

import (
	"fmt"
	"strings"
)

type Function struct {
	Package, name string
	Args          []Argument
}

func (f Function) Name() string {
	if f.Package == "" {
		return strings.ToLower(f.name)
	}
	return unocap(f.Package) + "." + strings.ToLower(f.name)
}

func (f Function) String() string {
	return f.Name() + "(" + fmt.Sprintf("%s", f.Args) + ")"
}

const (
	DIR_IN  = 1
	DIR_OUT = 2

	FLAVOR_SIMPLE = 0
	FLAVOR_RECORD = 1
	FLAVOR_TABLE  = 2
)

type Argument struct {
	Name   string
	Flavor uint8
	//Level                   uint8
	//Position                uint8
	Direction               uint8
	Type, PlsType, TypeName string
	Precision               uint8
	Scale                   uint8
	Charset                 string
	Charlength              uint
	TableOf                 []Argument
	RecordOf                map[string]Argument
}

func (a Argument) String() string {
	typ := a.Type
	switch a.Flavor {
	case FLAVOR_RECORD:
		typ = fmt.Sprintf("%s{%s}", a.PlsType, a.RecordOf)
	case FLAVOR_TABLE:
		typ = fmt.Sprintf("%s[%s]", a.PlsType, a.TableOf)
	}
	dir := ""
	switch a.Direction {
	case DIR_IN:
		dir = "IN"
	case DIR_OUT:
		dir = "OUT"
	default:
		dir = "INOUT"
	}
	return a.Name + " " + dir + " " + typ
}

func NewArgument(name, dataType, plsType, typeName, dirName string, dir uint8,
	charset string, precision, scale uint8, charlength uint) Argument {
	arg := Argument{Name: name, Type: dataType, PlsType: plsType,
		TypeName: typeName, Direction: dir,
		Precision: precision, Scale: scale, Charlength: charlength,
		Charset: charset}
	if arg.Direction < DIR_IN {
		arg.Direction = DIR_IN
	}
	if dirName != "" {
		switch dirName {
		case "IN/OUT":
			arg.Direction = DIR_IN | DIR_OUT
		case "OUT":
			arg.Direction = DIR_OUT
		default:
			arg.Direction = DIR_IN
		}
	}
	switch arg.Type {
	case "PL/SQL RECORD":
		arg.Flavor = FLAVOR_RECORD
		arg.RecordOf = make(map[string]Argument, 1)
	case "TABLE", "PL/SQL TABLE":
		arg.Flavor = FLAVOR_TABLE
		arg.TableOf = make([]Argument, 0, 1)
	}
	if arg.TypeName == "..@" {
		arg.TypeName = ""
	}
	return arg
}
