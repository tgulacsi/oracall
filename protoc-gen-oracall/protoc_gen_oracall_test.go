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
	"strconv"
	"testing"
	"time"
)

var (
	flagRun     = flag.String("only", "", "run this")
	flagConnect = flag.String("connect", nvl(os.Getenv("ORACALL_DSN"), os.Getenv("BRUNO_ID")), "DSN to connect with")
)

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
		cmd = exec.CommandContext(ctx, "sh", "-c", `. ./env.sh; echo "${ORACALL_DSN:-${BRUNO_ID:-}}"`)
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

	cmd = exec.CommandContext(ctx, "go", "test", "-count=1", "-run="+*flagRun, "-connect="+*flagConnect, "-v="+strconv.FormatBool(testing.Verbose()))
	cmd.Dir = "testdata"
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	t.Cleanup(func() {
		exec.CommandContext(context.Background(),
			"killall", "-9", "testdata.test",
		).Run()
	})
	if err := cmd.Start(); err != nil {
		t.Fatalf("%q: %+v", cmd.Args, err)
	}
	go func() {
		<-ctx.Done()
		cmd.Process.Kill()
	}()
	t.Log(cmd.Args)
	if state, err := cmd.Process.Wait(); err != nil {
		t.Fatal(err)
	} else if state.ExitCode() != 0 {
		t.Fatal(state.ExitCode())
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
