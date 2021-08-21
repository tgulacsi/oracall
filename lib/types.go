// Copyright 2017, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// TxPool is for transaction handling.
// Transaction handling over function calls
// - TranBegin will create an *sql.Tx with Begin()
// - TranClose(success bool) will COMMIT/ROLLBACK the transaction.
type TxPool struct {
	mu         sync.RWMutex
	ctx        context.Context // for cancel all transactions
	cancel     context.CancelFunc
	m          map[uint64]transaction
	count, max uint32
}
type transaction struct {
	lastacc time.Time
	*sql.Tx
}

func (p *TxPool) Close() error {
	p.mu.Lock()
	cancel, m := p.cancel, p.m
	p.count, p.ctx, p.cancel, p.m = 0, nil, nil, nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	for _, tx := range m {
		tx.Rollback()
	}
	return nil
}

const DefaultMaxTran = 16

// NewTxPool creates a new TxPool for over-function-call transaction handling.
// The maximum number of open transactions is max, or DefaultMaxTran if max <= 0.
func NewTxPool(ctx context.Context, max int) *TxPool {
	if max <= 0 {
		max = DefaultMaxTran
	}
	ctx, cancel := context.WithCancel(ctx)
	return &TxPool{max: uint32(max), m: make(map[uint64]transaction), ctx: ctx, cancel: cancel}
}

// Get the transaction identified by tranID from the pool.
func (p *TxPool) Get(tranID uint64) (*sql.Tx, error) {
	p.mu.RLock()
	tx := p.m[tranID]
	p.mu.RUnlock()
	if tx.Tx == nil {
		return nil, fmt.Errorf("%d: %w", tranID, ErrTranNotExist)
	}
	return tx.Tx, nil
}

