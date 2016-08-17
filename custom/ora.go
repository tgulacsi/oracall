package custom

import (
	"io/ioutil"
	"time"

	"gopkg.in/rana/ora.v3"
)

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

func (d *Date) Set(date ora.Date) {
	*d = Date(date.Get().Format(timeFormat))
}
func (d Date) Get() ora.Date {
	t, err := time.Parse(timeFormat, string(d)) // TODO(tgulacsi): more robust parser
	var od ora.Date
	if err == nil {
		od.Set(t)
	}
	return od
}

type Lob struct {
	ora.Lob
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
