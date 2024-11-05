// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects_test

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"flag"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	_ "github.com/godror/godror"
	"github.com/tgulacsi/oracall/lib/objects"
)

//go:generate go install github.com/bufbuild/buf/cmd/buf@latest

func TestReadTypes(t *testing.T) {
	logger := zlog.NewT(t).SLog()
	ctx := zlog.NewSContext(context.Background(), logger)
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	db, err := sql.Open("godror", os.Getenv("BRUNO_OWNER_ID"))
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

	var buf bytes.Buffer
	buf.WriteString(`
syntax = "proto3";

package objects;
option go_package = "github.com/tgulacsi/oracall/lib/objects/testdata";

` + objects.ProtoImports.String() + `
` + objects.ProtoExtends.String() + `
`)
	bw := bufio.NewWriter(&buf)
	for _, nm := range names {
		x, err := types.Get(ctx, nm)
		t.Logf("%s: %v", nm, x)
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
	os.WriteFile("testdata/x.proto", buf.Bytes(), 0664)
	os.WriteFile("testdata/buf.gen.yaml", []byte(`version: v2
plugins:
  - local: protoc-gen-go
    out: .
    opt:
      - paths=source_relative
`), 0640)

	cmd := exec.CommandContext(ctx,
		//"protoc", "-I.", "-I../../../../../google/protobuf/timestamp.proto", "--go_out=_test.go",
		"buf", "generate",
		"x.proto")
	cmd.Dir = "testdata"
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("%q\n%s\n%+v", cmd.Args, string(b), err)
	}
}
