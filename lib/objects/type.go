// Copyright 2024, 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package objects

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"

	oracall "github.com/tgulacsi/oracall/lib"
)

type (
	Type struct {
		Elem *Type `json:"-"`
		// SELECT A.package_name, A.type_name, A.typecode, B.attr_type_owner, B.attr_type_name, B.attr_type_package, B.length, B.precision, B.scale, NULL AS index_by
		Owner, Package, Name        string
		TypeCode, CollType, IndexBy string
		Arguments                   []Argument
		TypeParams
	}
	Argument struct {
		Type *Type
		Name string
	}

	typeM struct {
		TypeIdx, ElemTypeIdx        uint
		Owner, Package, Name        string
		TypeCode, CollType, IndexBy string
		Arguments                   []argumentM
		Length, Precision, Scale    sql.NullInt32
	}
	argumentM struct {
		Name    string
		TypeIdx uint
	}
)

func (t *Type) IsZero() bool {
	return t == nil ||
		t.Name == "" ||
		t.TypeCode == "" ||
		t.Owner == "" && t.Package == "" && t.Name != "" && t.Length.Int32 == 0 && t.Precision.Int32 == 0 ||
		t.Elem == nil && t.IsColl() ||
		(t.Owner != "" || t.Package != "") && !t.IsColl() && len(t.Arguments) == 0
}

func (t Type) name(sep string) string {
	name := t.Name
	if t.Package != "" {
		name = t.Package + sep + name
	}
	if t.Owner != "" {
		name = t.Owner + sep + name
	}
	return name
}
func (t Type) IsColl() bool {
	switch t.TypeCode {
	case "COLLECTION", "TABLE", "VARYING ARRAY":
		return true
	default:
		return t.CollType != "" && t.CollType != "TABLE"
	}
}
func (t Type) String() string {
	if !t.composite() {
		return t.Name
	}
	var buf strings.Builder

	buf.WriteString(t.Owner)
	buf.WriteByte('.')
	if t.Package != "" {
		buf.WriteString(t.Package)
		buf.WriteByte('.')
	}
	buf.WriteString(t.Name)
	if t.Elem != nil {
		fmt.Fprintf(&buf, "[%s]", t.Elem)
	} else if len(t.Arguments) != 0 {
		fmt.Fprintf(&buf, "{%s}", t.Arguments)
	}
	return buf.String()
}
func (t Type) composite() bool {
	if t.Owner != "" || t.Package != "" {
		return true
	}
	return t.protoTypeName() == ""
}

func (t Type) ProtoMessageName() string {
	return oracall.CamelCase(strings.ReplaceAll(t.name("__"), "%", "__"))
}

func (t Type) WriteProtobufMessageType(ctx context.Context, w io.Writer) error {
	bw := bufio.NewWriter(w)
	// logger := zlog.SFromContext(ctx)
	fmt.Fprintf(bw, "\n// %s\nmessage %s {\n\toption (oracall_object_type) = %q;\n", t.name("."), t.ProtoMessageName(), t.OraType())
	if len(t.Arguments) != 0 {
		var i int
		for _, a := range t.Arguments {
			name := a.Name
			if strings.HasSuffix(name, "#") {
				name = name[:len(name)-1] + "_hidden"
			}
			s := a.Type
			oraType := s.OraType()
			var rule string
			// logger.Debug("attr", "t", t.Name, "a", a.Name, "type", fmt.Sprintf("%#v", s), "isColl", s.IsColl(), "elem", s.Elem)
			if s.IsColl() {
				rule = "repeated "
				if !isSimpleType(s.Name) && s.Elem != nil {
					s = s.Elem
				}
			} else if s.Elem != nil {
				rule = "repeated "
				s = s.Elem
				if name == "" {
					name = "record"
				}
			}
			i++
			fmt.Fprintf(bw, "\t%s%s %s = %d [(oracall_field_type) = %q];\n", rule, s.protoType(), strings.ToLower(name), i, oraType)
		}
	} else if s := t.Elem; s != nil {
		// fmt.Fprintf(bw, "\trepeated %s = %d;  //b %s\n", s.protoType(), 1, s.name("."))
	} else {
		fmt.Fprintf(bw, "\t  //c ?%#v\n", t)
		panic(fmt.Sprintf("%#v\n", t))
	}
	bw.WriteString("}\n")
	return bw.Flush()
}

