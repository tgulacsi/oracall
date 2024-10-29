// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	_ "github.com/godror/godror"
	"github.com/tgulacsi/oracall/lib/objects"
)

func TestReadTypes(t *testing.T) {
	logger := zlog.NewT(t).SLog()
	ctx := zlog.NewSContext(context.Background(), logger)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	db, err := sql.Open("godror", os.Getenv("BRUNO_OWNER_ID"))
	if err != nil {
		t.Fatal(err)
	}
	types, err := objects.ReadTypes(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("types:", types)
}
