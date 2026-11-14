// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package objects_test

import (
	"strings"
	"testing"

	"github.com/tgulacsi/oracall/lib/objects"
)

func simpleType(name string) *objects.Type { return &objects.Type{Name: name} }

func recordType(owner, pkg, name string, args ...objects.Argument) *objects.Type {
	return &objects.Type{Owner: owner, Package: pkg, Name: name, Arguments: args}
}

func collType(name string, elem *objects.Type) *objects.Type {
	return &objects.Type{Name: name, TypeCode: "COLLECTION", Elem: elem}
}

func TestTypeShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		typ  *objects.Type
		want objects.Shape
	}{
		{"simple", simpleType("NUMBER"), objects.ShapeSimple},
		{"record", recordType("O", "P", "R", objects.Argument{Name: "N", Type: simpleType("NUMBER")}), objects.ShapeRecord},
		{"collection", collType("C", simpleType("NUMBER")), objects.ShapeCollection},
		{"unknown", &objects.Type{}, objects.ShapeUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.typ.ShapeOf(); got != tc.want {
				t.Errorf("ShapeOf() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestQualifiedName(t *testing.T) {
	for _, tc := range []struct {
		typ  *objects.Type
		want string
	}{
		{&objects.Type{Owner: "BRUNO", Package: "PKG", Name: "REC"}, "BRUNO.PKG.REC"},
		{&objects.Type{Owner: "BRUNO", Name: "REC"}, "BRUNO.REC"},
		{&objects.Type{Name: "NUMBER"}, "NUMBER"},
		{&objects.Type{}, ""},
	} {
		if got := tc.typ.QualifiedName(); got != tc.want {
			t.Errorf("QualifiedName() = %q, want %q", got, tc.want)
		}
	}
}

func TestGoTypeNameFromQualified(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"BRUNO.PKG.REC_TYP", "BrunoPkgRecTyp"},
		{"BRUNO.REC", "BrunoRec"},
		{"123ABC", "T123abc"},
		{"", ""},
	} {
		if got := objects.GoTypeNameFromQualified(tc.in); got != tc.want {
			t.Errorf("GoTypeNameFromQualified(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTypeRegistry(t *testing.T) {
	r := objects.NewTypeRegistry()
	a := r.NameOf("BRUNO.PKG.REC_TYP")
	if again := r.NameOf("BRUNO.PKG.REC_TYP"); again != a {
		t.Errorf("shared type not deduplicated: %q != %q", a, again)
	}
	// "A.B" and "A_B" sanitize to the same base, so the second must collide-resolve.
	first := r.NameOf("A.B")
	second := r.NameOf("A_B")
	if first == second {
		t.Errorf("collision not resolved: both %q", first)
	}
	if !strings.HasPrefix(second, first) {
		t.Errorf("collision name %q should extend %q", second, first)
	}
	if got := r.Registered(); len(got) != 3 || got[0] != "BRUNO.PKG.REC_TYP" {
		t.Errorf("Registered() = %v", got)
	}
}

func TestValidateShape(t *testing.T) {
	simpleRec := func() *objects.Type {
		return recordType("O", "P", "R",
			objects.Argument{Name: "N", Type: simpleType("NUMBER")},
			objects.Argument{Name: "T", Type: simpleType("VARCHAR2")})
	}
	ok := []struct {
		name string
		typ  *objects.Type
	}{
		{"simple", simpleType("NUMBER")},
		{"record_of_simple", simpleRec()},
		{"array_of_simple", collType("NUMS", simpleType("NUMBER"))},
		{"array_of_record", collType("TAB", simpleRec())},
	}
	for _, tc := range ok {
		t.Run("ok/"+tc.name, func(t *testing.T) {
			if err := tc.typ.ValidateShape(); err != nil {
				t.Errorf("ValidateShape() = %v, want nil", err)
			}
		})
	}

	bad := []struct {
		name string
		typ  *objects.Type
		want string
	}{
		{"array_of_array", collType("AA", collType("C", simpleType("NUMBER"))), "array of array"},
		{"array_in_record", recordType("O", "P", "R", objects.Argument{Name: "C", Type: collType("C", simpleType("NUMBER"))}), "array in record"},
		{"nested_record", recordType("O", "P", "R", objects.Argument{Name: "R2", Type: simpleRec()}), "nested record"},
		{"unknown", &objects.Type{}, "unsupported/unknown"},
		{"collection_without_elem", &objects.Type{Name: "C", TypeCode: "COLLECTION"}, "no element type"},
	}
	for _, tc := range bad {
		t.Run("bad/"+tc.name, func(t *testing.T) {
			err := tc.typ.ValidateShape()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("ValidateShape() = %v, want error containing %q", err, tc.want)
			}
		})
	}
}
