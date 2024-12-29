// Copyright 2015, 2024 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"context"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/google/go-cmp/cmp"
)

func TestLex(t *testing.T) {
	// logger := zlog.NewT(t).SLog()
	ctx := context.Background()
	// ctx = zlog.NewSContext(ctx, logger)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	dis, err := os.ReadDir("testdata")
	if err != nil && len(dis) == 0 {
		t.Fatal(err)
	}
	for _, di := range dis {
		if !strings.HasSuffix(di.Name(), ".sql") {
			continue
		}
		b, err := os.ReadFile(filepath.Join("testdata", di.Name()))
		if err != nil {
			t.Errorf("%q: %+v", di.Name(), err)
		}
		nm := di.Name()
		t.Run(nm, func(t *testing.T) { testLexString(ctx, t, string(b)) })
	}

	logger := zlog.NewT(t).SLog()
	ctx = zlog.NewSContext(ctx, logger)
	for _, tC := range []struct {
		Text string
		Want []Item
	}{
		{Text: "'a'", Want: []Item{
			{typ: itemSep, val: "'"},
			{typ: itemString, val: "a"},
			{typ: itemSep, val: "'"},
			{typ: itemText, val: "\n"},
		}},
		{Text: "z/*+comment--'a'\n*/:='a';--'a'", Want: []Item{
			{typ: itemText, val: "z"},
			{typ: itemSep, val: "/*"},
			{typ: itemComment, val: "+comment--'a'\n"},
			{typ: itemSep, val: "*/"},
			{typ: itemText, val: ":="},
			{typ: itemSep, val: "'"},
			{typ: itemString, val: "a"},
			{typ: itemSep, val: "'"},
			{typ: itemText, val: ";"},
			{typ: itemSep, val: "--"},
			{typ: itemComment, val: "'a'"},
		}},
	} {
		t.Run(tC.Text, func(t *testing.T) {
			gotS := printItems(testLexString(ctx, t, tC.Text))
			wantS := printItems(iterItems(tC.Want))
			if d := cmp.Diff(gotS, wantS); d != "" {
				t.Logf("got %s,\n\twanted %s", gotS, wantS)
				t.Error(d)
			}
		})
	}
}

type itemsIter iter.Seq[Item]

func iterItems(items []Item) itemsIter {
	return func(yield func(Item) bool) {
		for _, it := range items {
			if !yield(it) {
				break
			}
		}
	}
}

func printItems(seq itemsIter) string {
	var buf strings.Builder
	for it := range seq {
		if it.typ == itemEOF {
			break
		}
		fmt.Fprintf(&buf, "<%[1]s>%[2]s</%[1]s>\n", it.typ.String(), it.String())
	}
	return buf.String()
}

func testLexString(ctx context.Context, t *testing.T, text string) itemsIter {
	l := Lex(ctx, text)
	return func(yield func(Item) bool) {
		l(func(it Item) bool {
			t.Log(it.typ, it.val)
			return yield(it)
		})
	}
}
