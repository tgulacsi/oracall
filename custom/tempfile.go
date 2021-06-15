// Copyright 2019, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package custom

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"unsafe"
)

// ReadAll reads the reader and returns the byte slice.
//
// If the read length is below the threshold, then the bytes are read into memory;
// otherwise, a temp file is created, and mmap-ed.
func ReadAll(r io.Reader, threshold int) ([]byte, error) {
	lr := io.LimitedReader{R: r, N: int64(threshold) + 1}
	var buf bytes.Buffer
	_, err := io.Copy(&buf, &lr)
	if err != nil || buf.Len() <= threshold {
		return buf.Bytes(), err
	}
	fh, err := ioutil.TempFile("", "oracall-custom-readall-")
	if err != nil {
		return buf.Bytes(), err
	}
	os.Remove(fh.Name())
	if _, err = fh.Write(buf.Bytes()); err != nil {
		fh.Close()
		return buf.Bytes(), err
	}
	buf.Truncate(0)
	if _, err = io.Copy(fh, r); err != nil {
		fh.Close()
		return nil, err
	}
	b, err := Mmap(fh)
	fh.Close()
	return b, err
}

// ReadAllString is like ReadAll, but returns a string.
func ReadAllString(r io.Reader, threshold int) (string, error) {
	b, err := ReadAll(r, threshold)
	return *((*string)(unsafe.Pointer(&b))), err
}
