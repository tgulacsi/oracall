/*
Copyright 2019 Tamás Gulácsi

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
