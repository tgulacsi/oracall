// Copyright 2015, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tgulacsi/go/loghlp/kitloghlp"
)

var flagKeep = flag.Bool("keep", false, "keep temp files")

func TestWriteStruct(t *testing.T) {
	Log = kitloghlp.NewTestLogger(t).Log
	var (
		dn, fn string
		keep   = *flagKeep
		err    error
	)
	for i, tc := range testCases {
		functions := tc.ParseCsv(t, i)

		if dn == "" {
			if dn, err = ioutil.TempDir("", "structs-"); err != nil {
				t.Skipf("cannot create temp dir: %v", err)
				return
			}
			defer func() {
				if !keep {
					os.RemoveAll(dn)
				}
			}()
		}
		if !keep && fn != "" {
			_ = os.Remove(fn)
		}
		fn = filepath.Join(dn, fmt.Sprintf("main-%d.go", i))
		defer func() {
			if !keep {
				os.Remove(fn)
			}
		}()
		fh, err := os.Create(fn)
		if err != nil {
			t.Skipf("cannot create temp file in %q: %v", dn, err)
			return
		}
		err = SaveFunctions(fh, functions, "main", "unosoft.hu/ws/bruno/pb", true, "")
		if err != nil {
			_ = fh.Close()
			t.Errorf("%d. Saving functions: %v", i, err)
			t.FailNow()
		}
		if _, err = io.WriteString(fh, "\nfunc main() {}\n"); err != nil {
			t.Errorf("%d. append main: %v", i, err)
		}
		if err = fh.Close(); err != nil {
			t.Errorf("%d. Writing to %s: %v", i, fh.Name(), err)
		}
		cmd := exec.Command("go", "run", fh.Name())
		var errBuf bytes.Buffer
		cmd.Stderr = &errBuf
		if err := cmd.Run(); err != nil {
			keep = true
			t.Errorf("%d. go run %q: %v\n%s", i, fh.Name(), err, errBuf.String())
			t.FailNow()
		}
	}
}

func TestGoName(t *testing.T) {
	for eltNum, elt := range [][2]string{
		{"a", "A"},
		{"a_b", "AB"},
		{"a__b", "A_B"},
		{"db_web__calculate_31101__input", "DbWeb_Calculate_31101_Input"},
		{"db_web__calculate_242xx__output", "DbWeb_Calculate_242Xx_Output"},
		{"Db_dealer__zaradek_rec_typ__bruno", "DbDealer_ZaradekRecTyp_Bruno"},
		{"*Db_dealer__zaradek_rec_typ__bruno", "*DbDealer_ZaradekRecTyp_Bruno"},
	} {
		if got := CamelCase(elt[0]); got != elt[1] {
			t.Errorf("%d. %q => got %q, awaited %q.", eltNum, elt[0], got, elt[1])
		}
	}
}
