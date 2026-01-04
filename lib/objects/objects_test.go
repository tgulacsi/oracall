// Copyright 2024, 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects_test

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"slices"
	// "encoding/json/v2"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	_ "github.com/godror/godror"
	"github.com/google/renameio/v2"
	"github.com/tgulacsi/oracall/lib/objects"
)

//go:generate go install github.com/bufbuild/buf/cmd/buf@latest

var typesFn = filepath.Join("testdata", "funcs.types.json")

func TestReadTypes(t *testing.T) {
	logger := zlog.NewT(t).SLog()
	ctx := zlog.NewSContext(context.Background(), logger)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	db, err := sql.Open("godror", nvl(os.Getenv("ORACALL_DSN"), os.Getenv("BRUNO_OWNER_ID")))
	if err != nil {
		t.Fatal(err)
	}
	types, err := objects.NewTypes(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	names, err := types.Names(ctx, flag.Args()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(len(names))
	fh, err := renameio.NewPendingFile(typesFn, renameio.WithPermissions(0644))
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Cleanup()
	enc := jsontext.NewEncoder(fh, jsontext.WithIndent("  "))
	if err = json.MarshalEncode(enc, types,
		json.DefaultOptionsV2(), json.OmitZeroStructFields(true),
	); err != nil {
		t.Fatal(err)
	}
	if err = fh.CloseAtomicallyReplace(); err != nil {
		t.Fatal(err)
	}
}

func TestWriteProtobufSpec(t *testing.T) {
	logger := zlog.NewT(t).SLog()
	ctx := zlog.NewSContext(context.Background(), logger)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	fh, err := os.Open(typesFn)
	if err != nil {
		t.Skip(err)
	}
	defer fh.Close()
	var types objects.Types
	if err = json.UnmarshalRead(fh, &types); err != nil {
		t.Fatal(err)
	}
	fh.Close()
	names, err := types.Names(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	buf.WriteString(`
syntax = "proto3";
// edition = "2023";

package objects;
option go_package = "github.com/tgulacsi/oracall/lib/objects/testdata";

` + objects.ProtoImports.String() + `
` + objects.ProtoExtends.String() + `
`)
	bw := bufio.NewWriter(&buf)
	slices.Sort(names)
	for _, nm := range names {
		x, err := types.Get(ctx, nm)
		// t.Logf("%s: %v", nm, x)
		if err != nil {
			if errors.Is(err, objects.ErrNotSupported) {
				t.Logf("%+v", err)
				continue
			} else {
				t.Fatalf("%s: %+v", nm, err)
			}
		}
		if err = x.WriteProtobufMessageType(ctx, bw); err != nil {
			t.Fatal(nm, err)
		}
	}
	bw.Flush()
	os.MkdirAll("testdata", 0755)
	os.WriteFile("testdata/funcs.proto", buf.Bytes(), 0664)
	os.WriteFile("testdata/buf.gen.yaml", []byte(`version: v2
plugins:
  - remote: buf.build/grpc/go:v1.6.0
    out: .
    opt:
      - paths=source_relative
  - remote: buf.build/protocolbuffers/go:v1.36.11
    out: .
    opt:
      - paths=source_relative
  - remote: buf.build/community/mfridman-go-json:v1.5.0
    out: .
    opt:
      - paths=source_relative
  - remote: buf.build/community/planetscale-vtprotobuf:v0.6.0
    out: .
    opt:
      - paths=source_relative
`), 0640)

	cmd := exec.CommandContext(ctx,
		//"protoc", "-I.", "-I../../../../../google/protobuf/timestamp.proto", "--go_out=_test.go",
		"buf", "generate",
		"funcs.proto")
	cmd.Dir = "testdata"
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("%q\n%s\n%+v", cmd.Args, string(b), err)
	}
}

func nvl[T comparable](a T, b ...T) T {
	var z T
	if a != z {
		return a
	}
	for _, a := range b {
		if a != z {
			return a
		}
	}
	return a
}
