// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"sync"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/godror/godror"
	"golang.org/x/sync/errgroup"
)

type Type struct {
	// SELECT A.package_name, A.type_name, A.typecode, B.attr_type_owner, B.attr_type_name, B.attr_type_package, B.length, B.precision, B.scale, NULL AS index_by
	Owner, Package, Name        string
	TypeCode, CollType, IndexBy string
	Length, Precision, Scale    sql.NullInt32
	Elem                        *Type
	Arguments                   []Argument
}
type Argument struct {
	Name string
	Type *Type
}

func ReadTypes(ctx context.Context,
	db interface {
		godror.Querier
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	only ...string,
) (map[string]*Type, error) {
	logger := zlog.SFromContext(ctx)
	_ = logger
	var currentSchema string
	{
		const qry = `SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') FROM DUAL`
		if err := db.QueryRowContext(ctx, qry).Scan(&currentSchema); err != nil {
			return nil, fmt.Errorf("%s: %w", qry, err)
		}
	}
	const qry = `SELECT A.type_name, NULL AS package_name, A.typecode FROM user_types A
UNION ALL
SELECT A.type_name, A.package_name, A.typecode FROM user_plsql_types A`
	rows, err := db.QueryContext(ctx, qry, godror.PrefetchCount(1025), godror.FetchArraySize(1024))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", qry, err)
	}
	defer rows.Close()
	// TODO T_UGYFEL%ROWTYPE
	types := make(map[string]*Type)
	list := make([]*Type, 0, 1024)
	for rows.Next() {
		var t Type
		if err := rows.Scan(&t.Name, &t.Package, &t.TypeCode); err != nil {
			return types, fmt.Errorf("scan %s: %w", qry, err)
		}
		if len(only) != 0 {
			var found bool
			for _, k := range only {
				if found = k == t.Name || k == t.Package; found {
					break
				}
			}
			if !found {
				continue
			}
		}
		list = append(list, &t)
	}
	if err = rows.Close(); err != nil {
		return types, err
	}
	const collQry = `
SELECT NULL AS attr_no, NULL AS attr_name, B.elem_type_owner, B.elem_type_name, NULL AS elem_package_name, B.length, B.precision, B.scale, B.coll_type, NULL AS index_by
  FROM all_coll_types B 
  WHERE B.owner = :owner AND B.type_name = :name
UNION ALL
SELECT NULL AS attr_no, NULL AS attr_name, B.elem_type_owner, B.elem_type_name, B.elem_type_package, B.length, B.precision, B.scale, B.coll_type, B.index_by 
  FROM user_plsql_coll_types B 
  WHERE B.type_name = :name AND B.package_name = :package
UNION ALL
SELECT B.column_id AS attr_no, B.column_name, B.data_type_owner, B.data_type, NULL, B.data_length, B.data_precision, B.data_scale, NULL as coll_type, NULL AS index_by
  FROM user_plsql_coll_types A INNER JOIN user_tab_cols B ON B.table_name = REGEXP_REPLACE(A.elem_type_name, '%ROWTYPE$') 
  WHERE A.elem_type_name = :name AND A.elem_type_name LIKE '%ROWTYPE'
  ORDER BY 1, 2, 3
`
	const attrQry = `SELECT B.attr_no, B.attr_name, B.attr_type_owner, B.attr_type_name, NULL AS attr_package_name, B.length, B.precision, B.scale, NULL as coll_type, NULL AS index_by
  FROM all_type_attrs B 
  WHERE B.owner = :owner AND B.type_name = :name
UNION ALL
SELECT B.attr_no, B.attr_name, B.attr_type_owner, B.attr_type_name, B.attr_type_package, B.length, B.precision, B.scale, NULL as coll_type, NULL AS index_by
  FROM user_plsql_type_attrs B 
  WHERE B.package_name = :package AND B.type_name = :name
  ORDER BY 1, 2
`
	var mu sync.RWMutex
	var resolve func(ctx context.Context, t *Type) error
	resolve = func(ctx context.Context, t *Type) error {
		if isSimpleType(t.Name) {
			return nil
		}
		typesKey := t.name(".")
		mu.RLock()
		u, ok := types[typesKey]
		mu.RUnlock()
		if ok {
			*t = *u
			return nil
		}
		qry := attrQry
		isColl := t.IsColl()
		if isColl {
			qry = collQry
		}
		// logger.Debug("resolve", "type", t, "isColl", isColl)
		rows, err := db.QueryContext(ctx, qry, sql.Named("package", t.Package), sql.Named("owner", t.Owner), sql.Named("name", t.Name))
		if err != nil {
			return fmt.Errorf("%s: %w", qry, err)
		}
		defer rows.Close()
		for rows.Next() {
			var attrName string
			var attrNo sql.NullInt32
			var s Type
			if err := rows.Scan(
				// SELECT B.attr_type_owner, B.attr_type_name, B.attr_type_package, B.length, B.precision, B.scale, NULL as coll_type, NULL AS index_by
				&attrNo, &attrName,
				&s.Owner, &s.Name, &s.Package, &s.Length, &s.Precision, &s.Scale, &s.CollType, &s.IndexBy,
			); err != nil {
				return fmt.Errorf("%s: %w", qry, err)
			}
			if s.Owner != "" && s.Owner == currentSchema {
				s.Owner = ""
			}
			if s.Owner != "" {
				logger.Debug("resolve", "s", fmt.Sprintf("%#v", s), "coll", s.IsColl())
			}
			p := &s
			if err = resolve(ctx, p); err != nil {
				return err
			}
			if isColl {
				t.Elem = p
			} else {
				t.Arguments = append(t.Arguments, Argument{Name: attrName, Type: p})
			}
		}
		// logger.Debug("resolved", "type", t)
		if t.composite() && !(t.Elem == nil && t.Arguments == nil) {
			mu.Lock()
			if x := types[typesKey]; x == nil || x.Elem == nil || x.Arguments == nil {
				types[typesKey] = t
			}
			mu.Unlock()
		}

		return rows.Close()
	}

	grp, grpCtx := errgroup.WithContext(ctx)
	grp.SetLimit(8)
	for _, t := range list {
		t := t
		grp.Go(func() error { return resolve(grpCtx, t) })
	}
	err = grp.Wait()

	var fill func(t *Type) *Type
	fill = func(t *Type) *Type {
		if t == nil || !t.composite() {
			return t
		}
		if t.Elem == nil && t.Arguments == nil {
			nm := t.name(".")
			if u := types[nm]; u == nil {
				if nm != "PUBLIC.XMLTYPE" {
					panic("no type found of " + t.name("."))
				}
			} else {
				return u
			}
		}
		logger.Debug("fill", "elem", t)
		t.Elem = fill(t.Elem)
		for i, v := range t.Arguments {
			v.Type = fill(v.Type)
			t.Arguments[i] = v
		}
		return t
	}
	for _, t := range types {
		fill(t)
	}
	return types, err
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
	if t.composite() {
		return fmt.Sprintf("%s.%s.%s[%s]{%s}", t.Owner, t.Package, t.Name, t.Elem, t.Arguments)
	}
	return t.Name
}
func (t Type) composite() bool {
	if t.Owner != "" || t.Package != "" {
		return true
	}
	return t.protoTypeName() == ""
}

