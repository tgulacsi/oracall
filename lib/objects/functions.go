// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/UNO-SOFT/zlog/v2"
)

type Function struct {
	Package, Name string
	Arguments     []FuncArg
}

func (f Function) IsFunction() bool { return len(f.Arguments) != 0 && f.Arguments[0].Name == "" }
func (f Function) Returns() (Argument, bool) {
	if len(f.Arguments) != 0 {
		if a := f.Arguments[0]; a.Name == "" {
			return a.Argument, true
		}
	}
	return Argument{}, false
}

type FuncArg struct {
	Argument
	InOut InOut
}

func (tt *Types) NewPackage(ctx context.Context, db querier, pkg string) ([]Function, error) {
	logger := zlog.SFromContext(ctx)
	pkg = strings.ToUpper(pkg)
	const qry = `SELECT object_name, subprogram_id, argument_name, 
		in_out, data_type, data_length, data_precision, data_scale,
		type_owner, type_name, type_subname, type_link, 
		type_object_type, pls_type
	FROM user_arguments
	WHERE data_level = 0 AND overload IS NULL AND package_name = :1
	ORDER BY subprogram_id, sequence`
	var funcs []Function
	rows, err := db.QueryContext(ctx, qry, pkg)
	if err != nil {
		return funcs, fmt.Errorf("%s: %w", qry, err)
	}
	defer rows.Close()
	var lastID int32
	var F Function
	for rows.Next() {
		var (
			obj, inOut string
			arg        Argument
			subID      int32
			tParams    TypeParams
		)
		if err = rows.Scan(
			&obj, &subID, &arg.Name,
			&inOut, &tParams.DataType, &tParams.Length, &tParams.Precision, &tParams.Scale,
			&tParams.Owner, &tParams.Name, &tParams.Subname, &tParams.Link,
			&tParams.ObjectType, &tParams.PlsType,
		); err != nil {
			return funcs, fmt.Errorf("scan %s: %w", qry, err)
		}
		if lastID != subID {
			if lastID != 0 && F.Name != "" {
				funcs = append(funcs, F)
			}
			F = Function{Package: pkg, Name: obj}
			lastID = subID
		}
		if arg.Type, err = tt.FromParams(ctx, tParams); err != nil {
			if errors.Is(err, ErrNotSupported) {
				logger.Warn("not supported", "pkg", pkg, "fun", obj, "arg", arg.Name, "type", tParams)
				F.Name = ""
				continue
			}
			return funcs, fmt.Errorf("resolve %+v: %w", tParams, err)
		}
		F.Arguments = append(F.Arguments, FuncArg{
			Argument: arg, InOut: newInOut(inOut),
		})
	}
	if F.Name != "" {
		funcs = append(funcs, F)
	}
	return funcs, rows.Close()
}

type InOut uint8

const (
	IOIn   = InOut('I')
	IOOut  = InOut('O')
	IOBoth = InOut('X')
)

func newInOut(s string) InOut {
	switch s {
	case "IN":
		return IOIn
	case "OUT":
		return IOOut
	case "IN/OUT":
		return IOBoth
	default:
		panic(fmt.Errorf("unknown InOut string %q", s))
	}
}
func (x InOut) String() string { return string(x) }
