package custom

import (
	"io/ioutil"
	"time"

	"gopkg.in/rana/ora.v3"
)

type Number struct {
	ora.OCINum
}

func (n Number) Size() int {
	return len(n.OCINum.OCINum)
}
func (n Number) MarshalTo(p []byte) (int, error) {
	p = n.Print(p)
	return len(p), nil
}
func (n Number) Marshal() ([]byte, error) {
	return []byte(n.String()), nil
}
func (n *Number) Unmarshal(p []byte) error {
	return n.OCINum.SetString(string(p))
}

type Date struct {
	ora.Date
}

const timeFormat = "2006-01-02 15:04:05 +0700"

func (d Date) Size() int {
	return 7
}
func (d Date) MarshalTo(p []byte) (int, error) {
	i := copy(p, d.Get().Format(timeFormat))
	return i, nil
}
func (d Date) Marshal() ([]byte, error) {
	return []byte(d.Get().Format(timeFormat)), nil
}
func (d *Date) Unmarshal(p []byte) error {
	t, err := time.Parse(timeFormat, string(p))
	if err != nil {
		return err
	}
	d.Set(t)
	return nil
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
