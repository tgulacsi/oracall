/*
Copyright 2013 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package structs

import (
	"fmt"
	"strings"
)

const (
	MarkNull = "␀" // 0x2400 = nul
	//MarkValid  = "滿" // 0x6eff = fill; full, satisfied
	MarkValid  = "Valid" // 0x6eff = fill; full, satisfied
	MarkHidden = "匿"     // 0x533f = hide
)

type Function struct {
	Package, name string
	Returns       *Argument
	Args          []Argument
	types         map[string]string
}

func (f Function) Name() string {
	if f.Package == "" {
		return strings.ToLower(f.name)
	}
	return unocap(f.Package) + "." + strings.ToLower(f.name)
}

func (f Function) String() string {
	args := make([]string, len(f.Args))
	for i := range args {
		args[i] = f.Args[i].String()
	}
	return f.Name() + "(" + strings.Join(args, ", ") + ")"
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
	AbsType                 string
	Precision               uint8
	Scale                   uint8
	Charset                 string
	Charlength              uint
	TableOf                 *Argument           // this argument is a table (array) of this type
	RecordOf                map[string]Argument //this argument is a record (map) of this type
	goTypeName              string
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

func (a Argument) IsInput() bool {
	return a.Direction&DIR_IN > 0
}
func (a Argument) IsOutput() bool {
	return a.Direction&DIR_OUT > 0
}

func NewArgument(name, dataType, plsType, typeName, dirName string, dir uint8,
	charset string, precision, scale uint8, charlength uint) Argument {

	name = strings.ToLower(name)
	if typeName == "..@" {
		typeName = ""
	}
	if typeName != "" && typeName[len(typeName)-1] == '@' {
		typeName = typeName[:len(typeName)-1]
	}

	if dirName != "" {
		switch dirName {
		case "IN/OUT":
			dir = DIR_IN | DIR_OUT
		case "OUT":
			dir = DIR_OUT
		default:
			dir = DIR_IN
		}
	}
	if dir < DIR_IN {
		dir = DIR_IN
	}

	arg := Argument{Name: name, Type: dataType, PlsType: plsType,
		TypeName: typeName, Direction: dir,
		Precision: precision, Scale: scale, Charlength: charlength,
		Charset: charset}
	switch arg.Type {
	case "PL/SQL RECORD":
		arg.Flavor = FLAVOR_RECORD
		arg.RecordOf = make(map[string]Argument, 1)
		if arg.TypeName == "" {
			arg.TypeName = arg.Name
		}
	case "TABLE", "PL/SQL TABLE":
		arg.Flavor = FLAVOR_TABLE
	}

	switch arg.Type {
	case "CHAR", "NCHAR", "VARCHAR", "NVARCHAR", "VARCHAR2", "NVARCHAR2":
		if arg.Charlength <= 0 {
			if strings.Index(arg.Type, "VAR") >= 0 {
				arg.Charlength = 1000
			} else {
				arg.Charlength = 10
			}
		}
		arg.AbsType = fmt.Sprintf("%s(%d)", arg.Type, arg.Charlength)
	case "NUMBER":
		if arg.Scale > 0 {
			arg.AbsType = fmt.Sprintf("NUMBER(%d, %d)", arg.Precision, arg.Scale)
		} else if arg.Precision > 0 {
			arg.AbsType = fmt.Sprintf("NUMBER(%d)", arg.Precision)
		} else {
			arg.AbsType = "NUMBER"
		}
	case "PLS_INTEGER", "BINARY_INTEGER":
		arg.AbsType = "INTEGER(10)"
	default:
		arg.AbsType = arg.Type
	}
	return arg
}
