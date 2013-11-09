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
	Precision               uint8
	Scale                   uint8
	Charset                 string
	Charlength              uint
	TableOf                 *Argument
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
	case "TABLE", "PL/SQL TABLE":
		arg.Flavor = FLAVOR_TABLE
	}
	return arg
}
