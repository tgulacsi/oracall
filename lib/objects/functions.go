// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/tgulacsi/oracall/lib"
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

type Package struct {
	Name  string
	Funcs []Function
}

func (P Package) GenProto() ProtoService {
	svc := ProtoService{Name: oracall.CamelCase(P.Name)}
	svc.RPCs = make([]ProtoRPC, len(P.Funcs))
	pkg := oracall.CamelCase(P.Name) + "_"
	for i, f := range P.Funcs {
		nm := pkg + oracall.CamelCase(f.Name)
		svc.RPCs[i] = ProtoRPC{
			Name:   nm,
			Input:  ProtoMessage{Name: nm + "_Input"},
			Output: ProtoMessage{Name: nm + "_Output"},
		}
		for _, dst := range []struct {
			Dest   *ProtoMessage
			Output bool
		}{
			{Dest: &svc.RPCs[i].Input, Output: false},
			{Dest: &svc.RPCs[i].Output, Output: true},
		} {
			for _, a := range f.Arguments {
				if dst.Output && a.InOut.IsOut() || !dst.Output && a.InOut.IsIn() {
					A := a.Argument
					if dst.Output && A.Name == "" {
						A.Name = "RET"
					}
					dst.Dest.Fields = append(dst.Dest.Fields, A.GenProto(len(dst.Dest.Fields)+1))
				}
			}
		}
	}
	return svc
}

func (A Argument) GenProto(num int) ProtoField {
	return ProtoField{
		Name:       oracall.CamelCase(A.Name),
		Number:     num,
		Repeated:   A.Type.IsColl(),
		Type:       A.Type.GenProto(),
		Extensions: []ProtoExtension{{Name: protoExtendFieldName, Value: A.Type.name(".")}},
	}
}

func (t Type) GenProto() ProtoType {
	if t.IsColl() && t.Elem != nil {
		return t.Elem.GenProto()
	}
	exts := []ProtoExtension{{Name: protoExtendTypeName, Value: t.name(".")}}
	if !t.composite() {
		return ProtoType{Simple: t.protoType(), Extensions: exts}
	}
	msg := ProtoMessage{
		Name:       oracall.CamelCase(t.name("__")),
		Fields:     make([]ProtoField, len(t.Arguments)),
		Extensions: exts,
	}
	for i, A := range t.Arguments {
		msg.Fields[i] = A.GenProto(i + 1)
	}
	return ProtoType{Message: &msg, Extensions: exts}
}

func (tt *Types) WritePackage(ctx context.Context, w io.Writer, pkg Package) error {
	bw := bufio.NewWriter(w)
	if err := tt.WriteInOuts(ctx, bw, pkg.Funcs); err != nil {
		return err
	}
	fmt.Fprintf(bw, "service %s {\n", oracall.CamelCase(pkg.Name))
	err := tt.WriteRPC(ctx, bw, pkg.Funcs)
	bw.WriteString("}\n")
	return err
}

func (tt *Types) NewPackage(ctx context.Context, db querier, pkg string) (Package, error) {
	logger := zlog.SFromContext(ctx)
	pkg = strings.ToUpper(pkg)
	const qry = `SELECT object_name, subprogram_id, argument_name, 
		in_out, data_type, data_length, data_precision, data_scale,
		type_owner, type_name, type_subname, type_link, 
		type_object_type, pls_type
	FROM user_arguments
	WHERE object_name NOT LIKE '%#' 
	  AND data_level = 0 AND overload IS NULL 
	  AND package_name = :1
	ORDER BY subprogram_id, sequence`
	P := Package{Name: pkg}
	rows, err := db.QueryContext(ctx, qry, pkg)
	if err != nil {
		return P, fmt.Errorf("%s: %w", qry, err)
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
			return P, fmt.Errorf("scan %s: %w", qry, err)
		}
		if lastID != subID {
			if lastID != 0 && F.Name != "" {
				P.Funcs = append(P.Funcs, F)
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
			return P, fmt.Errorf("resolve %+v for %+v: %w",
				tParams, F, err)
		}
		F.Arguments = append(F.Arguments, FuncArg{
			Argument: arg, InOut: newInOut(inOut),
		})
	}
	if F.Name != "" {
		P.Funcs = append(P.Funcs, F)
	}
	return P, rows.Close()
}

func (tt *Types) WriteInOuts(ctx context.Context, w io.Writer, funcs []Function) error {
	bw := bufio.NewWriter(w)
	for _, f := range funcs {
		if err := f.writeInOut(bw); err != nil {
			return err
		}
	}
	return nil
}

func (tt *Types) WriteRPC(ctx context.Context, w io.Writer, funcs []Function) error {
	bw := bufio.NewWriter(w)
	for _, f := range funcs {
		var streamQual string
		if false { //f.HasCursorOut() {
			streamQual = "stream "
		}
		fmt.Fprintf(bw, "rpc %[1]s (%[1]s_Input) returns (%[2]s%[1]s_Output) {%[3]s}\n",
			oracall.CamelCase(f.Package)+"_"+oracall.CamelCase(f.Name),
			streamQual,
			"", // tags
		)
	}
	return bw.Flush()
}

func (f Function) writeInOut(bw *bufio.Writer) error {
	for _, output := range []bool{false, true} {
		nm := "Input"
		if output {
			nm = "Output"
		}
		args := make([]Argument, 0, len(f.Arguments))
		args = args[:0]
		for _, a := range f.Arguments {
			if output && a.InOut.IsOut() || !output && a.InOut.IsIn() {
				A := a.Argument
				if output && A.Name == "" {
					A.Name = "RET"
				}
				args = append(args, A)
			}
		}
		fmt.Fprintf(bw, "message %s_%s_%s {\n", oracall.CamelCase(f.Package), oracall.CamelCase(f.Name), nm)
		if err := writeArguments(bw, args); err != nil {
			return err
		}
		bw.WriteString("}\n")
	}
	return nil
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
func (x InOut) IsIn() bool     { return x == IOIn || x == IOBoth }
func (x InOut) IsOut() bool    { return x == IOOut || x == IOBoth }
