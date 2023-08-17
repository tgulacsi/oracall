// Copyright 2013, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/UNO-SOFT/zlog/v2/slog"
)

// Log is discarded by default.
var logger *slog.Logger

func SetLogger(lgr *slog.Logger) { logger = lgr }

const (
	MarkNull = "\u2400" // 0x2400 = nul
	//MarkValid  = "\u6eff" // 0x6eff = fill; full, satisfied
	MarkValid = "Valid" // 0x6eff = fill; full, satisfied
	//MarkHidden = "\u533f"     // 0x533f = hide
	MarkHidden = "_hidden"

	DefaultMaxVARCHARLength = 32767
	DefaultMaxCHARLength    = 10
)

type Function struct {
	LastDDL              time.Time
	Replacement          *Function
	Returns              *Argument
	Package, name, alias string
	Documentation        string
	Args                 []Argument
	Tag, handle          []string
	maxTableSize         int
	ReplacementIsJSON    bool
}

func (f Function) Name() string {
	nm := strings.ToLower(f.name)
	if f.alias != "" {
		nm = strings.ToLower(f.name)
	}
	if f.Package == "" {
		return nm
	}
	return UnoCap(f.Package) + "." + nm
}
func (f Function) RealName() string {
	if f.Replacement != nil {
		return f.Replacement.RealName()
	}
	nm := strings.ToLower(f.name)
	if f.Package == "" {
		return nm
	}
	return UnoCap(f.Package) + "." + nm
}

func (f Function) String() string {
	args := make([]string, len(f.Args))
	for i := range args {
		args[i] = f.Args[i].String()
	}
	s := f.Name() + "(" + strings.Join(args, ", ") + ")"
	if f.Documentation == "" {
		return s
	}
	return s + "\n" + f.Documentation
}

func (f Function) HasCursorOut() bool {
	if f.Returns != nil &&
		f.Returns.IsOutput() && f.Returns.Type == "REF CURSOR" {
		return true
	}
	for _, arg := range f.Args {
		if arg.IsOutput() && arg.Type == "REF CURSOR" {
			return true
		}
	}
	return false
}

type direction uint8

func (dir direction) IsInput() bool  { return dir&DIR_IN > 0 }
func (dir direction) IsOutput() bool { return dir&DIR_OUT > 0 }
func (dir direction) String() string {
	switch dir {
	case DIR_IN:
		return "IN"
	case DIR_OUT:
		return "OUT"
	case DIR_INOUT:
		return "INOUT"
	}
	return fmt.Sprintf("%d", dir)
}
func (dir direction) MarshalText() ([]byte, error) {
	return []byte(dir.String()), nil
}

const (
	DIR_IN    = direction(1)
	DIR_OUT   = direction(2)
	DIR_INOUT = direction(3)
)

type flavor uint8

func (f flavor) String() string {
	switch f {
	case FLAVOR_SIMPLE:
		return "SIMPLE"
	case FLAVOR_RECORD:
		return "RECORD"
	case FLAVOR_TABLE:
		return "TABLE"
	}
	return fmt.Sprintf("%d", f)
}
func (f flavor) MarshalText() ([]byte, error) {
	return []byte(f.String()), nil
}

const (
	FLAVOR_SIMPLE = flavor(0)
	FLAVOR_RECORD = flavor(1)
	FLAVOR_TABLE  = flavor(2)
)

type Argument struct {
	TableOf          *Argument // this argument is a table (array) of this type
	mu               *sync.Mutex
	goTypeName       string
	Name             string
	Type, TypeName   string
	AbsType          string
	Charset, IndexBy string
	RecordOf         []NamedArgument //this argument is a record (map) of this type
	PlsType
	Charlength uint
	Flavor     flavor
	Direction  direction
	Precision  uint8
	Scale      uint8
}
type NamedArgument struct {
	*Argument
	Name string
}

func (a Argument) String() string {
	typ := a.Type
	switch a.Flavor {
	case FLAVOR_RECORD:
		typ = fmt.Sprintf("%s{%v}", a.PlsType, a.RecordOf)
	case FLAVOR_TABLE:
		typ = fmt.Sprintf("%s[%v]", a.PlsType, a.TableOf)
	}
	return a.Name + " " + a.Direction.String() + " " + typ
}

func (a Argument) IsInput() bool {
	return a.Direction&DIR_IN > 0
}
func (a Argument) IsOutput() bool {
	return a.Direction&DIR_OUT > 0
}

func NewArgument(name, dataType, plsType, typeName, dirName string, dir direction,
	charset, indexBy string, precision, scale uint8, charlength uint) Argument {

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
			dir = DIR_INOUT
		case "OUT":
			dir = DIR_OUT
		default:
			dir = DIR_IN
		}
	}
	if dir < DIR_IN {
		dir = DIR_IN
	}

	arg := Argument{
		Name: name, Type: dataType,
		PlsType:  NewPlsType(plsType, precision, scale),
		TypeName: typeName, Direction: dir,
		Precision: precision, Scale: scale, Charlength: charlength,
		Charset: charset, IndexBy: indexBy,
		mu: new(sync.Mutex),
	}
	if arg.ora == "" {
		panic(fmt.Sprintf("empty PLS type of %#v", arg))
	}
	switch arg.Type {
	case "PL/SQL PLS INTEGER":
		arg.Type = "PLS_INTEGER"
	case "PL/SQL BINARY INTEGER":
		arg.Type = "BINARY_INTEGER"
	case "PL/SQL RECORD":
		arg.Flavor = FLAVOR_RECORD
		arg.RecordOf = make([]NamedArgument, 0, 1)
	case "TABLE", "PL/SQL TABLE", "REF CURSOR":
		arg.Flavor = FLAVOR_TABLE
	}

	switch arg.Type {
	case "CHAR", "NCHAR", "VARCHAR", "NVARCHAR", "VARCHAR2", "NVARCHAR2":
		if arg.Charlength == 0 {
			if strings.Contains(arg.Type, "VAR") {
				arg.Charlength = DefaultMaxVARCHARLength
			} else {
				arg.Charlength = DefaultMaxCHARLength
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

func UnoCap(text string) string {
	i := strings.Index(text, "_")
	if i == 0 {
		return capitalize(text)
	}
	return strings.ToUpper(text[:i]) + "_" + strings.ToLower(text[i+1:])
}