func (t Type) OraType() string {
	if t.composite() {
		return t.name(".")
	}
	switch t.Name {
	case "CHAR", "VARCHAR2":
		return fmt.Sprintf("%s(%d)", t.Name, t.Length.Int32)
	case "NUMBER":
		if t.Precision.Valid {
			if t.Scale.Int32 != 0 {
				return fmt.Sprintf("NUMBER(%d,%d)", t.Precision.Int32, t.Scale.Int32)
			}
			return fmt.Sprintf("NUMBER(%d)", t.Precision.Int32)
		} else if t.Scale.Valid {
			return fmt.Sprintf("NUMBER(*,%d)", t.Scale.Int32)
		}
		return "NUMBER"
	default:
		return t.Name
	}
}

func (t Type) protoType() string {
	if t.Name == "XMLTYPE" && (t.Owner == "PUBLIC" || t.Owner == "SYS") ||
		t.Owner == "SYS" && strings.HasPrefix(t.Name, "JSON_") {
		return "string"
	}
	if t.composite() {
		return t.ProtoMessageName()
	}
	if typ := t.protoTypeName(); typ != "" {
		return typ
	}
	panic(fmt.Errorf("protoType(%q)", t.Name))
}

func isSimpleType(s string) bool {
	switch s {
	case "XMLTYPE", "PUBLIC.XMLTYPE", "RAW", "LONG RAW", "BLOB",
		"VARCHAR2", "CHAR", "LONG", "CLOB", "PL/SQL ROWID",
		"BOOLEAN", "PL/SQL BOOLEAN",
		"DATE", "TIMESTAMP",
		"PL/SQL PLS INTEGER", "PL/SQL BINARY INTEGER",
		"BINARY_DOUBLE",
		"NUMBER", "INTEGER":
		return true
	default:
		return false
	}
}
func (t Type) protoTypeName() string {
	switch t.Name {
	case "RAW", "LONG RAW", "BLOB":
		return "bytes"
	case "VARCHAR2", "CHAR", "LONG", "CLOB", "PL/SQL ROWID", "XMLTYPE", "PUBLIC.XMLTYPE":
		return "string"
	case "BOOLEAN", "PL/SQL BOOLEAN":
		return "bool"
	case "DATE", "TIMESTAMP":
		return "google.protobuf.Timestamp"
	case "PL/SQL PLS INTEGER", "PL/SQL BINARY INTEGER":
		return "int32"
	case "BINARY_DOUBLE":
		return "double"
	case "NUMBER", "INTEGER":
		if t.Precision.Valid {
			if t.Scale.Int32 == 0 {
				if t.Precision.Int32 < 10 {
					return "int32"
				} else if t.Precision.Int32 < 19 {
					return "int64"
				}
			}
		}
		return "string"
	}
	return ""
}
func (t *Type) Valid() bool {
	return t != nil && t.Name != "" && (isSimpleType(t.Name) || t.Elem != nil || t.Arguments != nil)
}

