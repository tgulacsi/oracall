// Copyright 2019, 2020 Tamás Gulácsi
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package custom

import (
	"bufio"
	"bytes"
	"encoding"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"reflect"
	"time"
	"unsafe"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	_ = json.Marshaler((*DateTime)(nil))
	_ = json.Unmarshaler((*DateTime)(nil))
	_ = encoding.TextMarshaler((*DateTime)(nil))
	_ = encoding.TextUnmarshaler((*DateTime)(nil))
	_ = xml.Marshaler((*DateTime)(nil))
	_ = xml.Unmarshaler((*DateTime)(nil))
)

type DateTime struct {
	Time time.Time
}

func getWriter(enc *xml.Encoder) *bufio.Writer {
	rEnc := reflect.ValueOf(enc)
	rP := rEnc.Elem().FieldByName("p").Addr()
	return *(**bufio.Writer)(unsafe.Pointer(rP.Elem().FieldByName("Writer").UnsafeAddr()))
}

func (dt DateTime) MarshalXML(enc *xml.Encoder, start xml.StartElement) error {
	if dt.IsZero() {
		start.Attr = append(start.Attr,
			xml.Attr{Name: xml.Name{Space: "http://www.w3.org/2001/XMLSchema-instance", Local: "nil"}, Value: "true"})

		bw := getWriter(enc)
		bw.Flush()
		old := *bw
		var buf bytes.Buffer
		*bw = *bufio.NewWriter(&buf)
		if err := enc.EncodeElement("", start); err != nil {
			return err
		}
		b := bytes.ReplaceAll(bytes.ReplaceAll(bytes.ReplaceAll(bytes.ReplaceAll(
			buf.Bytes(),
			[]byte("_XMLSchema-instance:"), []byte("xsi:")),
			[]byte("xmlns:_XMLSchema-instance="), []byte("xmlns:xsi=")),
			[]byte("XMLSchema-instance:"), []byte("xsi:")),
			[]byte("xmlns:XMLSchema-instance="), []byte("xmlns:xsi="))
		*bw = old
		bw.Write(b)
		return bw.Flush()
	}
	return enc.EncodeElement(dt.Time.In(time.Local).Format(time.RFC3339), start)
}
func (dt *DateTime) UnmarshalXML(dec *xml.Decoder, st xml.StartElement) error {
	var s string
	if err := dec.DecodeElement(&s, &st); err != nil {
		return err
	}
	return dt.UnmarshalText([]byte(s))
}

func (dt *DateTime) IsZero() (zero bool) {
	//defer func() { log.Printf("IsZero(%#v): %t", dt, zero) }()
	if dt == nil {
		return true
	}
	defer func() {
		if r := recover(); r != nil {
			zero = true
		}
	}()
	return dt.Time.IsZero()
}
func (dt DateTime) MarshalJSON() ([]byte, error) {
	if dt.IsZero() {
		return []byte(`""`), nil
	}
	return dt.Time.In(time.Local).MarshalJSON()
}
func (dt *DateTime) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte(`""`)) || bytes.Equal(data, []byte("null")) {
		dt.Time = time.Time{}
		return nil
	}
	return dt.UnmarshalText(data)
}

// MarshalText implements the encoding.TextMarshaler interface.
// The time is formatted in RFC 3339 format, with sub-second precision added if present.
func (dt DateTime) MarshalText() ([]byte, error) {
	if dt.IsZero() {
		return nil, nil
	}
	return dt.Time.In(time.Local).MarshalText()
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// The time is expected to be in RFC 3339 format.
func (dt *DateTime) UnmarshalText(data []byte) error {
	data = bytes.Trim(data, " \"")
	n := len(data)
	if n == 0 {
		dt.Time = time.Time{}
		//log.Println("time=")
		return nil
	}
	if n > len(time.RFC3339) {
		n = len(time.RFC3339)
	} else if n < 4 {
		n = 4
	} else if n > 10 && data[10] != time.RFC3339[10] {
		data[10] = time.RFC3339[10]
	}
	var err error
	// Fractional seconds are handled implicitly by Parse.
	dt.Time, err = time.ParseInLocation(time.RFC3339[:n], string(data), time.Local)
	//log.Printf("s=%q time=%v err=%+v", data, dt.Time, err)
	if err != nil {
		return fmt.Errorf("ParseInLocation(%q, %q): %w", time.RFC3339[:n], string(data), err)
	}
	return nil
}

func (dt *DateTime) Timestamp() *timestamppb.Timestamp {
	if dt.IsZero() {
		return nil
	}
	return timestamppb.New(dt.Time)
}
func (dt *DateTime) MarshalTo(dAtA []byte) (int, error) {
	if dt.IsZero() {
		return 0, nil
	}
	b, err := proto.MarshalOptions{}.MarshalAppend(dAtA[:0], dt.Timestamp())
	_ = dAtA[len(b)-1] // panic if buffer is too short
	return len(b), err
}
func (dt *DateTime) Marshal() (dAtA []byte, err error) {
	if dt.IsZero() {
		return nil, nil
	}
	return proto.Marshal(dt.Timestamp())
}
func (dt *DateTime) String() string {
	if dt.IsZero() {
		return ""
	}
	return dt.Time.In(time.Local).Format(time.RFC3339)
}

func (dt *DateTime) ProtoMessage() {}

func (dt *DateTime) ProtoSize() (n int) {
	if dt.IsZero() {
		return 0
	}
	return proto.Size(dt.Timestamp())
}
func (dt *DateTime) Reset() {
	if dt != nil {
		dt.Time = time.Time{}
	}
}
func (dt *DateTime) Size() (n int) {
	if dt.IsZero() {
		return 0
	}
	return proto.Size(dt.Timestamp())
}
func (dt *DateTime) Unmarshal(dAtA []byte) error {
	var ts timestamppb.Timestamp
	if err := proto.Unmarshal(dAtA, &ts); err != nil {
		return err
	}
	if ts.Seconds == 0 && ts.Nanos == 0 {
		dt.Time = time.Time{}
	} else {
		dt.Time = ts.AsTime()
	}
	return nil
}
