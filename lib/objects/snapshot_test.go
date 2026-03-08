// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects_test

import (
	"bytes"
	"database/sql"
	"os"
	"testing"

	"github.com/tgulacsi/oracall/lib/objects"
)

func TestDumpRestore(t *testing.T) {
	db, err := sql.Open("godror", nvl(
		os.Getenv("ORACALL_DSN"), os.Getenv("BRUNO_OWNER_ID"),
	))
	if err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	var buf bytes.Buffer
	if err := objects.DumpDB(ctx, &buf, db); err != nil {
		t.Fatal(err)
	}
	types, procs, err := objects.LoadSnapshot(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(procs.Items) == 0 {
		t.Fatal("no procs")
	}
	names, err := types.Names(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no types")
	}
}
