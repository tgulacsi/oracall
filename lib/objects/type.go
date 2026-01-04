// Copyright 2024, 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package objects

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tgulacsi/oracall/lib"
)

type (
	Type struct {
		Elem *Type `json:"-"`
		// SELECT A.package_name, A.type_name, A.typecode, B.attr_type_owner, B.attr_type_name, B.attr_type_package, B.length, B.precision, B.scale, NULL AS index_by
		Owner, Package, Name        string
		TypeCode, CollType, IndexBy string
		Arguments                   []Argument
		Length, Precision, Scale    sql.NullInt32
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
	if t.Arguments == nil {
		return nil
	}
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
					// } else if t.Precision.Int32 < 19 {
					// 	return "int64"
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

var errNotImplemented = errors.New("not implemented")

func (t Type) WriteOraToFrom(ctx context.Context, w io.Writer) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, `
func (x *%s) WriteObject(o *godror.Object) error {
	return nil
}
func (x *%s) Scan(v any) error {
	return nil
}
		`,
		t.ProtoMessageName(), t.ProtoMessageName(),
	)
	return bw.Flush()
}
