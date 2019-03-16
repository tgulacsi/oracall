package custom

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/golang/protobuf/ptypes"
	tspb "github.com/golang/protobuf/ptypes/timestamp"

	"github.com/pkg/errors"
	"gopkg.in/goracle.v2"
)

var ZeroIsAlmostZero bool

type Number goracle.Number

func (n *Number) Set(num goracle.Number) {
	*n = Number(num)
}
func (n Number) Get() goracle.Number {
	return goracle.Number(n)
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

func NumbersFromStrings(s *[]string) *[]goracle.Number {
	if s == nil {
		return nil
	}
	return (*[]goracle.Number)(unsafe.Pointer(s))
}

type Date tspb.Timestamp

const timeFormat = time.RFC3339 //"2006-01-02 15:04:05 -0700"

func NewDate(t time.Time) Date {
	var d Date
	d.Set(t)
	return d
}
func (d *Date) Set(t time.Time) {
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(errors.Wrap(err, t.String()))
	}
	*d = Date(*ts)
}
func (d Date) Get() (od time.Time, err error) {
	return ptypes.Timestamp((*tspb.Timestamp)(&d))
}

// Value returns a driver Value.
func (d Date) Value() (driver.Value, error) {
	return d.Get()
}

func (d Date) String() string { return ptypes.TimestampString((*tspb.Timestamp)(&d)) }
func (d *Date) SetString(s string) error {
	if s == "" {
		((*tspb.Timestamp)(d)).Reset()
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

	n := len(s)
	if n > len(timeFormat) {
		n = len(timeFormat)
	}
	t, err := time.Parse(timeFormat[:n], s) // TODO(tgulacsi): more robust parser
	if err != nil {
		return errors.Wrap(err, s)
	}
	d.Set(t)
	return nil

}

// Scan assigns a value from a database driver.
func (d *Date) Scan(src interface{}) error {
	switch x := src.(type) {
	case string:
		return d.SetString(x)
	case []byte:
		return d.SetString(string(x))
	case time.Time:
		d.Set(x)
		return nil
	default:
		return errors.Errorf("unknown type %T", src)
	}
	return nil
}

type Lob struct {
	*goracle.Lob
	data []byte
	err  error
}

func (L *Lob) read() error {
	if L.err != nil {
		return L.err
	}
	if L.data == nil {
		L.data, L.err = ioutil.ReadAll(L.Lob)
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
		L.data, L.err = ioutil.ReadAll(L.Lob)
	case *Lob:
		L.data, L.err = ioutil.ReadAll(L.Lob)
	case io.Reader:
		L.data, L.err = ioutil.ReadAll(x)
	case []byte:
		L.data = x
	case string:
		L.data = []byte(x)
	default:
		return errors.Errorf("unknown type %T", src)
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
	case string, goracle.Number:
		var s string
		switch x := x.(type) {
		case string:
			s = x
		case goracle.Number:
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
		log.Printf("WARN: unknown Int64 type %T", v)
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
	case int64:
		return int32(x)
	case float64:
		return int32(x)
	case float32:
		return int32(x)
	case sql.NullInt64:
		return int32(x.Int64)
	case string, goracle.Number:
		var s string
		switch x := x.(type) {
		case string:
			s = x
		case goracle.Number:
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
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	case float32:
		return int64(x)
	case sql.NullInt64:
		return x.Int64
	case string, goracle.Number:
		var s string
		switch x := x.(type) {
		case string:
			s = x
		case goracle.Number:
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
func AsDate(v interface{}) Date {
	var d Date
	if v == nil {
		return d
	}
	switch x := v.(type) {
	case Date:
		return x
	case time.Time:
		d.Set(x)
	case string:
		d.SetString(x)
	default:
		log.Printf("WARN: unknown Date type %T", v)
	}

	return d
}

func AsTime(v interface{}) time.Time {
	if v == nil {
		return time.Time{}
	}
	if t, ok := v.(time.Time); ok {
		return t
	}
	t, err := AsDate(v).Get()
	if err != nil {
		log.Printf("ERROR parsing %q as Date: %v", v, err)
	}
	return t
}
