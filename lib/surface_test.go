// Copyright 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package oracall

import (
	"bytes"
	"strings"
	"testing"
)

// surfaceCSVHeader is the column order exported by lib/readcsv.go (and by
// `oracall call -dump`); every row below uses exactly these 18 columns.
const surfaceCSVHeader = "OBJECT_ID;SUBPROGRAM_ID;PACKAGE_NAME;OBJECT_NAME;DATA_LEVEL;POSITION;ARGUMENT_NAME;IN_OUT;DATA_TYPE;DATA_PRECISION;DATA_SCALE;CHARACTER_SET_NAME;PLS_TYPE;CHAR_LENGTH;TYPE_LINK;TYPE_OWNER;TYPE_NAME;TYPE_SUBNAME\n"

// TestSupportedShapes is the canonical, Oracle-free test surface for every
// argument shape the generator supports today. It feeds USER_ARGUMENTS-shaped
// CSV through parsing, PL/SQL block generation, and protobuf generation, so the
// supported surface is exercised consistently and without a database.
//
// Supported: simple scalars, arrays of simple types, records of simple types,
// and arrays of records of simple types. Deliberately not exercised (they are
// unsupported): array of array, array inside a record, nested records.
func TestSupportedShapes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		csv       string
		wantPlsql []string
	}{
		{
			name: "simple_scalars",
			csv: surfaceCSVHeader +
				"1;1;STUB;PROC;0;1;TXT;IN OUT;VARCHAR2;;;CHAR_CS;VARCHAR2;100;;;;\n" +
				"1;1;STUB;PROC;0;2;NUM;IN OUT;NUMBER;12;2;;NUMBER;0;;;;\n" +
				"1;1;STUB;PROC;0;3;DT;OUT;DATE;;;;DATE;0;;;;\n",
			wantPlsql: []string{"DECLARE", "BEGIN"},
		},
		{
			name: "array_of_simple",
			csv: surfaceCSVHeader +
				"1;1;STUB;PROC;0;1;NUMS;IN;PL/SQL TABLE;;;;;0;;;;\n" +
				"1;1;STUB;PROC;1;1;;IN;NUMBER;;;;NUMBER;0;;;;\n",
			wantPlsql: []string{"IS TABLE OF", "INDEX BY BINARY_INTEGER"},
		},
		{
			name: "record_of_simple",
			csv: surfaceCSVHeader +
				"1;1;STUB;PROC;0;1;REC;IN;PL/SQL RECORD;;;;;0;;;;\n" +
				"1;1;STUB;PROC;1;1;NUM;IN;NUMBER;12;2;;NUMBER;0;;;;\n" +
				"1;1;STUB;PROC;1;2;TXT;IN;VARCHAR2;;;CHAR_CS;VARCHAR2;100;;;;\n",
			wantPlsql: []string{"IS RECORD"},
		},
		{
			name: "array_of_record",
			csv: surfaceCSVHeader +
				"1;1;STUB;PROC;0;1;TAB;IN;PL/SQL TABLE;;;;;0;;;;\n" +
				"1;1;STUB;PROC;1;1;;IN;PL/SQL RECORD;;;;;0;;;;\n" +
				"1;1;STUB;PROC;2;1;NUM;IN;NUMBER;12;2;;NUMBER;0;;;;\n" +
				"1;1;STUB;PROC;2;2;TXT;IN;VARCHAR2;;;CHAR_CS;VARCHAR2;100;;;;\n",
			wantPlsql: []string{"IS TABLE OF"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fns, err := ParseCsv(strings.NewReader(tc.csv), nil)
			if err != nil {
				t.Fatalf("ParseCsv: %v", err)
			}
			if len(fns) != 1 {
				t.Fatalf("got %d functions, want 1", len(fns))
			}
			plsql, _ := fns[0].PlsqlBlock("")
			t.Logf("PL/SQL block:\n%s", plsql)
			for _, want := range tc.wantPlsql {
				if !strings.Contains(plsql, want) {
					t.Errorf("PL/SQL block missing %q", want)
				}
			}

			var proto bytes.Buffer
			if err := SaveProtobuf(t.Context(), &proto, fns, "pb", "stub/pb"); err != nil {
				t.Fatalf("SaveProtobuf: %v", err)
			}
			if !strings.Contains(proto.String(), "message ") {
				t.Errorf("protobuf output has no message:\n%s", proto.String())
			}
		})
	}
}
