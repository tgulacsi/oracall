// Copyright 2019, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package custom

import (
	"bufio"
	"bytes"
	"database/sql/driver"
	"encoding"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	_ = json.Marshaler((*DateTime)(nil))
	_ = json.Unmarshaler((*DateTime)(nil))
	_ = encoding.TextMarshaler((*DateTime)(nil))
	_ = encoding.TextUnmarshaler((*DateTime)(nil))
	_ = xml.Marshaler((*DateTime)(nil))
	_ = xml.Unmarshaler((*DateTime)(nil))
	_ = proto.Message((*DateTime)(nil))
)

type DateTime struct {
	time.Time
}

func getWriter(enc *xml.Encoder) *bufio.Writer {
	rEnc := reflect.ValueOf(enc)
	rP := rEnc.Elem().FieldByName("p").Addr()
	return *(**bufio.Writer)(unsafe.Pointer(rP.Elem().FieldByName("Writer").UnsafeAddr()))
}

func (dt *DateTime) Format(layout string) string {
	if dt == nil {
		return ""
	}
	return dt.Time.Format(layout)
}
func (dt *DateTime) AppendFormat(b []byte, layout string) []byte {
	if dt == nil {
		return nil
	}
	return dt.Time.AppendFormat(b, layout)
}
func (dt *DateTime) Scan(src interface{}) error {
	if src == nil {
		dt.Time = time.Time{}
		return nil
	}
	t, ok := src.(time.Time)
	if !ok {
		return fmt.Errorf("cannot scan %T to DateTime", src)
	}
	dt.Time = t
	return nil
}
func (dt *DateTime) Value() (driver.Value, error) {
	if dt == nil {
		return nil, nil
	}
	return dt.Time, nil
}

func (dt *DateTime) MarshalXML(enc *xml.Encoder, start xml.StartElement) error {
	if dt != nil && !dt.IsZero() {
		return enc.EncodeElement(dt.Time.In(time.Local).Format(time.RFC3339), start)
	}
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
func (dt *DateTime) MarshalJSON() ([]byte, error) {
	if dt == nil || dt.IsZero() {
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
func (dt *DateTime) MarshalText() ([]byte, error) {
	if dt == nil || dt.IsZero() {
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
	layout := time.RFC3339
	if bytes.IndexByte(data, '.') >= 19 {
		layout = time.RFC3339Nano
	}
	if n < 10 {
		layout = "20060102"
	} else {
		if n > len(layout) {
			data = data[:len(layout)]
		} else if n < 4 {
			layout = layout[:4]
		} else {
			for _, i := range []int{4, 7, 10} {
				if n <= i {
					break
				}
				if data[i] != layout[i] {
					data[i] = layout[i]
				}
			}
			if bytes.IndexByte(data, '.') < 0 {
				layout = layout[:n]
			} else if _, err := time.ParseInLocation(layout, string(data), time.Local); err != nil && strings.HasSuffix(err.Error(), `"" as "Z07:00"`) {
				layout = strings.TrimSuffix(layout, "Z07:00")
			}
		}
	}
	var err error
	// Fractional seconds are handled implicitly by Parse.
	dt.Time, err = time.ParseInLocation(layout, string(data), time.Local)
	//log.Printf("s=%q time=%v err=%+v", data, dt.Time, err)
	if err != nil {
		return fmt.Errorf("ParseInLocation(%q, %q): %w", layout, string(data), err)
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

func (dt *DateTime) ProtoReflect() protoreflect.Message {
	return dt.Timestamp().ProtoReflect()
}
