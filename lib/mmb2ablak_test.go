// Copyright 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package oracall

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// tryGenSurface attempts to generate both the PL/SQL binding block and the
// protobuf schema for one function, recovering from panics (the PL/SQL path
// panics on unsupported argument shapes instead of returning an error). It
// reports whether generation succeeded and, if not, why.
func tryGenSurface(fn Function, seen map[string]struct{}) (ok bool, reason string) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
			reason = fmt.Sprintf("%v", r)
		}
	}()

	plsql, _ := fn.PlsqlBlock("")
	if plsql == "" {
		return false, "empty PL/SQL block"
	}

	var buf bytes.Buffer
	if err := fn.SaveProtobuf(&buf, seen); err != nil {
		return false, "protobuf: " + err.Error()
	}
	return true, ""
}

// TestMMB2ABLAKSurface is the Oracle-free test surface for the real
// DB_MMB2ABLAK package, driven by its USER_ARGUMENTS metadata
// (../testdata/db_mmb2ablak.csv). It feeds every procedure through PL/SQL and
// protobuf generation, asserting that all procedures whose argument shapes are
// supported today generate cleanly, and that exactly the two procedures with
// OBJECT-type arguments (the known, deferred gap) fail with "unknown simple
// type".
//
// This pins the generator's supported surface against a real, large package
// without a database. When the generator gains OBJECT support, the two
// documented gaps move into the supported set and this test must be updated.
func TestMMB2ABLAKSurface(t *testing.T) {
	const (
		wantTotal     = 85
		wantSupported = 83
	)

	fns, err := ParseCsvFile("../testdata/db_mmb2ablak.csv", nil)
	if err != nil {
		t.Fatalf("ParseCsvFile: %v", err)
	}
	if len(fns) != wantTotal {
		t.Fatalf("got %d functions, want %d", len(fns), wantTotal)
	}

	// The two procedures whose OBJECT-type arguments the generator cannot yet
	// bind. Everything else must generate cleanly.
	wantGaps := map[string]bool{
		"DB_mmb2ablak.kontakt_kut_akr_uf_tolt": true,
		"DB_mmb2ablak.wa_torolt_tetelek":       true,
	}

	var gotGaps []string
	supported := 0
	seen := make(map[string]struct{}, 16)
	for _, fn := range fns {
		ok, reason := tryGenSurface(fn, seen)
		if ok {
			supported++
			continue
		}
		gotGaps = append(gotGaps, fn.Name())
		t.Logf("GAP %s: %s", fn.Name(), reason)
		if !strings.Contains(reason, "unknown simple type") {
			t.Errorf("gap %s failed for an unexpected reason (want the OBJECT 'unknown simple type' gap): %s", fn.Name(), reason)
		}
	}

	if supported != wantSupported {
		t.Errorf("got %d supported procedures, want %d", supported, wantSupported)
	}
	if len(gotGaps) != len(wantGaps) {
		t.Errorf("got %d gaps %q, want %d %v", len(gotGaps), gotGaps, len(wantGaps), mapKeys(wantGaps))
	}
	for _, g := range gotGaps {
		if !wantGaps[g] {
			t.Errorf("unexpected gap %q (it should generate cleanly)", g)
		}
	}
	for g := range wantGaps {
		if !inSlice(gotGaps, g) {
			t.Errorf("expected gap %q did not fail", g)
		}
	}
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func inSlice(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
