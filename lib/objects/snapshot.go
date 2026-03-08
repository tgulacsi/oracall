// Copyright 2024, 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package objects

import (
	"context"
	"io"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

// Snapshot holds the Oracle metadata needed by the code generator.
type Snapshot struct {
	Types      *Types      `json:"types"`
	Procedures *Procedures `json:"procedures"`
}

// DumpDB queries Oracle dictionary views (all_arguments, all_plsql_types,
// all_plsql_type_attrs, all_plsql_coll_types, …) via the existing NewTypes /
// Names / ReadProcedures functions, then writes a JSON snapshot to w.
//
// funcNames optionally restricts which procedures are included (all types are
// always fetched because the generator needs cross-references).
func DumpDB(ctx context.Context, w io.Writer, db querier, funcNames ...string) error {
	types, err := NewTypes(ctx, db)
	if err != nil {
		return err
	}
	if _, err = types.Names(ctx); err != nil {
		return err
	}
	procs, err := ReadProcedures(ctx, db, funcNames...)
	if err != nil {
		return err
	}
	enc := jsontext.NewEncoder(w, jsontext.WithIndent("  "))
	return json.MarshalEncode(enc,
		Snapshot{Types: types, Procedures: procs},
		json.DefaultOptionsV2(),
		json.OmitZeroStructFields(true),
	)
}

// LoadSnapshot reads a JSON snapshot (produced by DumpDB) and returns the
// populated *Types and *Procedures ready for code generation.
// The *Types has no DB connection after loading; it serves from its cache.
func LoadSnapshot(r io.Reader) (*Types, *Procedures, error) {
	var snap Snapshot
	if err := json.UnmarshalRead(r, &snap); err != nil {
		return nil, nil, err
	}
	return snap.Types, snap.Procedures, nil
}
