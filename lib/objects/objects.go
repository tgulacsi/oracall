// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/godror/godror"
	"golang.org/x/sync/errgroup"
)

type (
	Type struct {
		Elem *Type
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

	Types struct {
		db            querier
		m             map[string]*Type
		currentSchema string
		mu            sync.RWMutex
	}

	querier interface {
		godror.Querier
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}
)

func NewTypes(ctx context.Context, db querier) (*Types, error) {
	var currentSchema string
	{
		const qry = `SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') FROM DUAL`
		if err := db.QueryRowContext(ctx, qry).Scan(&currentSchema); err != nil {
			return nil, fmt.Errorf("%s: %w", qry, err)
		}
	}
	return &Types{db: db, m: make(map[string]*Type), currentSchema: currentSchema}, nil
}

func (tt *Types) Names(ctx context.Context, only ...string) ([]string, error) {
	const qry = `SELECT A.type_name, NULL AS package_name, A.typecode FROM user_types A
UNION ALL
SELECT A.type_name, A.package_name, A.typecode FROM user_plsql_types A
  ORDER BY 2, 1`
	rows, err := tt.db.QueryContext(ctx, qry, godror.PrefetchCount(1025), godror.FetchArraySize(1024))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", qry, err)
	}
	defer rows.Close()
	grp, grpCtx := errgroup.WithContext(ctx)
	grp.SetLimit(8)
	for rows.Next() {
		var t Type
		if err := rows.Scan(&t.Name, &t.Package, &t.TypeCode); err != nil {
			return nil, fmt.Errorf("scan %s: %w", qry, err)
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
		key := t.name(".")
		tt.mu.RLock()
		u := tt.m[key]
		tt.mu.RUnlock()
		if u != nil {
			continue
		}
		u = &t
		grp.Go(func() error {
			tt.mu.Lock()
			defer tt.mu.Unlock()
			if tt.m[key] != nil {
				return nil
			}
			tt.m[key] = u
			_, err = tt.get(grpCtx, key)
			return err
		})
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if err = grp.Wait(); err != nil {
		return nil, err
	}
	tt.mu.RLock()
	names := make([]string, 0, len(tt.m))
	for nm := range tt.m {
		names = append(names, nm)
	}
	tt.mu.RUnlock()
	slices.Sort(names)
	return names, err
}

var errUnknownType = errors.New("unknown type")

func (tt *Types) Get(ctx context.Context, name string) (*Type, error) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	return tt.get(ctx, name)
}

func (tt *Types) get(ctx context.Context, name string) (*Type, error) {
	if name == "" {
		panic(`Types.Get("")`)
	}
	logger := zlog.SFromContext(ctx)
	_ = logger
	t := tt.m[name]
	if t == nil {
		const qry = `SELECT A.type_name, NULL AS package_name, A.typecode FROM user_types A WHERE type_name = :type
UNION ALL
SELECT A.type_name, A.package_name, A.typecode FROM user_plsql_types A WHERE package_name = :package AND type_name = :type
`
		pkg, typ, ok := strings.Cut(name, ".")
		if !ok {
			pkg, typ = typ, pkg
		}
		t = &Type{}
		if err := tt.db.QueryRowContext(ctx, qry, sql.Named("package", pkg), sql.Named("type", typ)).Scan(&t.Name, &t.Package, &t.TypeCode); err != nil {
			return nil, fmt.Errorf("%q: %s [%q, %q]: %w: %w", name, qry, pkg, typ, err, errUnknownType)
		}
		name = t.name(".")
		tt.m[name] = t
	}
	const (
		collQry = `
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
		attrQry = `SELECT B.attr_no, B.attr_name, B.attr_type_owner, B.attr_type_name, NULL AS attr_package_name, B.length, B.precision, B.scale, NULL as coll_type, NULL AS index_by
  FROM all_type_attrs B 
  WHERE B.owner = :owner AND B.type_name = :name
UNION ALL
SELECT B.attr_no, B.attr_name, B.attr_type_owner, B.attr_type_name, B.attr_type_package, B.length, B.precision, B.scale, NULL as coll_type, NULL AS index_by
  FROM user_plsql_type_attrs B 
  WHERE B.package_name = :package AND B.type_name = :name
  ORDER BY 1, 2
`
		tblQry = `
SELECT B.column_id AS attr_no, B.column_name, B.data_type_owner, B.data_type, NULL, B.data_length, B.data_precision, B.data_scale, NULL as coll_type, NULL AS index_by
  FROM user_tab_cols B 
  WHERE B.table_name = REGEXP_REPLACE(:name, '%ROWTYPE$') AND :package IS NULL AND 
        COALESCE(:owner, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')) = SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')
  ORDER BY 1, 2`
	)

	logger.Debug("resolve", "t", t)
	typesKey := t.name(".")
	u := tt.m[typesKey]
	if u != nil && t.Valid() {
		*t = *u
		return t, nil
	}
	qry := attrQry
	isColl := t.IsColl()
	if strings.HasSuffix(t.Name, "%ROWTYPE") {
		qry = tblQry
	} else if isColl {
		qry = collQry
	}
	// logger.Debug("resolve", "type", t, "isColl", isColl)
	rows, err := tt.db.QueryContext(ctx, qry, sql.Named("package", t.Package), sql.Named("owner", t.Owner), sql.Named("name", t.Name))
	if err != nil {
		return t, fmt.Errorf("%s: %w", qry, err)
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
			return t, fmt.Errorf("%s: %w", qry, err)
		}
		if s.Owner != "" && s.Owner == tt.currentSchema {
			s.Owner = ""
		}
		// if s.Owner != "" {
		// logger.Debug("resolve", "s", fmt.Sprintf("%#v", s), "coll", s.IsColl())
		// }
		p := &s
		if !p.Valid() {
			key := p.name(".")
			tt.m[key] = p
			if p, err = tt.get(ctx, p.name(".")); err != nil {
				return t, err
			}
		}
		if isColl {
			t.Elem = p
		} else {
			t.Arguments = append(t.Arguments, Argument{Name: attrName, Type: p})
		}
	}
	logger.Debug("resolved", "type", t)
	if !t.Valid() {
		return t, fmt.Errorf("could not resolve %s: %#v", typesKey, t)
	}
	// if x := tt.m[typesKey]; x == nil || x.Elem == nil || x.Arguments == nil {
	tt.m[typesKey] = t
	// }

	if err := rows.Close(); err != nil {
		return t, err
	}

	/*
		var fill func(t *Type) *Type
		fill = func(t *Type) *Type {
			if t == nil || t.Name == "" || !t.composite() {
				return t
			}
			if !t.Valid() {
				logger.Debug("fill", "t", t)
				u, err := tt.get(ctx, t.name("."))
				if err != nil {
					if t.name(".") != "PUBLIC.XMLTYPE" {
						panic(fmt.Errorf("no type found of %s: %w", t.name("."), err))
					}
				} else {
					*t = *u
					return u
				}
			}
			// logger.Debug("fill", "elem", t)
			t.Elem = fill(t.Elem)
			for i, v := range t.Arguments {
				v.Type = fill(v.Type)
				t.Arguments[i] = v
			}
			return t
		}
		for _, nm := range seen {
			fill(tt.m[nm])
		}
	*/
	return t, nil
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

func (t Type) protoMessageName() string {
	return strings.ReplaceAll(t.name("__"), "%", "__")
}

func (t Type) WriteProtobufMessageType(ctx context.Context, w io.Writer) error {
	if t.Arguments == nil {
		return nil
	}
	bw := bufio.NewWriter(w)
	// logger := zlog.SFromContext(ctx)
	fmt.Fprintf(bw, "// %s\nmessage %s {\n", t.name("."), t.protoMessageName())
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
			fmt.Fprintf(bw, "\t%s%s %s = %d;  // %s\n", rule, s.protoType(), a.Name, i, s.name("."))
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
		return t.protoMessageName()
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
func (t Type) Valid() bool {
	return t.Name != "" && (isSimpleType(t.Name) || t.Elem != nil || t.Arguments != nil)
}
