// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package objects_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/godror/godror"
)

// stubPackage is the package-level PL/SQL stub used by the Phase 2 research.
// It declares a record of simple types (SIMPLE_REC) and an associative array
// of them (SIMPLE_REC_TAB) plus an array of simple types (NUM_TAB).
const stubPackage = "ORACALL_STUB"

func stubDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("ORACALL_DSN")
	if dsn == "" {
		t.Skip("ORACALL_DSN not set")
	}
	return dsn
}

// installStub installs testdata/stub/stub.sql into the connected schema.
// The file holds several statements separated by a line containing only "/".
func installStub(t *testing.T, db *sql.DB) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "stub", "stub.sql"))
	if err != nil {
		t.Fatalf("read stub.sql: %v", err)
	}
	for _, stmt := range strings.Split(string(b), "\n/\n") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("install stub: %v", err)
		}
	}
}

// dataString renders a godror.Data value as text.
func dataString(d godror.Data) string {
	switch v := d.Get().(type) {
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// TestPLSQLRecordAsGodrorObject checks that a package-level PL/SQL record type
// is addressable as a godror object and that its attributes round-trip.
//
// This is the core feasibility assumption of PLAN.md Phase 2: Oracle Objects
// binding is not limited to schema-level SQL OBJECT types.
func TestPLSQLRecordAsGodrorObject(t *testing.T) {
	db, err := sql.Open("godror", stubDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	installStub(t, db)
	ctx := context.Background()

	ot, err := godror.GetObjectType(ctx, db, stubPackage+".SIMPLE_REC")
	if err != nil {
		t.Fatalf("GetObjectType(%s.SIMPLE_REC): %v", stubPackage, err)
	}
	if got := ot.FullName(); !strings.HasSuffix(got, stubPackage+".SIMPLE_REC") {
		t.Errorf("FullName() = %q, want suffix %q", got, stubPackage+".SIMPLE_REC")
	}
	obj, err := ot.NewObject()
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	defer obj.Close()

	var d godror.Data
	if err := d.Set(int64(42)); err != nil {
		t.Fatalf("Set(NUM): %v", err)
	}
	if err := obj.SetAttribute("NUM", &d); err != nil {
		t.Fatalf("SetAttribute(NUM): %v", err)
	}
	if err := d.Set("hello"); err != nil {
		t.Fatalf("Set(TXT): %v", err)
	}
	if err := obj.SetAttribute("TXT", &d); err != nil {
		t.Fatalf("SetAttribute(TXT): %v", err)
	}

	var back godror.Data
	if err := obj.GetAttribute(&back, "NUM"); err != nil {
		t.Fatalf("GetAttribute(NUM): %v", err)
	}
	if got := dataString(back); got != "42" {
		t.Errorf("NUM round-trip = %q, want %q", got, "42")
	}
	if err := obj.GetAttribute(&back, "TXT"); err != nil {
		t.Fatalf("GetAttribute(TXT): %v", err)
	}
	if got := dataString(back); got != "hello" {
		t.Errorf("TXT round-trip = %q, want %q", got, "hello")
	}
}

// TestPLSQLCollectionAsGodrorObject checks that a package-level associative
// array of simple types is addressable as a godror object collection and that
// elements round-trip.
func TestPLSQLCollectionAsGodrorObject(t *testing.T) {
	db, err := sql.Open("godror", stubDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	installStub(t, db)
	ctx := context.Background()

	ot, err := godror.GetObjectType(ctx, db, stubPackage+".NUM_TAB")
	if err != nil {
		t.Fatalf("GetObjectType(%s.NUM_TAB): %v", stubPackage, err)
	}
	cobj, err := ot.NewObject()
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	defer cobj.Close()
	coll := cobj.Collection()

	var e godror.Data
	if err := e.Set(int64(7)); err != nil {
		t.Fatalf("Set(element): %v", err)
	}
	if err := coll.AppendData(&e); err != nil {
		t.Fatalf("AppendData: %v", err)
	}
	i, err := coll.First()
	if err != nil {
		t.Fatalf("First: %v", err)
	}
	var it godror.Data
	if err := coll.GetItem(&it, i); err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if got := dataString(it); got != "7" {
		t.Errorf("element round-trip = %q, want %q", got, "7")
	}
}
