// Copyright 2013, 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/UNO-SOFT/zlog/v2/slog"
	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
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
	DefaultMaxRAWLength     = 32767
	DefaultMaxCHARLength    = 10
)

type Function struct {
	LastDDL           time.Time `json:",omitzero"`
	Replacement       *Function `json:",omitempty"`
	Returns           *Type     `json:",omitempty"`
	Package           string    `json:",omitzero"`
	name, alias       string
	Documentation     string     `json:",omitzero"`
	Args              []Argument `json:",omitempty"`
	Tag               []string   `json:",omitempty"`
	handle            []string
	maxTableSize      int
	ReplacementIsJSON bool `json:",omitzero"`
}

func (f Function) MarshalJSONTo(enc *jsontext.Encoder) error {
	W := func(k string, v any) { enc.WriteToken(jsontext.String(k)); json.MarshalEncode(enc, v) }
	enc.WriteToken(jsontext.BeginObject)
	W("Package", f.Package)
	W("Name", f.name)
	if f.alias != "" {
		W("Alias", f.alias)
	}
	W("RealName", f.RealName())
	W("LastDDL", f.LastDDL)
	if f.Replacement != nil {
		W("Replacement", f.Replacement.Name())
		if f.ReplacementIsJSON {
			W("ReplacementIsJSON", true)
		}
	}
	if f.Returns != nil {
		W("Returns", f.Returns.Type)
	}
	if f.Documentation != "" {
		W("Documentation", f.Documentation)
	}
	W("Args", f.Args)
	if len(f.Tag) != 0 {
		W("Tag", f.Tag)
	}
	if len(f.handle) != 0 {
		W("Handle", f.handle)
	}
	if f.maxTableSize != 0 {
		W("MaxTableSize", f.maxTableSize)
	}
	return enc.WriteToken(jsontext.EndObject)
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
		if arg.IsOutput() && arg.Type.Type == "REF CURSOR" {
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

type (
	Type struct {
		TableOf       *Type `json:",omitempty"` // this argument is a table (array) of this type
		mu            *sync.Mutex
		goTypeName    string
		Type          string     `json:",omitzero"`
		TypeName      string     `json:",omitzero"`
		AbsType       string     `json:",omitzero"`
		Charset       string     `json:",omitzero"`
		IndexBy       string     `json:",omitzero"`
		Documentation string     `json:",omitzero"`
		RecordOf      []Argument `json:",omitzero"` //this argument is a record (map) of this type
		PlsType
		Charlength uint      `json:",omitzero"`
		Flavor     flavor    `json:",omitzero"`
		Direction  direction `json:",omitzero"`
		Precision  uint8     `json:",omitzero"`
		Scale      uint8     `json:",omitzero"`
	}

	Argument struct {
		*Type
		Name string
	}
)

func (a Type) TypeString(prefix, indent string) string {
	typ := a.Type
	var suffix string
	var buf strings.Builder
	switch a.Flavor {
	case FLAVOR_RECORD:
		fmt.Fprintf(&buf, "%s%s{\n", prefix, a.PlsType)
		for _, b := range a.RecordOf {
			fmt.Fprintf(&buf, "%s%s- %s - %s\n", prefix, indent, b.Name, b.TypeString(prefix+indent, indent))
		}
		buf.WriteString(prefix + "}")
		return buf.String()
	case FLAVOR_TABLE:
		fmt.Fprintf(&buf, "%s%s[%s]\n", prefix, a.PlsType, a.TableOf.TypeString(prefix+indent, indent))
		return buf.String()
	}
	switch typ {
	case "CHAR", "NCHAR", "VARCHAR2", "NVARCHAR2":
		suffix = fmt.Sprintf("(%d)", a.Charlength)
	case "NUMBER":
		if a.Precision <= 0 {
			if a.Scale <= 0 {
				suffix = ""
			} else {
				suffix = fmt.Sprintf("(*,%d)", a.Scale)
			}
		} else {
			if a.Scale <= 0 {
				suffix = fmt.Sprintf("(%d)", a.Precision)
			} else {
				suffix = fmt.Sprintf("(%d,%d)", a.Precision, a.Scale)
			}
		}
	}
	return typ + suffix
}

func (t Type) String() string {
	typ := t.Type
	switch t.Flavor {
	case FLAVOR_RECORD:
		typ = fmt.Sprintf("%s{%v}", t.PlsType, t.RecordOf)
	case FLAVOR_TABLE:
		typ = fmt.Sprintf("%s[%v]", t.PlsType, t.TableOf)
	}
	return typ
}

func (a Argument) String() string {
	return a.Name + " " + a.Direction.String() + " " + a.Type.String()
}

func (a Type) IsInput() bool {
	return a.Direction&DIR_IN > 0
}
func (a Type) IsOutput() bool {
	return a.Direction&DIR_OUT > 0
}

// Should check for Associative Array (when using INDEX BY)
func (a Type) IsNestedTable() bool {
	if a.Type == "TABLE" && a.IndexBy == "" {
		return true
	}

	return false
}

func NewArgument(
	name, dataType, plsType, typeName, dirName string, dir direction,
	charset, indexBy string, precision, scale uint8, charlength uint,
) Argument {
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
		Name: name,
		Type: &Type{
			Type:     dataType,
			PlsType:  NewPlsType(plsType, precision, scale),
			TypeName: typeName, Direction: dir,
			Precision: precision, Scale: scale, Charlength: charlength,
			Charset: charset, IndexBy: indexBy,
			mu: new(sync.Mutex),
		},
	}
	if arg.ora == "" {
		panic(fmt.Sprintf("empty PLS type of %#v", arg))
	}
	switch arg.Type.Type {
	case "PL/SQL PLS INTEGER":
		arg.Type.Type = "PLS_INTEGER"
	case "PL/SQL BINARY INTEGER":
		arg.Type.Type = "BINARY_INTEGER"
	case "PL/SQL RECORD":
		arg.Flavor = FLAVOR_RECORD
		arg.RecordOf = make([]Argument, 0, 1)
	case "TABLE", "PL/SQL TABLE", "REF CURSOR":
		arg.Flavor = FLAVOR_TABLE
	}

	switch arg.Type.Type {
	case "CHAR", "NCHAR", "VARCHAR", "NVARCHAR", "VARCHAR2", "NVARCHAR2",
		"RAW":
		if arg.Charlength == 0 {
			if arg.Type.Type == "RAW" {
				arg.Charlength = DefaultMaxRAWLength
			} else if strings.Contains(arg.Type.Type, "VAR") {
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
		arg.AbsType = arg.Type.Type
	}
	return arg
}

func UnoCap(text string) string {
	i := strings.Index(text, "_")
	if i <= 0 {
		return capitalize(text)
	}
	return strings.ToUpper(text[:i]) + "_" + strings.ToLower(text[i+1:])
}
