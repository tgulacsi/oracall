// Copyright 2026
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package custom

import (
	"database/sql"
	"encoding/xml"
	"strings"
	"testing"
)

func TestNullableInt32SQL(t *testing.T) {
	for _, test := range []struct {
		name string
		in   *int32
	}{
		{name: "null"},
		{name: "zero", in: int32Ptr(0)},
		{name: "positive", in: int32Ptr(123)},
		{name: "negative", in: int32Ptr(-123)},
		{name: "minimum", in: int32Ptr(-2147483648)},
		{name: "maximum", in: int32Ptr(2147483647)},
	} {
		t.Run(test.name, func(t *testing.T) {
			gotSQL := ToSQLInt32(test.in)
			if (test.in == nil) != !gotSQL.Valid {
				t.Fatalf("validity mismatch: in=%v sql=%+v", test.in, gotSQL)
			}
			gotPtr := FromSQLInt32(gotSQL)
			if test.in == nil {
				if gotPtr != nil {
					t.Fatalf("NULL became %d", *gotPtr)
				}
			} else if gotPtr == nil || *gotPtr != *test.in {
				t.Fatalf("roundtrip mismatch: in=%v out=%v", test.in, gotPtr)
			}
		})
	}
	if got := FromSQLInt32(sql.NullInt32{Valid: false, Int32: 999}); got != nil {
		t.Fatal("invalid sql value should be nil")
	}
}

func int32Ptr(n int32) *int32 { return &n }

func TestSOAPInt32(t *testing.T) {
	type Envelope struct {
		XMLName xml.Name  `xml:"Envelope"`
		Count   SOAPInt32 `xml:"count"`
	}
	for _, test := range []struct {
		name string
		in   *int32
		want string
	}{
		{"null", nil, `xsi:nil="true"`},
		{"zero", int32Ptr(0), `<count>0</count>`},
		{"positive", int32Ptr(42), `<count>42</count>`},
		{"negative", int32Ptr(-7), `<count>-7</count>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload, err := xml.Marshal(Envelope{Count: SOAPInt32{Value: test.in}})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(payload), test.want) {
				t.Fatalf("xml: %s, expected %s", payload, test.want)
			}
			if test.in == nil && !strings.Contains(string(payload), `xmlns:xsi="`+xsiNamespace+`"`) {
				t.Fatalf("missing XSI namespace declaration: %s", payload)
			}
			var decoded Envelope
			if err := xml.Unmarshal(payload, &decoded); err != nil {
				t.Fatal(err)
			}
			if test.in == nil {
				if decoded.Count.Value != nil {
					t.Fatalf("NULL became %d", *decoded.Count.Value)
				}
			} else if decoded.Count.Value == nil || *decoded.Count.Value != *test.in {
				t.Fatalf("SOAP roundtrip: %s", payload)
			}
		})
	}
}

func TestSOAPInt32Validation(t *testing.T) {
	for _, input := range []string{
		`<count/>`,
		`<count>2147483648</count>`,
		`<count xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true">2</count>`,
		`<count xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="bogus"/>`,
	} {
		var n SOAPInt32
		if err := xml.Unmarshal([]byte(input), &n); err == nil {
			t.Errorf("wanted error for %s", input)
		}
	}
}
