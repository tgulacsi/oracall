// Copyright 2015, 2024 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package source_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/tgulacsi/oracall/source"
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
		Want []source.Item
	}{
		{Text: "'a'", Want: []source.Item{
			source.NewItem(source.ItemSep, "'"),
			source.NewItem(source.ItemString, "a"),
			source.NewItem(source.ItemSep, "'"),
			source.NewItem(source.ItemText, "\n"),
		}},
		{Text: "z/*+comment--'a'\n*/:='a';--'a'", Want: []source.Item{
			source.NewItem(source.ItemText, "z"),
			source.NewItem(source.ItemSep, "/*"),
			source.NewItem(source.ItemComment, "+comment--'a'\n"),
			source.NewItem(source.ItemSep, "*/"),
			source.NewItem(source.ItemText, ":="),
			source.NewItem(source.ItemSep, "'"),
			source.NewItem(source.ItemString, "a"),
			source.NewItem(source.ItemSep, "'"),
			source.NewItem(source.ItemText, ";"),
			source.NewItem(source.ItemSep, "--"),
			source.NewItem(source.ItemComment, "'a'"),
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

func iterItems(items []source.Item) source.ItemsIter {
	return func(yield func(source.Item) bool) {
		for _, it := range items {
			if !yield(it) {
				break
			}
		}
	}
}

func printItems(seq source.ItemsIter) string {
	var buf strings.Builder
	for it := range seq {
		if it.Is(source.ItemEOF) {
			break
		}
		fmt.Fprintf(&buf, "<%[1]s>%[2]s</%[1]s>\n", it.Type().String(), it.String())
	}
	return buf.String()
}

func testLexString(ctx context.Context, t *testing.T, text string) source.ItemsIter {
	l := source.Lex(ctx, text)
	return func(yield func(source.Item) bool) {
		l(func(it source.Item) bool {
			t.Log(it)
			return yield(it)
		})
	}
}
