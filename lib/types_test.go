// Copyright 2017, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import "testing"

func TestParseDigits(t *testing.T) {
	for tN, tC := range []struct {
		In          string
		Prec, Scale int
		WantErr     bool
	}{
		{In: ""},
		{In: "0", Prec: 32, Scale: 4},
		{In: "-12.3", Prec: 3, Scale: 1},
		{In: "12.34", Prec: 3, Scale: 1, WantErr: true},
	} {
		if err := ParseDigits(tC.In, tC.Prec, tC.Scale); err == nil && tC.WantErr {
			t.Errorf("%d. wanted error for %q", tN, tC.In)
		} else if err != nil && !tC.WantErr {
			t.Errorf("%d. got error %+v for %q", tN, err, tC.In)
		}
	}
}
