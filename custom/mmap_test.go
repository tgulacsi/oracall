/*
Copyright 2021 Tamás Gulácsi

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
	t.Log("GC didn't panic")
}
