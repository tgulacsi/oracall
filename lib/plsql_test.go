// Copyright 2015, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import (
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/kylelemons/godebug/diff"
)

func TestOne(t *testing.T) {
	logger = testr.New(t)
	for i, tc := range testCases {
		functions := tc.ParseCsv(t, i)
		got, _ := functions[0].PlsqlBlock("")
		d := diff.Diff(tc.PlSql, got)
		if d != "" {
			//FIXME(tgulacsi): this should be an error!
			t.Logf("plsql block diff:\n" + d)
			//fmt.Printf("GOT:\n", got)
		}
	}
}
