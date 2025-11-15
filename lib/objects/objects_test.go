// Copyright 2024, 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects_test

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	// "encoding/json/v2"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"sync"
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

func TestReadPackages(t *testing.T) {
	logger := zlog.NewT(t).SLog()
	ctx := zlog.NewSContext(context.Background(), logger)
	db, err := sql.Open("godror", nvl(
		os.Getenv("ORACALL_DSN"), os.Getenv("BRUNO_OWNER_ID"),
	))
	if err != nil {
		t.Fatal(err)
	}

	tt, err := objects.NewTypes(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	const qry = `SELECT object_name FROM user_objects
		WHERE object_type = 'PACKAGE' AND
			object_name IN (
 'DB_ANYR',
 'DB_CASCO_LEKER',
 'DB_PGW_WS',
 'DB_WEB_GDPR',
 'DB_KAR_VEZENYLO4', 'DB_KAR_ONLINE', 'DB_KAR_MMB',
 'DB_KUT_WS',
 'DB_MMB2ABLAK',
 'DB_SAM',
 'DB_SPOOLSYS3',
			'DB_WEB', 'DB_WEB_PORTAL', 'DB_UPORTAL'
		)`
	rows, err := db.QueryContext(ctx, qry)
	if err != nil {
		t.Fatalf("%s: %+v", err, qry)
	}
	defer rows.Close()
	pkgs := make([]string, 0, 16)
	for rows.Next() {
		var pkg string
		if err = rows.Scan(&pkg); err != nil {
			t.Fatalf("scan %s: %+v", qry, err)
		}
		pkgs = append(pkgs, pkg)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("%s: %+v", qry, err)
	}

	var wg sync.WaitGroup
	funcsCh := make(chan objects.Package, 1)
	for _, pkg := range pkgs {
		wg.Add(1)
		t.Run(pkg, func(t *testing.T) {
			defer wg.Done()
			t.Parallel()
			ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			P, err := tt.NewPackage(ctx, db, pkg)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					t.Skip(err)
				}
				t.Fatalf("%s: %+v", pkg, err)
			}
			funcsCh <- P
		})
	}
	t.Run("wait", func(t *testing.T) { t.Parallel(); wg.Wait(); close(funcsCh) })

	t.Run("funcs.proto", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		buf.WriteString(`
syntax = "proto3";

package objects;
option go_package = "github.com/tgulacsi/oracall/lib/objects/testdata";

` + objects.ProtoImports.String() + `
` + objects.ProtoExtends.String() + `
`)
		var svcs objects.Protobuf
		for pkg := range funcsCh {
			svcs.Services = append(svcs.Services, pkg.GenProto())
		}
		bw := bufio.NewWriter(&buf)
		svcs.Print(bw)
		bw.Flush()
		os.MkdirAll("testdata", 0755)
		os.WriteFile("testdata/funcs.proto", buf.Bytes(), 0664)
		if b, err := exec.CommandContext(ctx, "buf", "format", "-w", "testdata/funcs.proto").CombinedOutput(); err != nil {
			t.Fatalf("buf format: %s: %+v", string(b), err)
		}
	})
}

func TestReadTypes(t *testing.T) {
	logger := zlog.NewT(t).SLog()
	ctx := zlog.NewSContext(context.Background(), logger)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	db, err := sql.Open("godror",
		nvl(os.Getenv("ORACALL_DSN"), os.Getenv("BRUNO_OWNER_ID")))
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

	pkg := "testdata"
	var buf bytes.Buffer
	buf.WriteString(`
syntax = "proto3";
// edition = "2023";
// option features.(pb.go).api_level = API_OPAQUE;

package ` + pkg + `;
option go_package = "github.com/tgulacsi/oracall/lib/objects/testdata";

`)
	for _, nm := range names {
		_, err := types.Get(ctx, nm)
		if err != nil {
			if errors.Is(err, objects.ErrNotSupported) {
				t.Logf("%+v", err)
				continue
			} else {
				t.Fatalf("%s: %+v", nm, err)
			}
		}
	}
	bw := bufio.NewWriter(&buf)
	if err = types.WritePB(ctx, bw); err != nil {
		t.Fatal(err)
	}

	os.MkdirAll("testdata", 0755)
	os.WriteFile("testdata/funcs.proto", buf.Bytes(), 0664)
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
