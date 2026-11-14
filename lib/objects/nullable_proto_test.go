// Copyright 2026 Tamás Gulácsi.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package objects

import (
	"context"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/tgulacsi/oracall/lib"
)

func TestNullablePLSIntegerObject(t *testing.T) {
	prev := oracall.NullablePLSInteger
	oracall.NullablePLSInteger = true
	t.Cleanup(func() { oracall.NullablePLSInteger = prev })
	typ := Type{Owner: "TEST", Name: "MY_RECORD", TypeCode: "OBJECT", Arguments: []Argument{
		{Name: "COUNT", Type: &Type{Name: "PL/SQL PLS INTEGER"}},
	}}
	var b strings.Builder
	if err := typ.WriteProtobufMessageType(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "optional int32 count = 1") {
		t.Fatalf("missing optional PLS_INTEGER: %s", b.String())
	}
	b.Reset()
	if err := typ.WriteOraToFrom(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	output := b.String()
	if !strings.Contains(output, "if x.Count == nil") || !strings.Contains(output, "if data.IsNull()") {
		t.Errorf("missing NULL guards: %s", output)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "generated.go", "package generated\n"+output, parser.AllErrors); err != nil {
		t.Errorf("generated Go syntax: %v", err)
	}
}

func TestNullablePLSIntegerProcedure(t *testing.T) {
	prev := oracall.NullablePLSInteger
	oracall.NullablePLSInteger = true
	t.Cleanup(func() { oracall.NullablePLSInteger = prev })
	p := Procedure{Package: "TEST_PKG", Name: "TEST_PROC", Parameters: []ProcParameter{
		{Name: "P_IN", Direction: DirIn, OraType: "PLS_INTEGER"},
		{Name: "P_OUT", Direction: DirOut, OraType: "PL/SQL PLS INTEGER"},
		{Name: "P_IN_OUT", Direction: DirInOut, OraType: "BINARY_INTEGER"},
	}}
	var b strings.Builder
	if err := p.WriteProcedureWrapper(context.Background(), &b, nil); err != nil {
		t.Fatal(err)
	}
	src := b.String()
	for _, want := range []string{
		"PIn *int32", "POut *int32", "PInOut *int32",
		"sql.NullInt32", "custom.ToSQLInt32(input.PIn)",
		"custom.FromSQLInt32(p_out)", "custom.FromSQLInt32(p_in_out)",
		"sql.Out{Dest: &p_in_out, In: true}",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("missing %q in generated code:\n%s", want, src)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "generated.go", "package generated\n"+src, parser.AllErrors); err != nil {
		t.Errorf("generated code has invalid syntax: %v", err)
	}
}

func TestNullablePLSIntegerObjectDefaultUnchanged(t *testing.T) {
	prev := oracall.NullablePLSInteger
	oracall.NullablePLSInteger = false
	t.Cleanup(func() { oracall.NullablePLSInteger = prev })

	typ := Type{Owner: "TEST", Name: "MY_RECORD", TypeCode: "OBJECT", Arguments: []Argument{
		{Name: "COUNT", Type: &Type{Name: "PL/SQL PLS INTEGER"}},
	}}
	var b strings.Builder
	if err := typ.WriteProtobufMessageType(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "optional int32 count") || !strings.Contains(b.String(), "int32 count = 1") {
		t.Errorf("opt-out must preserve proto type: %s", b.String())
	}
	b.Reset()
	if err := typ.WriteOraToFrom(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "if x.Count == nil") || strings.Contains(b.String(), "if data.IsNull()") {
		t.Errorf("opt-out must preserve legacy object converter: %s", b.String())
	}

	p := Procedure{Package: "TEST_PKG", Name: "TEST_PROC", Parameters: []ProcParameter{
		{Name: "P_IN", Direction: DirIn, OraType: "PLS_INTEGER"},
	}}
	b.Reset()
	if err := p.WriteProcedureWrapper(context.Background(), &b, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "PIn int32") || strings.Contains(b.String(), "custom.ToSQLInt32") {
		t.Errorf("opt-out must preserve legacy wrapper: %s", b.String())
	}
}
