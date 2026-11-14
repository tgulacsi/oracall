// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main_test

import (
	"bytes"
	"cmp"
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	flagRun     = flag.String("only", "", "run this")
	flagConnect = flag.String("connect", cmp.Or(os.Getenv("ORACALL_DSN"), os.Getenv("BRUNO_ID")), "DSN to connect with")
)

// toolEnv returns the process environment with GOBIN prepended to PATH, so
// the locally installed protoc plugins resolve for buf's `local:` plugins.
func toolEnv() []string {
	gobin := os.Getenv("GOBIN")
	if gobin == "" {
		if b, err := exec.Command("go", "env", "GOPATH").Output(); err == nil {
			gobin = filepath.Join(strings.TrimSpace(string(b)), "bin")
		}
	}
	env := os.Environ()
	for i, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			env[i] = "PATH=" + gobin + string(os.PathListSeparator) + strings.TrimPrefix(kv, "PATH=")
			break
		}
	}
	return env
}

func TestProtocGenOracall(t *testing.T) {
	if *flagConnect == "" {
		t.Skip("ORACALL_DSN/BRUNO_ID not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	// GOBIN first: buf's `local:` plugins (protoc-gen-go, protoc-gen-oracall)
	// and the installed tools must be on PATH.
	env := toolEnv()
	cmd := exec.CommandContext(ctx, "go", "install", "-v")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%q: %+v", cmd.Args, err)
	}

	if *flagConnect == "" {
		cmd = exec.CommandContext(ctx, "sh", "-c", `. ./env.sh; echo "${ORACALL_DSN:-${BRUNO_ID:-}}"`)
		b, err := cmd.Output()
		if *flagConnect = string(bytes.TrimSpace(b)); *flagConnect == "" {
			t.Fatalf("%q: %+v", cmd.Args, err)
		}
	}

	// `go tool buf` (the buf module tool) instead of a buf binary on PATH.
	cmd = exec.CommandContext(ctx, "go", "tool", "buf", "generate")
	cmd.Dir = "testdata"
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%q: %+v", cmd.Args, err)
	}

	cmd = exec.CommandContext(ctx, "go", "test", "-count=1", "-run="+*flagRun, "-connect="+*flagConnect, "-v="+strconv.FormatBool(testing.Verbose()))
	cmd.Dir = "testdata"
	cmd.Env = env
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
