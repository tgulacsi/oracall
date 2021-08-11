// Copyright 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package custom_test

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tgulacsi/oracall/custom"
)

func TestReadAll(t *testing.T) {
	src := strings.Repeat("0123456789ABCDEF", 1024)
	b, err := custom.ReadAll(strings.NewReader(src), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != src {
		t.Fatalf("got %q, wanted %q", got, src)
	}

	b, err = custom.ReadAll(strings.NewReader(src), 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != src {
		t.Fatalf("got %q, wanted %q", got, src)
	}
	//runtime.SetFinalizer(&b, func(b *[]byte) { t.Logf("free %p", b) })
	b = nil
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	t.Log("GC didn't panic", b)
}
