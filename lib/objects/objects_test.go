// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects_test

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"flag"
	"os"
	"os/exec"
	"sort"
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
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	db, err := sql.Open("godror", os.Getenv("BRUNO_OWNER_ID"))
	if err != nil {
		t.Fatal(err)
	}
	types, err := objects.ReadTypes(ctx, db, flag.Args()...)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(types))
	for k := range types {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, nm := range names {
		t.Logf("%s: %v", nm, types[nm])
	}

	var buf bytes.Buffer
	buf.WriteString(`
syntax = "proto3";

package objects;
option go_package = "objects";

import "google/protobuf/timestamp.proto";

`)
	bw := bufio.NewWriter(&buf)
	for _, nm := range names {
		if err = types[nm].WriteProtobufMessageType(ctx, bw); err != nil {
			t.Fatal(nm, err)
		}
	}
	bw.Flush()
	os.WriteFile("_test.proto", buf.Bytes(), 0664)

	cmd := exec.CommandContext(ctx,
		//"protoc", "-I.", "-I../../../../../google/protobuf/timestamp.proto", "--go_out=_test.go",
		"buf", "build",
		"_test.proto")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("%q\n%s\n%+v", cmd.Args, string(b), err)
	}
}