func (t Type) WriteProtobufMessageType(ctx context.Context, w io.Writer) error {
	if t.Arguments == nil {
		return nil
	}
	bw := bufio.NewWriter(w)
	// logger := zlog.SFromContext(ctx)
	fmt.Fprintf(bw, "// %s\nmessage %s {\n", t.name("."), t.name("__"))
	if t.Arguments != nil {
		var i int
		for _, a := range t.Arguments {
			s := a.Type
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
			}
			i++
			fmt.Fprintf(bw, "\t%s%s %s = %d;  //a %s\n", rule, s.protoType(), a.Name, i, s.name("."))
		}
	} else if s := t.Elem; s != nil {
		fmt.Fprintf(bw, "\trepeated %s = %d;  //b %s\n", s.protoType(), 1, s.name("."))
	} else {
		fmt.Fprintf(bw, "\t  //c ?%#v\n", t)
		panic(fmt.Sprintf("%#v\n", t))
	}
	bw.WriteString("}\n")
	return bw.Flush()
}

func (t Type) protoType() string {
	if t.composite() {
		return t.name("__")
	}
	if typ := t.protoTypeName(); typ != "" {
		return typ
	}
	panic(fmt.Errorf("protoType(%q)", t.Name))
}

func isSimpleType(s string) bool {
	switch s {
	case "RAW", "LONG RAW", "BLOB",
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
	case "VARCHAR2", "CHAR", "LONG", "CLOB", "PL/SQL ROWID":
		return "string"
	case "BOOLEAN", "PL/SQL BOOLEAN":
		return "bool"
	case "DATE", "TIMESTAMP":
		return "google.protobuf.Timestamp"
	case "PL/SQL PLS INTEGER", "PL/SQL BINARY INTEGER":
		return "int32"
	case "BINARY_DOUBLE":
		return "float"
	case "NUMBER", "INTEGER":
		if t.Precision.Valid {
			if t.Scale.Int32 == 0 {
				if t.Precision.Int32 < 10 {
					return "int32"
				} else if t.Precision.Int32 < 20 {
					return "int64"
				}
			} else if t.Scale.Valid {
				if t.Precision.Int32 < 8 {
					return "float"
				} else if t.Precision.Int32 < 16 {
					return "double"
				}
			}
		}
		return "string"
	}
	return ""
}
