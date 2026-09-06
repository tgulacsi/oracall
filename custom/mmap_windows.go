//go:build windows
// +build windows

// Copyright 2015, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: EUPL-1.2

package custom

import (
	"bytes"
	"io"
	"os"
)

// Mmap returns a mmap of the given file - just a copy of it.
func Mmap(f *os.File) ([]byte, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, f)
	return buf.Bytes(), err
}
