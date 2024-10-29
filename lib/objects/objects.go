// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects

import (
	"context"
	"database/sql"
	"fmt"
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
	Arguments                   map[string]*Type
}

func ReadTypes(ctx context.Context, db godror.Querier) (map[string]*Type, error) {
	logger := zlog.SFromContext(ctx)
	const qry = `SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') AS owner, A.type_name, NULL AS package_name, A.typecode FROM user_types A
UNION ALL
SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') AS owner, A.type_name, A.package_name, A.typecode FROM user_plsql_types A`
	rows, err := db.QueryContext(ctx, qry, godror.PrefetchCount(1025), godror.FetchArraySize(1024))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", qry, err)
	}
	defer rows.Close()
	// TODO T_UGYFEL%ROWTYPE
	types := make(map[string]*Type)
	for rows.Next() {
		var t Type
		if err := rows.Scan(&t.Owner, &t.Name, &t.Package, &t.TypeCode); err != nil {
			return types, fmt.Errorf("scan %s: %w", qry, err)
		}
		types[t.name()] = &t
	}
	if err = rows.Close(); err != nil {
		return types, err
	}
	logger.Debug("first round", "types", types)
	const collQry = `SELECT NULL AS attr_name, B.elem_type_owner, NULL AS elem_package_name, B.elem_type_name, B.length, B.precision, B.scale, B.coll_type, NULL AS index_by
  FROM user_coll_types B 
  WHERE B.type_name = :name
UNION ALL
SELECT NULL AS attr_name, B.elem_type_owner, B.elem_type_name, B.elem_type_package, B.length, B.precision, B.scale, B.coll_type, B.index_by
  FROM user_plsql_coll_types B 
  WHERE B.type_name = :name AND B.package_name = :package`
	const attrQry = `SELECT B.attr_name, B.attr_type_owner, NULL AS attr_package_name, B.attr_type_name, B.length, B.precision, B.scale, NULL as coll_type, NULL AS index_by
  FROM user_type_attrs B 
  WHERE B.type_name = :name
UNION ALL
SELECT B.attr_name, B.attr_type_owner, B.attr_type_name, B.attr_type_package, B.length, B.precision, B.scale, NULL as coll_type, NULL AS index_by
  FROM user_plsql_type_attrs B 
  WHERE B.package_name = :package AND B.type_name = :name`

	grp, grpCtx := errgroup.WithContext(ctx)
	grp.SetLimit(8)
	var mu sync.Mutex
	list := make([]*Type, 0, len(types))
	for _, t := range types {
		list = append(list, t)
	}
	for _, t := range list {
		t := t
		grp.Go(func() error {
			qry := attrQry
			isColl := t.isColl()
			if isColl {
				qry = collQry
			}
			rows, err := db.QueryContext(grpCtx, qry, sql.Named("package", t.Package), sql.Named("name", t.Name))
			if err != nil {
				return fmt.Errorf("%s: %w", qry, err)
			}
			defer rows.Close()
			for rows.Next() {
				var attrName string
				var s Type
				if err := rows.Scan(
					// SELECT B.attr_type_owner, B.attr_type_name, B.attr_type_package, B.length, B.precision, B.scale, NULL as coll_type, NULL AS index_by
					&attrName, &s.Owner, &s.Name, &s.Package, &s.Length, &s.Precision, &s.Scale, &s.CollType, &s.IndexBy,
				); err != nil {
					return fmt.Errorf("%s: %w", qry, err)
				}
				if isColl {
					t.Elem = &s
					if s.composite() {
						if s.Owner == "" {
							s.Owner = t.Owner
						}
						mu.Lock()
						types[s.name()] = t.Elem
						mu.Unlock()
						logger.Debug("coll", "elem", s)
					}
				} else {
					p := &s
					if t.Arguments == nil {
						t.Arguments = make(map[string]*Type)
					}
					t.Arguments[attrName] = p
					if s.composite() {
						if s.Owner == "" {
							s.Owner = t.Owner
						}
						mu.Lock()
						types[s.name()] = p
						mu.Unlock()
					}
				}
			}

			return rows.Close()
		})
	}
	err = grp.Wait()
	return types, err
}

func (t Type) name() string {
	name := t.Name
	if t.Package != "" {
		name = t.Package + "." + name
	}
	if t.Owner != "" {
		name = t.Owner + "." + name
	}
	return name
}
func (t Type) isColl() bool {
	switch t.TypeCode {
	case "COLLECTION", "TABLE", "VARYING ARRAY":
		return true
	default:
		return false
	}
}
func (t Type) String() string {
	if t.composite() {
		return fmt.Sprintf("%s.%s.%s[%s]{%s}", t.Owner, t.Package, t.Name, t.Elem, t.Arguments)
	}
	return t.Name
}
func (t Type) composite() bool {
	return t.Owner != "" || t.Package != ""
}
