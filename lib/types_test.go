/*
Copyright 2017 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
