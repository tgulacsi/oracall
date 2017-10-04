package custom

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"strconv"
	"time"

	"gopkg.in/rana/ora.v4"
)

var ZeroIsAlmostZero bool

type Number string

func (n *Number) Set(num ora.OCINum) {
	*n = Number(num.String())
}
func (n Number) Get() ora.OCINum {
	var num ora.OCINum
	num.SetString(string(n))
	return num
}

type Date string

const timeFormat = "2006-01-02 15:04:05 -0700"

func NewDate(date ora.Date) Date {
	t := date.Get()
	if t.IsZero() {
		return Date("")
	}
	return Date(t.Format(timeFormat))
}
func (d *Date) Set(date ora.Date) {
	if date.IsNull() {
		*d = Date("")
	}
	*d = NewDate(date)
}
func (d Date) Get() (od ora.Date) {
	if d == "" {
		return
	}
	t, err := time.Parse(timeFormat[:len(d)], string(d)) // TODO(tgulacsi): more robust parser
	if err != nil || t.IsZero() {
		return
	}
	od.Set(t)
	return
}

type Lob struct {
	*ora.Lob
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

func AsString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case ora.String:
		return x.Value
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
	case ora.Float64:
		result = x.Value
	case ora.Float32:
		result = float64(x.Value)
	case string:
		if x == "" {
			return 0
		}
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			log.Printf("ERROR parsing %q as Float64", x)
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
	case ora.Int32:
		return x.Value
	case ora.Int64:
		return int32(x.Value)
	case string:
		if x == "" {
			return 0
		}
		i, err := strconv.ParseInt(x, 10, 32)
		if err != nil {
			log.Printf("ERROR parsing %q as Int32", x)
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
	case ora.Int64:
		return x.Value
	case ora.Int32:
		return int64(x.Value)
	case string:
		if x == "" {
			return 0
		}
		i, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			log.Printf("ERROR parsing %q as Int64", x)
		}
		return i
	default:
		log.Printf("WARN: unknown Int64 type %T", v)
	}
	return 0
}
func AsDate(v interface{}) Date {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case Date:
		return x
	case time.Time:
		return Date(x.Format(timeFormat))
	case string:
		return Date(x)
	case ora.Date:
		var d Date
		d.Set(x)
		return d
	default:
		log.Printf("WARN: unknown Date type %T", v)
	}

	return Date("")
}
