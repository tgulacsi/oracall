// Copyright 2019, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
)

func TestSplitDoc(t *testing.T) {
	fh, err := os.Open("testdata/split_doc.json")
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	dec := json.NewDecoder(fh)
	var elt struct{ Name, Documentation string }
	for {
		if err = dec.Decode(&elt); err != nil {
			if !errors.Is(err, io.EOF) {
				t.Fatal(err)
			}
			break
		}
		common, input, output := splitDoc(elt.Documentation)
		t.Logf("%s: [%q, %q, %q]", elt.Name, common, input, output)
		parts := splitByOffset(input)
		t.Log(len(parts), parts)

		D := getDirDoc(elt.Documentation, DIR_IN)
		t.Log(D)
	}
}