// protoGoName converts a proto field name (snake_case, may have trailing underscores)
// to the Go identifier used by protoc-gen-go (e.g. "size_" → "Size_", "f_szorzo" → "FSzorzo").
func protoGoName(protoField string) string {
	trailing := len(protoField) - len(strings.TrimRight(protoField, "_"))
	inner := protoField[:len(protoField)-trailing]
	parts := strings.Split(inner, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	b.WriteString(strings.Repeat("_", trailing))
	return b.String()
}

// protoFieldName converts an Oracle attribute name to the proto field name.
func protoFieldName(oraName string) string {
	name := oraName
	if strings.HasSuffix(name, "#") {
		name = name[:len(name)-1] + "_hidden"
	}
	return strings.ToLower(name)
}

func (t Type) WriteOraToFrom(ctx context.Context, w io.Writer) error {
	bw := bufio.NewWriter(w)
	msgName := t.ProtoMessageName()

	// WriteObject
	fmt.Fprintf(bw, "\nfunc (x *%s) WriteObject(o *godror.Object) error {\n\tvar data godror.Data\n", msgName)
	for _, a := range t.Arguments {
		goName := protoGoName(protoFieldName(a.Name))
		s := a.Type
		if s.IsColl() {
			if s.Elem == nil {
				continue // unresolved collection, skip
			}
			fmt.Fprintf(bw, "\tif items := x.Get%s(); len(items) > 0 {\n", goName)
			fmt.Fprintf(bw, "\t\tattr, ok := o.ObjectType.Attributes[%q]\n", a.Name)
			fmt.Fprintf(bw, "\t\tif !ok { return fmt.Errorf(\"no attribute %s in %%s\", o.ObjectType.FullName()) }\n", a.Name)
			fmt.Fprintf(bw, "\t\tcoll, err := attr.ObjectType.NewCollection()\n")
			fmt.Fprintf(bw, "\t\tif err != nil { return fmt.Errorf(\"NewCollection %s: %%w\", err) }\n", a.Name)
			fmt.Fprintf(bw, "\t\tdefer coll.Object.Close()\n")
			fmt.Fprintf(bw, "\t\tfor _, item := range items {\n")
			if isSimpleType(s.Elem.Name) {
				fmt.Fprintf(bw, "\t\t\tvar d godror.Data\n")
				fmt.Fprintf(bw, "\t\t\tif err := d.Set(%s); err != nil { return err }\n", s.Elem.writeObjectExpr("item"))
				fmt.Fprintf(bw, "\t\t\tif err := coll.AppendData(&d); err != nil { return err }\n")
			} else {
				fmt.Fprintf(bw, "\t\t\tif attr.ObjectType.CollectionOf == nil { return fmt.Errorf(\"CollectionOf not set for %s\") }\n", a.Name)
				fmt.Fprintf(bw, "\t\t\telemObj, err := attr.ObjectType.CollectionOf.NewObject()\n")
				fmt.Fprintf(bw, "\t\t\tif err != nil { return err }\n")
				fmt.Fprintf(bw, "\t\t\tif err = item.WriteObject(elemObj); err != nil { return err }\n")
				fmt.Fprintf(bw, "\t\t\tif err = coll.AppendObject(elemObj); err != nil { return err }\n")
			}
			fmt.Fprintf(bw, "\t\t}\n")
			fmt.Fprintf(bw, "\t\tdata.SetObject(coll.Object)\n")
			fmt.Fprintf(bw, "\t\tif err = o.SetAttribute(%q, &data); err != nil { return fmt.Errorf(\"SetAttribute %s: %%w\", err) }\n", a.Name, a.Name)
			fmt.Fprintf(bw, "\t}\n")
		} else if isSimpleType(s.Name) {
			fmt.Fprintf(bw, "\tif err := data.Set(%s); err != nil { return err }\n", s.writeObjectExpr("x.Get"+goName+"()"))
			fmt.Fprintf(bw, "\tif err := o.SetAttribute(%q, &data); err != nil { return err }\n", a.Name)
		} else {
			// nested object
			fmt.Fprintf(bw, "\tif sub := x.Get%s(); sub != nil {\n", goName)
			fmt.Fprintf(bw, "\t\tattr, ok := o.ObjectType.Attributes[%q]\n", a.Name)
			fmt.Fprintf(bw, "\t\tif !ok { return fmt.Errorf(\"no attribute %s in %%s\", o.ObjectType.FullName()) }\n", a.Name)
			fmt.Fprintf(bw, "\t\tsubObj, err := attr.ObjectType.NewObject()\n")
			fmt.Fprintf(bw, "\t\tif err != nil { return fmt.Errorf(\"NewObject %s: %%w\", err) }\n", a.Name)
			fmt.Fprintf(bw, "\t\tdefer subObj.Close()\n")
			fmt.Fprintf(bw, "\t\tif err = sub.WriteObject(subObj); err != nil { return fmt.Errorf(\"WriteObject %s: %%w\", err) }\n", a.Name)
			fmt.Fprintf(bw, "\t\tdata.SetObject(subObj)\n")
			fmt.Fprintf(bw, "\t\tif err = o.SetAttribute(%q, &data); err != nil { return fmt.Errorf(\"SetAttribute %s: %%w\", err) }\n", a.Name, a.Name)
			fmt.Fprintf(bw, "\t}\n")
		}
	}
	fmt.Fprintf(bw, "\treturn nil\n}\n")

	// Scan
	fmt.Fprintf(bw, "func (x *%s) Scan(v any) error {\n\tvar data godror.Data\n", msgName)
	fmt.Fprintf(bw, "\to, ok := v.(*godror.Object)\n\tif !ok { return fmt.Errorf(\"wanted Object, got %%T\", v) }\n")
	for _, a := range t.Arguments {
		goName := protoGoName(protoFieldName(a.Name))
		s := a.Type
		if s.IsColl() {
			if s.Elem == nil {
				continue
			}
			fmt.Fprintf(bw, "\tif err := o.GetAttribute(&data, %q); err != nil { return err }\n", a.Name)
			fmt.Fprintf(bw, "\tif collObj := data.GetObject(); collObj != nil {\n")
			fmt.Fprintf(bw, "\t\tcoll := collObj.Collection()\n")
			fmt.Fprintf(bw, "\t\tvar itemData godror.Data\n")
			fmt.Fprintf(bw, "\t\tfor i, err := coll.First(); err == nil; i, err = coll.Next(i) {\n")
			fmt.Fprintf(bw, "\t\t\tif err := coll.GetItem(&itemData, i); err != nil { return err }\n")
			if isSimpleType(s.Elem.Name) {
				fmt.Fprintf(bw, "\t\t\tx.%s = append(x.%s, %s)\n", goName, goName, s.Elem.dataGetter("itemData"))
			} else {
				fmt.Fprintf(bw, "\t\t\tif subObj := itemData.GetObject(); subObj != nil {\n")
				fmt.Fprintf(bw, "\t\t\t\telem := new(%s)\n", s.Elem.ProtoMessageName())
				fmt.Fprintf(bw, "\t\t\t\tif err := elem.Scan(subObj); err != nil { return err }\n")
				fmt.Fprintf(bw, "\t\t\t\tx.%s = append(x.%s, elem)\n", goName, goName)
				fmt.Fprintf(bw, "\t\t\t}\n")
			}
			fmt.Fprintf(bw, "\t\t}\n\t}\n")
		} else if isSimpleType(s.Name) {
			fmt.Fprintf(bw, "\tif err := o.GetAttribute(&data, %q); err != nil { return err }\n", a.Name)
			fmt.Fprintf(bw, "\tx.%s = %s\n", goName, s.dataGetter("data"))
		} else {
			// nested object
			fmt.Fprintf(bw, "\tif err := o.GetAttribute(&data, %q); err != nil { return err }\n", a.Name)
			fmt.Fprintf(bw, "\tif subObj := data.GetObject(); subObj != nil {\n")
			fmt.Fprintf(bw, "\t\tx.%s = new(%s)\n", goName, s.ProtoMessageName())
			fmt.Fprintf(bw, "\t\tif err := x.%s.Scan(subObj); err != nil { return err }\n", goName)
			fmt.Fprintf(bw, "\t}\n")
		}
	}
	fmt.Fprintf(bw, "\treturn nil\n}\n")
	return bw.Flush()
}

func writeArguments(bw *bufio.Writer, args []Argument) error {
	var i int
	for _, a := range args {
		name := a.Name
		if strings.HasSuffix(name, "#") {
			name = name[:len(name)-1] + "_hidden"
		}
		s := a.Type
		oraType := s.OraType()
		var rule string
		if s.IsColl() {
			rule = "repeated "
			if !isSimpleType(s.Name) && s.Elem != nil {
				s = s.Elem
			}
		} else if s.Elem != nil {
			rule = "repeated "
			s = s.Elem
			if name == "" {
				name = "record"
			}
		}
		i++
		fmt.Fprintf(bw, "\t%s%s %s = %d [(oracall_field_type) = %q];\n",
			rule, s.protoType(), strings.ToLower(name), i, oraType)
	}
	return nil
}

// writeObjectExpr returns a Go expression that converts val to something data.Set() accepts.
func (t Type) writeObjectExpr(val string) string {
	switch t.protoType() {
	case "google.protobuf.Timestamp":
		return val + ".AsTime()"
	default:
		return val
	}
}

func (t Type) dataGetter(dataName string) string {
	switch t.protoType() {
	case "bool":
		return dataName + ".GetBool()"
	case "google.protobuf.Timestamp":
		return "timestamppb.New(" + dataName + ".GetTime())"
	case "string":
		return "string(" + dataName + ".GetBytes())"
	case "bytes":
		return dataName + ".GetBytes()"
	case "int32":
		return "int32(" + dataName + ".GetInt64())"
	case "int64":
		return dataName + ".GetInt64()"
	case "double":
		return dataName + ".GetFloat64()"
	default:
		if strings.ContainsRune(t.protoType(), '_') {
			return dataName + ".GetObject() // " + t.protoType()
		}
	}
	panic(fmt.Errorf("no dataGetter for %s (%#v)", t.protoType(), t))
}
