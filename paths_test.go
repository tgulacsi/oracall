// Copyright 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportPathFor(t *testing.T) {
	gopath, err := os.MkdirTemp("", "gopath")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(gopath)

	modDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modDir, "go.mod"),
		[]byte("module example.com/foo\n\ngo 1.27.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(modDir, "testdata", "pb")
	if err := os.MkdirAll(sub, 0775); err != nil {
		t.Fatal(err)
	}
	if got, ok := importPathFor(sub, ""); !ok || got != "example.com/foo/testdata/pb" {
		t.Errorf("importPathFor(sub) = %q, %v; want example.com/foo/testdata/pb, true", got, ok)
	}
	if got, ok := importPathFor(modDir, ""); !ok || got != "example.com/foo" {
		t.Errorf("importPathFor(modDir) = %q, %v; want example.com/foo, true", got, ok)
	}

	// no module anywhere: nonexistent dir -> not ok
	if _, ok := importPathFor(filepath.Join(modDir, "nonexistent"), ""); ok {
		t.Errorf("importPathFor(nonexistent) should be !ok")
	}
	// GOPATH fallback: dir under $GOPATH/src
	gpSub := filepath.Join(gopath, "src", "example.org/bar")
	if err := os.MkdirAll(gpSub, 0775); err != nil {
		t.Fatal(err)
	}
	if got, ok := importPathFor(gpSub, gopath); !ok || got != "example.org/bar" {
		t.Errorf("importPathFor(gpSub) = %q, %v; want example.org/bar, true", got, ok)
	}
	// dir outside both module and GOPATH -> not ok
	outside := t.TempDir()
	if got, ok := importPathFor(outside, gopath); ok {
		t.Errorf("importPathFor(outside) = %q, %v; want !ok", got, ok)
	}
}
