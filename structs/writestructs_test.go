/*
Copyright 2015 Tamás Gulácsi

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

package structs

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteStruct(t *testing.T) {
	functions, err := ParseCsv(strings.NewReader(csvSource))
	if err != nil {
		t.Errorf("error parsing csv: %v", err)
		t.FailNow()
	}

	dn, err := ioutil.TempDir("", "structs-")
	if err != nil {
		t.Skipf("cannot create temp dir: %v", err)
		return
	}
	defer os.RemoveAll(dn)
	fh, err := os.Create(filepath.Join(dn + "main.go"))
	if err != nil {
		t.Skipf("cannot create temp file in %q: %v", dn, err)
		return
	}
	defer os.Remove(fh.Name())
	err = SaveFunctions(fh, functions, "main", false)
	if closeErr := fh.Close(); closeErr != nil {
		t.Errorf("Writing to %s: %v", fh.Name(), err)
	}
	if err != nil {
		t.Errorf("Saving functions: %v", err)
		t.FailNow()
	}
	cmd := exec.Command("go", "run", fh.Name())
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("go run %q: %v", fh.Name(), err)
		t.FailNow()
	}
}
