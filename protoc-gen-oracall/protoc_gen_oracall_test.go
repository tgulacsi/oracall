// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	"bytes"
	"context"
	"flag"
	"os"
	"os/exec"
	"testing"
	"time"
)

var flagConnect = flag.String("connect", nvl(os.Getenv("ORACALL_DSN"), os.Getenv("BRUNO_ID")), "DSN to connect with")

func TestProtocGenOracall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "install", "-v")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%q: %+v", cmd.Args, err)
	}
	if _, err := exec.LookPath("buf"); err != nil {
		cmd = exec.CommandContext(ctx, "go", "install", "github.com/bufbuild/buf/cmd/buf@latest")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			t.Errorf("%q: %+v", cmd.Args, err)
		}
	}

	if *flagConnect == "" {
		cmd = exec.CommandContext(ctx, "sh", "-c", `. env.sh; echo "${ORACALL_DSN:-${BRUNO_ID:-}}"`)
		b, err := cmd.Output()
		if *flagConnect = string(bytes.TrimSpace(b)); *flagConnect == "" {
			t.Fatalf("%q: %+v", cmd.Args, err)
		}
	}

	cmd = exec.CommandContext(ctx, "buf", "generate")
	cmd.Dir = "testdata"
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%q: %+v", cmd.Args, err)
	}

	cmd = exec.CommandContext(ctx, "go", "test", "-connect="+*flagConnect)
	cmd.Dir = "testdata"
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%q: %+v", cmd.Args, err)
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
