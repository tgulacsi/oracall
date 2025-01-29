// Copyright 2014, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package custom_test

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/tgulacsi/oracall/custom"
)

func TestDateTimeXML(t *testing.T) {
	var buf strings.Builder
	enc := xml.NewEncoder(&buf)
	st := xml.StartElement{Name: xml.Name{Local: "element"}}
	for _, tC := range []struct {
		Dt custom.DateTime
		S  string
	}{
		{
			Dt: custom.DateTime{},
			S:  `<element xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"></element>`,
		},
		{
			Dt: custom.DateTime{Time: time.Date(2023, 6, 30, 0, 0, 0, 0, time.Local)},
			S:  `<element>2023-06-30</element>`,
		},
		{
			Dt: custom.DateTime{Time: time.Date(2019, 10, 22, 16, 56, 32, 0, time.Local)},
			S:  `<element>2019-10-22T16:56:32+02:00</element>`,
		},
		{
			Dt: custom.DateTime{Time: time.Date(2023, 6, 30, 0, 0, 0, 0, time.Local)},
			S:  `<element>2023-06-30T00:00:00.000+02:00</element>`,
		},
		{
			Dt: custom.DateTime{Time: time.Date(2023, 6, 30, 0, 0, 0, 0, time.Local)},
			S:  `<element>2023-06-30+02:00</element>`,
		},
		{
			Dt: custom.DateTime{Time: time.Date(2023, 6, 30, 13, 31, 26, 0, time.Local)},
			S:  `<element>2023-06-30T13:31:26+0200</element>`,
		},
	} {
		//t.Log(tC)
		buf.Reset()
		if err := tC.Dt.MarshalXML(enc, st); err != nil {
			t.Fatalf("%v: %+v", tC.Dt, err)
		}
		if got := buf.String(); tC.S != got &&
			got != strings.Replace(tC.S, ":00.000+", ":00+", 1) &&
			tC.S != strings.Replace(got, "T00:00:00+", "+", 1) &&
			tC.S != strings.Replace(got, "T00:00:00+02:00", "", 1) &&
			tC.S != strings.Replace(got, "+02:00", "+0200", 1) {
			t.Errorf("%v: got %q wanted %q", tC.Dt, got, tC.S)
		}

		var dt custom.DateTime
		if err := xml.Unmarshal([]byte(tC.S), &dt); err != nil {
			t.Fatalf("%v: %+v", tC.S, err)
		}
		t.Log(dt)
		if !dt.Time.Equal(tC.Dt.Time) {
			t.Errorf("%v: got %q wanted %q", tC.S, dt, tC.Dt)
		}
	}
}
