// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestProtocGenOracall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "buf", "generate")
	cmd.Dir = "testdata"
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	cmd = exec.CommandContext(ctx, "go", "test")
	cmd.Dir = "testdata"
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}
