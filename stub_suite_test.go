// Copyright 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installStubPackage compiles testdata/stub/stub.sql into the connected schema.
// The file contains multiple statements separated by a line containing only '/'.
func installStubPackage(t *testing.T) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "stub", "stub.sql"))
	if err != nil {
		t.Fatalf("read stub.sql: %v", err)
	}
	for i, stmt := range strings.Split(string(b), "\n/\n") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		typ := "PACKAGE"
		switch {
		case strings.Contains(stmt, "CREATE OR REPLACE PACKAGE BODY"):
			typ = "PACKAGE BODY"
		case !strings.Contains(stmt, "CREATE OR REPLACE PACKAGE"):
			typ = "" // plain DDL
		}
		db := getConnection(t)
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("install stub statement %d: %v", i, err)
		}
		if typ == "" {
			continue
		}
		rows, err := db.Query(
			`SELECT line||'/'||position||': '||text FROM user_errors WHERE type = :1 AND name = :2`,
			typ, "ORACALL_STUB")
		if err != nil {
			t.Fatalf("query user_errors: %v", err)
		}
		var errs []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				rows.Close()
				t.Fatalf("scan user_errors: %v", err)
			}
			errs = append(errs, s)
		}
		rows.Close()
		if len(errs) != 0 {
			t.Fatalf("stub package errors:\n%s", strings.Join(errs, "\n"))
		}
	}
}

func buildOracall(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "oracall")
	runCommand(t, "go", "build", "-o="+out, ".")
	return out
}

func generateAndBuildPattern(t *testing.T, oracallBin, pattern string) (outFn string) {
	t.Helper()
	runCommand(t, oracallBin, "call", "--connect="+dsn,
		"--out=./testdata/integration_test/generated_functions.go",
		"--pb-out=github.com/tgulacsi/oracall/testdata/integration_test/pb:pb",
		pattern)
	fh, err := os.CreateTemp("", "oracall-stub")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	outFn = fh.Name()
	fh.Close()
	os.Remove(outFn)
	runCommand(t, "go", "build", "-o="+outFn, "./testdata/integration_test")
	return outFn
}

// TestStubSurfaceE2E installs the dedicated stub package (testdata/stub/stub.sql)
// and exercises the generated callers for every supported shape. It needs an
// Oracle schema with CREATE PROCEDURE plus buf and the Go protoc plugins;
// it skips cleanly when any prerequisite is missing.
func TestStubSurfaceE2E(t *testing.T) {
	if dsn == "" {
		dsn = os.Getenv("GODROR_TEST_DSN")
		if dsn == "" {
			t.Skip("no ORACALL_DSN/GODROR_TEST_DSN")
		}
	}
	for _, p := range []string{"protoc-gen-go", "protoc-gen-go-grpc"} {
		if _, err := exec.LookPath(p); err != nil {
			t.Skipf("%s not in PATH", p)
		}
	}
	installStubPackage(t)
	outFn := generateAndBuildPattern(t, buildOracall(t), "ORACALL_STUB.%")
	for _, tc := range []struct{ name, in, want string }{
		{"f_char_echo", `{"txt":"hello"}`, "hello"},
		{"p_char_inout", `{"txt":"abraka"}`, "ABRAKA#"},
		{"f_num_add", `{"a":"1","b":"2"}`, "3"},
		{"f_num_sum", `{"nums":["1","2","3"]}`, "6"},
		{"f_rec_echo", `{"rec":{"num":"7","txt":"x","dt":"2024-01-02T05:00:00Z"}}`, "7;x;2024-01-02"},
		{"f_rec_tab_count", `{"tab":[{"num":"1"},{"num":"2"}]}`, "2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.TrimSpace(runTest(t, outFn, "-connect="+dsn, tc.name, tc.in))
			if !strings.Contains(got, tc.want) {
				t.Errorf("got %s, want substring %q", got, tc.want)
			}
		})
	}
}
