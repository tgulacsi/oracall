// Copyright 2014, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package custom_test

import (
	"bytes"
	"encoding/xml"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tgulacsi/oracall/custom"
	"github.com/tgulacsi/oracall/custom/testdata/pb"
	"google.golang.org/protobuf/proto"
)

//go:generate go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//go:generate go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
//go:generate sh -c "set -x; protoc -I. -I$(go env GOPATH)/src --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative ./testdata/pb/ts.proto"
//go:generate sh -c "./replace_timestamp.sh testdata/pb/*.go"

func TestTimestampMarshalXML(t *testing.T) {
	var buf strings.Builder
	enc := xml.NewEncoder(&buf)
	st := xml.StartElement{Name: xml.Name{Local: "element"}}
	for _, tC := range []struct {
		In   *custom.Timestamp
		Want string
	}{
		{In: nil, Want: `<element xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"></element>`},
		{In: custom.NewTimestamp(time.Date(2019, 10, 22, 16, 56, 32, 0, time.Local)), Want: `<element>2019-10-22T16:56:32+02:00</element>`},
	} {
		buf.Reset()
		if err := tC.In.MarshalXML(enc, st); err != nil {
			t.Fatalf("%v: %+v", tC.In, err)
		}
		if got := buf.String(); tC.Want != got {
			t.Errorf("%v: got %q wanted %q", tC.In, got, tC.Want)
		}
	}
}

func TestTimestampProto(t *testing.T) {
	var ts pb.Ts
	ts.Reset()
	ts.Ts = custom.NewTimestamp(time.Now())
	ts.Tss = []*custom.Timestamp{custom.NewTimestamp(time.Now()), nil, custom.NewTimestamp(time.Now())}
	t.Log("ts:", ts.String())
	b, err := proto.Marshal(&ts)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("b:", b)
	var ts2 pb.Ts
	if err = proto.Unmarshal(b, &ts2); err != nil {
		t.Fatal(err)
	}
	t.Log("ts2:", &ts2)
	if d := cmp.Diff(ts2.Ts, ts.Ts,
		cmp.Comparer(func(a, b *custom.Timestamp) bool { return a.AsTime().Equal(b.AsTime()) }),
		cmp.Exporter(func(t reflect.Type) bool { return false }),
		cmpopts.IgnoreUnexported(),
	); d != "" {
		t.Errorf("marshal-unmarshal result differs: %s", d)
	}
	if d := cmp.Diff(ts2.Tss, ts.Tss,
		cmp.Comparer(func(a, b *custom.Timestamp) bool { return a.AsTime().Equal(b.AsTime()) }),
		cmp.Exporter(func(t reflect.Type) bool { return false }),
		cmpopts.IgnoreUnexported(),
	); d != "" {
		t.Errorf("marshal-unmarshal result differs: %s", d)
	}
	if b, err = xml.Marshal(&ts2); err != nil {
		t.Fatal(err)
	}
	t.Log(string(b))
	if !bytes.Contains(b, []byte("nil")) {
		t.Errorf("no nil in %q", string(b))
	}
}