// Put back the transaction to the pool.
func (p *TxPool) Put(tranID uint64, tx *sql.Tx) {
	t := transaction{Tx: tx, lastacc: time.Now()}
	p.mu.Lock()
	p.m[tranID] = t
	p.mu.Unlock()
}
func (p *TxPool) Begin(conn interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}) (*sql.Tx, uint64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.count >= p.max {
		return nil, 0, fmt.Errorf("%w: %d", ErrTranTooMany, p.count)
	}
	tx, err := conn.BeginTx(p.ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	tranID := uint64(uintptr(unsafe.Pointer(tx)))
	t := transaction{Tx: tx, lastacc: time.Now()}
	p.m[tranID] = t
	return tx, tranID, nil
}
func (p *TxPool) End(tranID uint64, commit bool) error {
	p.mu.Lock()
	tx := p.m[tranID]
	delete(p.m, tranID)
	p.count--
	p.mu.Unlock()
	if tx.Tx == nil {
		return fmt.Errorf("%d: %w", tranID, ErrTranNotExist)
	}
	if commit {
		return tx.Commit()
	}
	return tx.Rollback()
}
func (p *TxPool) Evict(limit time.Duration) {
	threshold := time.Now().Add(-limit)
	p.mu.Lock()
	defer p.mu.Unlock()
	for k, tx := range p.m {
		if tx.lastacc.After(threshold) {
			continue
		}
		delete(p.m, k)
		tx.Rollback()
	}
}

var (
	ErrTranNotExist = errors.New("transaction does not exist")
	ErrTranTooMany  = errors.New("too many open transaction")
)

type PlsType struct {
	ora              string
	Precision, Scale uint8
}

func (arg PlsType) String() string { return arg.ora }

// NewArg returns a new argument to ease arument conversions.
func NewPlsType(ora string, precision, scale uint8) PlsType {
	return PlsType{ora: ora, Precision: precision, Scale: scale}
}

// FromOra retrieves the value of the argument with arg type, from src variable to dst variable.
func (arg PlsType) FromOra(dst, src, varName string) string {
	if Gogo {
		if varName != "" {
			switch arg.ora {
			case "DATE", "TIMESTAMP":
				return fmt.Sprintf("%s = &custom.DateTime{Time:%s}", dst, varName)
				//return fmt.Sprintf("%s = &custom.DateTime{Time:%s}", dst, varName)
			}
		}
	}
	switch arg.ora {
	case "BLOB":
		if varName != "" {
			return fmt.Sprintf("if %s.Reader != nil { if %s, err = custom.ReadAll(%s.Reader, 1<<20); err != nil { return } }", varName, dst, varName)
		}
		return fmt.Sprintf("%s = godror.Lob{Reader: bytes.NewReader(%s)}", dst, src)
	case "CLOB":
		if varName != "" {
			return fmt.Sprintf("if %s.Reader != nil { if %s, err = custom.ReadAllString(%s.Reader, 1<<20); err != nil { return } }", varName, dst, varName)
		}
		return fmt.Sprintf("%s = godror.Lob{IsClob:true, Reader: strings.NewReader(%s)}", dst, src)
	case "DATE", "TIMESTAMP":
		return fmt.Sprintf("%s = (%s)", dst, src)
	case "PLS_INTEGER", "PL/SQL PLS INTEGER":
		return fmt.Sprintf("%s = int32(%s)", dst, src)
	case "NUMBER":
		if arg.Precision < 19 {
			typ := goNumType(arg.Precision, arg.Scale)
			if typ == "godror.Number" {
				typ = "string"
			}
			return fmt.Sprintf("%s = %s(%s)", dst, typ, src)
		}
		return fmt.Sprintf("%s = string(%s)", dst, src)

	case "":
		panic(fmt.Sprintf("empty \"ora\" type: %#v", arg))
	}
	return fmt.Sprintf("%s = %s // %s fromOra", dst, src, arg.ora)
}

func (arg PlsType) GetOra(src, varName string) string {
	switch arg.ora {
	case "DATE":
		if varName != "" {
			return fmt.Sprintf("%s.Format(time.RFC3339)", varName)
		}
		if Gogo {
			return fmt.Sprintf("custom.AsDate(%s)", src)
		}
		return fmt.Sprintf("custom.AsTimestamp(%s)", src)
	case "NUMBER":
		if varName != "" {
			//return fmt.Sprintf("string(%s.(godror.Number))", varName)
			return fmt.Sprintf("custom.AsString(%s)", varName)
		}
		//return fmt.Sprintf("string(%s.(godror.Number))", src)
		return fmt.Sprintf("custom.AsString(%s)", src)
	}
	return src
}

// ToOra adds the value of the argument with arg type, from src variable to dst variable.
func (arg PlsType) ToOra(dst, src string, dir direction) (expr string, variable string) {
	dstVar := mkVarName(dst)
	var inTrue string
	if dir.IsInput() {
		inTrue = ",In:true"
	}
	if arg.ora == "NUMBER" && arg.Precision != 0 && arg.Precision < 10 && arg.Scale == 0 {
		arg.ora = "PLS_INTEGER"
	}
	switch arg.ora {
	case "DATE":
		np := strings.TrimPrefix(src, "&")
		if Gogo {
			if dir.IsOutput() {
				if !strings.HasPrefix(dst, "params[") {
					return fmt.Sprintf(`%s = %s.Time`, dst, np), ""
				}
				return fmt.Sprintf(`if %s == nil { %s = new(custom.DateTime) }
					%s = sql.Out{Dest:&%s.Time%s}`,
						np, np,
						dst, strings.TrimPrefix(src, "&"), inTrue,
					),
					""
			}
			return fmt.Sprintf(`%s = custom.AsDate(%s).Time // toOra D`, dst, np), ""
		}
		if dir.IsOutput() {
			if !strings.HasPrefix(dst, "params[") {
				return fmt.Sprintf(`%s = %s.AsTime()`, dst, np), ""
			}
			return fmt.Sprintf(`if %s == nil { %s = new(custom.Timestamp) }
				%s = sql.Out{Dest:&%s.AsTime()%s}`,
					np, np,
					dst, strings.TrimPrefix(src, "&"), inTrue,
				),
				""
		}
		return fmt.Sprintf(`%s = custom.AsTime(%s) // toOra D`, dst, np), ""

	case "PLS_INTEGER", "PL/SQL PLS INTEGER":
		if src[0] != '&' {
			return fmt.Sprintf("var %s sql.NullInt32; if %s != 0 { %s.Int32, %s.Valid = int32(%s), true }; %s = int32(%s.Int32)", dstVar, src, dstVar, dstVar, src, dst, dstVar), dstVar
		}
	case "NUMBER":
		if src[0] != '&' {

			return fmt.Sprintf("%s := %s(%s); %s = %s", dstVar, goNumType(arg.Precision, arg.Scale), src, dst, dstVar), dstVar
		}
	case "CLOB":
		if dir.IsOutput() {
			return fmt.Sprintf("%s := godror.Lob{IsClob:true}; %s = sql.Out{Dest:&%s}", dstVar, dst, dstVar), dstVar
		}
		return fmt.Sprintf("%s := godror.Lob{IsClob:true,Reader:strings.NewReader(%s)}; %s = %s", dstVar, src, dst, dstVar), dstVar
	}
	if dir.IsOutput() && !(strings.HasSuffix(dst, "]") && !strings.HasPrefix(dst, "params[")) {
		if arg.ora == "NUMBER" {
			if goNumType(arg.Precision, arg.Scale) == "godror.Number" {
				return fmt.Sprintf("%s = sql.Out{Dest:(*%s)(unsafe.Pointer(%s))%s} // NUMBER(%d,%d)",
					dst, goNumType(arg.Precision, arg.Scale), src, inTrue, arg.Precision, arg.Scale), ""
			}
			return fmt.Sprintf("%s = sql.Out{Dest:%s%s} // NUMBER(%d,%d)",
				dst, src, inTrue, arg.Precision, arg.Scale), ""
		}
		return fmt.Sprintf("%s = sql.Out{Dest:%s%s} // %s", dst, src, inTrue, arg.ora), ""
	}
	return fmt.Sprintf("%s = %s // %s", dst, src, arg.ora), ""
}

func mkVarName(dst string) string {
	h := fnv.New64()
	io.WriteString(h, dst)
	var raw [8]byte
	var enc [8 * 2]byte
	hex.Encode(enc[:], h.Sum(raw[:0]))
	return fmt.Sprintf("var_%s", enc[:])
}

func ParseDigits(s string, precision, scale int) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if s[0] == '-' || s[0] == '+' {
		s = s[1:]
	}
	var dotSeen bool
	bucket := precision
	if precision == 0 && scale == 0 {
		bucket = 38
	}
	for _, r := range s {
		if !dotSeen && r == '.' {
			dotSeen = true
			if !(precision == 0 && scale == 0) {
				bucket = scale
			}
			continue
		}
		if '0' <= r && r <= '9' {
			bucket--
			if bucket < 0 {
				return fmt.Errorf("want NUMBER(%d,%d), has %q", precision, scale, s)
			}
		} else {
			return fmt.Errorf("want number, has %c in %q", r, s)
		}
	}
	return nil
}

func goNumType(precision, scale uint8) string {
	if precision >= 19 || precision == 0 || scale != 0 {
		return "godror.Number"
	}
	if scale != 0 {
		if precision < 10 {
			return "float32"
		}
		return "float64"
	}
	if precision < 10 {
		return "int32"
	}
	return "int64"
}

// vim: set fileencoding=utf-8 noet:
