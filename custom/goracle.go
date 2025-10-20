// Copyright 2017, 2021 Tamas Gulacsi
//
// SPDX-License-Identifier: Apache-2.0

package custom

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/godror/godror"
	"github.com/godror/knownpb/timestamppb"
)

type SQLExecer interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
}

var ZeroIsAlmostZero bool

type Number godror.Number

func (n *Number) Set(num godror.Number) {
	*n = Number(num)
}
func (n Number) Get() godror.Number {
	return godror.Number(n)
}

// Value returns a driver Value.
func (n Number) Value() (driver.Value, error) {
	return string(n), nil
}

// Scan assigns a value from a database driver.
func (n *Number) Scan(src interface{}) error {
	switch x := src.(type) {
	case Number:
		*n = Number(x)
	case string:
		*n = Number(x)
	case []byte:
		*n = Number(x)
	case int, int8, int16, int32, int64,
		uint, uint16, uint32, uint64:
		*n = Number(fmt.Sprintf("%d", x))
	case float32, float64:
		*n = Number(fmt.Sprintf("%f", x))
	}
	return nil
}

func NumbersFromStrings(s *[]string) *[]godror.Number {
	if s == nil {
		return nil
	}
	return (*[]godror.Number)(unsafe.Pointer(s))
}

const timeFormat = time.RFC3339

func ParseTime(t *time.Time, s string) error {
	n := len(s)
	if s == "" || n < 8 { //20060102
		*t = time.Time{}
		return nil
	}
	var i int
	if i = strings.IndexByte(s, 'T'); i < 0 {
		if i = strings.IndexByte(s, ' '); i < 0 {
			s = s + "T00:00:00"
		} else {
			s = s[:i] + "T" + s[i+1:]
		}
	}

	if n > len(timeFormat) {
		n = len(timeFormat)
	}
	var err error
	*t, err = time.ParseInLocation(timeFormat[:n], s, time.Local) // TODO(tgulacsi): more robust parser
	if err != nil {
		return fmt.Errorf("%s: %w", s, err)
	}
	return nil
}

type Lob struct {
	err error
	*godror.Lob
	data []byte
}

func (L *Lob) read() error {
	if L.err != nil {
		return L.err
	}
	if L.data == nil {
		L.data, L.err = io.ReadAll(L.Lob)
	}
	return L.err
}
func (L *Lob) Size() int {
	if L.read() != nil {
		return 0
	}
	return len(L.data)
}
func (L *Lob) Marshal() ([]byte, error) {
	err := L.read()
	return L.data, err
}
func (L *Lob) MarshalTo(p []byte) (int, error) {
	err := L.read()
	i := copy(p, L.data)
	return i, err
}
func (L *Lob) Unmarshal(p []byte) error {
	L.data = p
	return nil
}

// Value returns a driver Value.
func (L *Lob) Value() (driver.Value, error) {
	err := L.read()
	if L.Lob.IsClob {
		return string(L.data), err
	}
	return L.data, err
}

// Scan assigns a value from a database driver.
func (L *Lob) Scan(src interface{}) error {
	switch x := src.(type) {
	case Lob:
		L.data, L.err = io.ReadAll(L.Lob)
	case *Lob:
		L.data, L.err = io.ReadAll(L.Lob)
	case io.Reader:
		L.data, L.err = io.ReadAll(x)
	case []byte:
		L.data = x
	case string:
		L.data = []byte(x)
	default:
		return fmt.Errorf("unknown type %T", src)
	}
	return nil
}

func AsString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case Number:
		return string(x)
	case sql.NullString:
		return x.String
	case fmt.Stringer:
		return x.String()
	}
	return fmt.Sprintf("%v", v)
}

func AsFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	var result float64
	switch x := v.(type) {
	case float64:
		result = x
	case float32:
		result = float64(x)
	case int64:
		result = float64(x)
	case int32:
		result = float64(x)
	case sql.NullFloat64:
		result = x.Float64
	case string, godror.Number:
		var s string
		switch x := x.(type) {
		case string:
			s = x
		case godror.Number:
			s = string(x)
		}
		if s == "" {
			return 0
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			log.Printf("ERROR parsing %q as Float64: %v", s, err)
		}
		result = f

	default:
		log.Printf("WARN: unknown Float64 type %T", v)
		return 0
	}
	if ZeroIsAlmostZero && result == 0 {
		return math.SmallestNonzeroFloat64
	}
	return result
}
func AsInt32(v interface{}) int32 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int32:
		return x
	case uint32:
		return int32(x)
	case int64:
		return int32(x)
	case uint64:
		return int32(x)
	case float64:
		return int32(x)
	case float32:
		return int32(x)
	case sql.NullInt64:
		return int32(x.Int64)
	case string, godror.Number:
		var s string
		switch x := x.(type) {
		case string:
			s = x
		case godror.Number:
			s = string(x)
		}
		if s == "" {
			return 0
		}
		i, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			log.Printf("ERROR parsing %q as Int32: %v", s, err)
		}
		return int32(i)
	default:
		log.Printf("WARN: unknown Int32 type %T", v)
	}
	return 0
}
func AsInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case uint64:
		return int64(x)
	case int32:
		return int64(x)
	case uint32:
		return int64(x)
	case float64:
		return int64(x)
	case float32:
		return int64(x)
	case sql.NullInt64:
		return x.Int64
	case string, godror.Number:
		var s string
		switch x := x.(type) {
		case string:
			s = x
		case godror.Number:
			s = string(x)
		}
		if s == "" {
			return 0
		}
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			log.Printf("ERROR parsing %q as Int64: %v", s, err)
		}
		return i
	default:
		log.Printf("WARN: unknown Int64 type %T", v)
	}
	return 0
}
func AsUint64(v interface{}) uint64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case uint64:
		return x
	case int64:
		return uint64(x)
	case uint32:
		return uint64(x)
	case int32:
		return uint64(x)
	case float64:
		return uint64(x)
	case float32:
		return uint64(x)
	case sql.NullInt64:
		return uint64(x.Int64)
	case string, godror.Number:
		var s string
		switch x := x.(type) {
		case string:
			s = x
		case godror.Number:
			s = string(x)
		}
		if s == "" {
			return 0
		}
		i, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			log.Printf("ERROR parsing %q as Uint64: %v", s, err)
		}
		return i
	default:
		log.Printf("WARN: unknown Uint64 type %T", v)
	}
	return 0
}

func AsTimestamp(v interface{}) *timestamppb.Timestamp {
	if v == nil {
		return nil
	}
	switch d := v.(type) {
	case *timestamppb.Timestamp:
		return d
	case time.Time:
		return timestamppb.New(d)
	case *time.Time:
		if !d.IsZero() {
			return timestamppb.New(*d)
		}
	case DateTime:
		return timestamppb.New(d.Time)
	case *DateTime:
		if !d.IsZero() {
			return timestamppb.New(d.Time)
		}
	case string:
		var t time.Time
		_ = ParseTime(&t, d)
		return timestamppb.New(t)
	default:
		log.Printf("WARN: unknown Date type %T", v)
	}

	return nil
}
func AsDate(v interface{}) *DateTime {
	//log.Printf("AsDate(%[1]v %[1]T)", v)
	if v == nil {
		return new(DateTime)
	}
	switch d := v.(type) {
	case *time.Time:
		if d == nil {
			return new(DateTime)
		}
		return &DateTime{Time: *d}
	case *DateTime:
		if d == nil {
			return new(DateTime)
		}
		return d
	case *timestamppb.Timestamp:
		if d == nil {
			return new(DateTime)
		}
		return &DateTime{Time: d.AsTime()}
	}
	d := new(DateTime)
	switch x := v.(type) {
	case DateTime:
		*d = x
	case time.Time:
		d.Time = x
	case string:
		_ = ParseTime(&d.Time, x)
	default:
		log.Printf("WARN: unknown Date type %T", v)
	}

	return d
}

func AsTime(v interface{}) time.Time {
	if v == nil {
		return time.Time{}
	}
	switch x := v.(type) {
	case time.Time:
		return x
	case sql.NullTime:
		return x.Time
	case *timestamppb.Timestamp:
		return x.AsTime()
	default:
		return AsDate(v).Time
	}
}
