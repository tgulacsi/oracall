// Copyright 2014, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package custom

import (
	"os"
)

// MaxInt is the maximum value an int can contain.
const MaxInt = int64(int(^uint(0) >> 1))

// MmapFile returns the mmap of the given path.
func MmapFile(fn string) ([]byte, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	p, err := Mmap(f)
	f.Close()
	return p, err
}
