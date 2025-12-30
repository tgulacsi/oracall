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
	"github.com/tgulacsi/oracall/lib"
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
	rows, err := tt.db.QueryContext(ctx, qry,
		godror.PrefetchCount(1025), godror.FetchArraySize(1024))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", qry, err)
	}
	defer rows.Close()
	logger := zlog.SFromContext(ctx)
	grp, grpCtx := errgroup.WithContext(ctx)
	grp.SetLimit(4)
	for rows.Next() {
		t := Type{Owner: tt.currentSchema}
		if err := rows.Scan(&t.Name, &t.Package, &t.TypeCode); err != nil {
			return nil, fmt.Errorf("scan %s: %w", qry, err)
		}
		if len(only) != 0 {
			var found bool
			for _, k := range only {
				i := strings.IndexByte(k, '.')
				if found = i < 0 && (k == t.Name || k == t.Package) ||
					i >= 0 && k == t.Package+"."+t.Name; found {
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
			if errors.Is(err, ErrNotSupported) {
				logger.Warn("not supported", "type", key, "error", err)
				err = nil
			}
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

// INDEX BY VARCHAR2 nem tamogatott!

func (tt *Types) get(ctx context.Context, name string) (*Type, error) {
	if name == "" {
		panic(`Types.Get("")`)
	}
	logger := zlog.SFromContext(ctx)
	_ = logger
	t := tt.m[name]
	if t == nil || t.TypeCode == "" {
		const qry = `SELECT A.owner, A.type_name, NULL AS package_name, A.typecode
  FROM all_types A
  WHERE owner IN (COALESCE(:owner, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')), COALESCE(:package, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA'))) AND
        type_name = :type
UNION ALL
SELECT A.owner, A.type_name, A.package_name, A.typecode
  FROM all_plsql_types A
  WHERE owner = COALESCE(:owner, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')) AND
        package_name = :package AND type_name = :type
UNION ALL
SELECT A.owner, A.type_name, A.package_name, A.typecode
  FROM all_plsql_types A
  WHERE owner = COALESCE(:package, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')) AND
        package_name IS NULL AND type_name = :type
UNION ALL
SELECT A.owner, A.type_name, NULL AS package_name, A.typecode
  FROM all_types A
  WHERE type_name = :type
UNION ALL
SELECT owner, :type AS type_name, NULL AS package_name, '%ROWTYPE' AS typecode
  FROM all_tables
  WHERE owner IN (COALESCE(:owner, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')), COALESCE(:package, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA'))) AND
        table_name = REGEXP_REPLACE(:type, '%ROWTYPE$')
`
		owner, pkg, typ := tt.currentSchema, "", name
		fields := strings.SplitN(name, ".", 4)
		switch len(fields) {
		case 1:
		case 2:
			if strings.HasSuffix(name, "%ROWTYPE") {
				owner, typ = fields[0], fields[1]
			} else {
				pkg, typ = fields[0], fields[1]
			}
		case 3:
			owner, pkg, typ = fields[0], fields[1], fields[2]
		default:
			return nil, fmt.Errorf("too many parts: %q", fields)
		}

		t = &Type{}
		if err := tt.db.QueryRowContext(ctx, qry,
			sql.Named("owner", owner),
			sql.Named("package", pkg),
			sql.Named("type", typ),
		).Scan(
			&t.Owner, &t.Name, &t.Package, &t.TypeCode,
		); err != nil {
			return nil, fmt.Errorf("get basic type info %q: %s [owner=%q, pkg=%q, typ=%q]: %w: %w", name, qry,
				owner, pkg, typ,
				err, errUnknownType)
		}

		name = t.name(".")
		tt.m[name] = t
	}
	switch name {
	case "PUBLIC.JSON_ARRAY_T", "SYS.JDOM_T", "SYS.JSON_ELEMENT_T", "SYS.JSON_ARRAY_T":
		return t, nil
	}

	const (
		collQry = `
SELECT 'C' AS orig, 0+NULL AS attr_no, NULL AS attr_name, B.elem_type_owner, B.elem_type_name, NULL AS elem_package_name, B.length, B.precision, B.scale, B.coll_type, NULL AS index_by
  FROM all_coll_types B
  WHERE B.owner = COALESCE(:owner, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')) AND
        B.type_name = :name
UNION ALL
SELECT 'D' as orig, 0+NULL AS attr_no, NULL AS attr_name, B.elem_type_owner, B.elem_type_name, B.elem_type_package, B.length, B.precision, B.scale, B.coll_type, B.index_by
  FROM user_plsql_coll_types B
  WHERE B.package_name = :package AND
        B.type_name = :name
  ORDER BY 2, 3
`

		attrQry = `
SELECT 'A' as orig, B.attr_no, B.attr_name, B.attr_type_owner, B.attr_type_name, NULL AS attr_package_name, B.length, B.precision, B.scale, NULL as coll_type, NULL AS index_by
  FROM all_type_attrs B
  WHERE B.owner = COALESCE(:owner, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')) AND
        B.type_name = :name
UNION ALL
SELECT 'B' as orign, B.attr_no, B.attr_name, B.attr_type_owner, B.attr_type_name, B.attr_type_package, B.length, B.precision, B.scale, NULL as coll_type, NULL AS index_by
  FROM user_plsql_type_attrs B
  WHERE B.package_name = :package AND
        B.type_name = :name
  ORDER BY 2, 3
`

		tblQry = `
SELECT 'T' AS orig, B.column_id AS attr_no, B.column_name, B.data_type_owner, B.data_type, NULL, B.data_length, B.data_precision, B.data_scale, NULL as coll_type, NULL AS index_by
  FROM all_tab_cols B
  WHERE INSTR(B.column_name, '$') = 0 AND
        B.virtual_column = 'NO' AND
        --B.hidden_column = 'NO' AND
        B.owner = COALESCE(:owner, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')) AND
        :package IS NULL AND
        B.table_name = REGEXP_REPLACE(:name, '%ROWTYPE$')
  ORDER BY 2, 3`
	)
	u := tt.m[name]
	logger.Debug("resolve", "t", t, "key", name, "tc", t.TypeCode, "u", u, "uIsValid", u.Valid(), "isColl", t.IsColl(), "t#", fmt.Sprintf("%#v", t))
	if u != nil && u.Valid() {
		*t = *u
		return t, nil
	}
	isColl := t.IsColl()
	qrys := []string{attrQry, collQry}
	if strings.HasSuffix(t.Name, "%ROWTYPE") {
		qrys = append(qrys[0:], tblQry)
	} else if isColl {
		qrys[0], qrys[1] = qrys[1], qrys[0]
	}
	// logger.Debug("resolve", "type", t, "isColl", isColl)
	var i int
	var qry string
	var errs []error
	for _, qry = range qrys {
		rows, err := tt.db.QueryContext(ctx, qry,
			sql.Named("owner", t.Owner),
			sql.Named("package", t.Package),
			sql.Named("name", t.Name))
		if err != nil {
			return t, fmt.Errorf("%s: %w", qry, err)
		}
		defer rows.Close()
		for rows.Next() {
			var orig, attrName, collType string
			var attrNo sql.NullInt32
			var s Type
			if err := rows.Scan(
				// SELECT B.attr_type_owner, B.attr_type_name, B.attr_type_package, B.length, B.precision, B.scale, NULL as coll_type, NULL AS index_by
				&orig, &attrNo, &attrName,
				&s.Owner, &s.Name, &s.Package,
				&s.Length, &s.Precision, &s.Scale,
				&collType, &s.IndexBy,
			); err != nil {
				return t, fmt.Errorf("%s: %w", qry, err)
			}
			if collType != "" && t.CollType == "" {
				t.CollType = collType
			}
			if t.CollType != "" && s.IndexBy != "" && !strings.Contains(s.IndexBy, "INTEGER") {
				logger.Warn("unsupported", "indexBy", s.IndexBy, "of", t)
				errs = append(errs, fmt.Errorf("%s[%s]: %w", t, s.IndexBy, ErrNotSupported))
				continue
			}
			if attrName != "" && len(t.Arguments) != 0 && t.Arguments[len(t.Arguments)-1].Name == attrName {
				continue
			}
			// if s.Owner != "" {
			// logger.Debug("resolve", "s", fmt.Sprintf("%#v", s), "coll", s.IsColl())
			// }
			p := &s
			if !p.Valid() {
				key := p.name(".")
				tt.m[key] = p
				if p, err = tt.get(ctx, key); err != nil {
					return t, err
				}
				logger.Debug("x", "get", key, "p", p)
			}
			i++
			if isColl {
				t.Elem = p
				break
			} else {
				t.Arguments = append(t.Arguments, Argument{Name: attrName, Type: p})
			}
		}
		if err := rows.Close(); err != nil {
			return t, err
		}
		if i != 0 {
			break
		}
	}
	logger.Debug("resolved", "type", t, "rows", i, "isColl", t.IsColl(), "elem", t.Elem)
	if !t.Valid() {
		return t, fmt.Errorf("could not resolve %s (%s owner=%q pkg=%q name=%q; rowcount=%d): %#v: %w",
			name, qry, t.Owner, t.Package, t.Name, i, t, errors.Join(errs...))
	}
	tt.m[name] = t

	return t, nil
}

var ErrNotSupported = errors.New("not supported")

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
	fmt.Fprintf(bw, "// %s\nmessage %s {\n\toption (oracall_object_type) = %q;\n", t.name("."), t.ProtoMessageName(), t.OraType())
	if t.Arguments != nil {
		var i int
		for _, a := range t.Arguments {
			name := a.Name
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
	if t.Name == "XMLTYPE" && (t.Owner == "PUBLIC" || t.Owner == "SYS") {
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

func (t Type) WriteOraToFrom(ctx context.Context, w io.Writer) error {
	return fmt.Errorf("not implemented")
}

var ProtoImports = protoImports{
	"google/protobuf/timestamp.proto",
	"google/protobuf/descriptor.proto",
}

type protoImport string

func (p protoImport) String() string {
	return fmt.Sprintf("import %q;\n", string(p))
}

type protoImports []protoImport

func (p protoImports) String() string {
	var buf strings.Builder
	for _, s := range p {
		buf.WriteString(s.String())
	}
	return buf.String()
}

const (
	MessageTypeExtension = 79396128
	FieldTypeExtension   = 79396128
)

// a number between 1 and 536,870,911 with the following restrictions:
// The given number must be unique among all fields for that message.
// Field numbers 19,000 to 19,999 are reserved for the Protocol Buffers implementation. The protocol buffer compiler will complain if you use one of these reserved field numbers in your message.
var ProtoExtends = protoExtends{
	`extend google.protobuf.MessageOptions {
  string oracall_object_type = 79396128;
}`,
	`extend google.protobuf.FieldOptions {
  string oracall_field_type = 79396128;
}`,
}

type protoExtends []string

func (p protoExtends) String() string {
	return strings.Join(p, "\n")
}
