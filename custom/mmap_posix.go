// +build posix linux !windows

// Copyright 2019, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package custom

import (
	"errors"
	"io/ioutil"
	"os"
	"runtime"
	"syscall"
)

// Mmap the file for read, return the bytes and the error.
// Will read the data directly if Mmap fails.
func Mmap(f *os.File) ([]byte, error) {
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() > MaxInt {
		return nil, errors.New("file too big to Mmap")
	}
	if fi.Size() == 0 {
		return []byte{}, nil
	}
	p, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()),
		syscall.PROT_READ,
		syscall.MAP_PRIVATE|syscall.MAP_DENYWRITE|syscall.MAP_POPULATE)
	if err != nil {
		p, _ = ioutil.ReadAll(f)
		return p, err
	}

	runtime.SetFinalizer(&p, func(p *[]byte) {
		if p != nil {
			syscall.Munmap(*p)
		}
	})

	return p, nil
}
