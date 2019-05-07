// Copyright 2019 Tamás Gulácsi
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
	"encoding/xml"
	"strings"
	"time"

	"github.com/gogo/protobuf/types"
)

var _ = xml.Unmarshaler((*DateTime)(nil))
var _ = xml.Marshaler(DateTime{})

type DateTime struct {
	time.Time
}

func (dt *DateTime) UnmarshalXML(dec *xml.Decoder, st xml.StartElement) error {
	var s string
	if err := dec.DecodeElement(&s, &st); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	n := len(s)
	if n == 0 {
		dt.Time = time.Time{}
		//log.Println("time=")
		return nil
	}
	if n > len(time.RFC3339) {
		n = len(time.RFC3339)
	} else if n < 4 {
		n = 4
	}
	var err error
	dt.Time, err = time.ParseInLocation(time.RFC3339[:n], s, time.Local)
	//log.Printf("s=%q time=%v err=%+v", s, dt.Time, err)
	return err
}
func (dt DateTime) MarshalXML(enc *xml.Encoder, start xml.StartElement) error {
	//fmt.Printf("Marshal %v: %v\n", start.Name.Local, dt.Time.Format(time.RFC3339))
	return enc.EncodeElement(dt.Time.In(time.Local).Format(time.RFC3339), start)
}
func (dt DateTime) Timestamp() *types.Timestamp {
	ts, err := types.TimestampProto(dt.Time)
	if err != nil {
		//fmt.Printf("ERROR: %+v\n", err)
	}
	return ts
}
func (dt DateTime) MarshalTo(dAtA []byte) (int, error) {
	return dt.Timestamp().MarshalTo(dAtA)
}
func (dt DateTime) Marshal() (dAtA []byte, err error) {
	return dt.Timestamp().Marshal()
}
func (dt DateTime) String() string { return dt.Time.In(time.Local).Format(time.RFC3339) }
func (DateTime) ProtoMessage()     {}
func (dt DateTime) ProtoSize() (n int) {
	return dt.Timestamp().ProtoSize()
}
func (dt *DateTime) Reset()       { dt.Time = time.Time{} }
func (dt DateTime) Size() (n int) { return dt.Timestamp().Size() }
func (dt *DateTime) Unmarshal(dAtA []byte) error {
	var ts types.Timestamp
	err := ts.Unmarshal(dAtA)
	if err != nil {
		dt.Time = time.Time{}
		return err
	}
	dt.Time, err = types.TimestampFromProto(&ts)
	return err
}
