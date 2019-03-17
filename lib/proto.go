/*
Copyright 2016 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oracall

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	fstructs "github.com/fatih/structs"
	"github.com/pkg/errors"
)

var SkipMissingTableOf = true

var Gogo bool
var NumberAsString bool

//go:generate sh ./download-protoc.sh
//go:generate go get -u github.com/gogo/protobuf/protoc-gen-gogofast

// build: protoc --gogofast_out=plugins=grpc:. my.proto
// build: protoc --go_out=plugins=grpc:. my.proto

func SaveProtobuf(dst io.Writer, functions []Function, pkg string) error {
	var err error
	w := errWriter{Writer: dst, err: &err}

	io.WriteString(w, `syntax = "proto3";`+"\n\n")

	if pkg != "" {
		fmt.Fprintf(w, "package %s;\n", pkg)
	}
	if Gogo {
		io.WriteString(w, `
	import "google/protobuf/timestamp.proto";
	import "github.com/gogo/protobuf/gogoproto/gogo.proto";
`)
	}
	seen := make(map[string]struct{}, 16)

	services := make([]string, 0, len(functions))

FunLoop:
	for _, fun := range functions {
		fName := fun.name
		if fun.alias != "" {
			fName = fun.alias
		}
		fName = strings.ToLower(fName)
		if err := fun.SaveProtobuf(w, seen); err != nil {
			if SkipMissingTableOf && (errors.Cause(err) == ErrMissingTableOf ||
				errors.Cause(err) == UnknownSimpleType) {
				Log("msg", "SKIP function, missing TableOf info", "function", fName)
				continue FunLoop
			}
			return errors.WithMessage(err, fun.name)
		}
		var streamQual string
		if fun.HasCursorOut() {
			streamQual = "stream "
		}
		name := CamelCase(dot2D.Replace(fName))
		var comment string
		if fun.Documentation != "" {
			comment = "\n/// " + strings.Replace(fun.Documentation, "\n", "\n/// ", -1) + "\n\t"
		}
		services = append(services,
			fmt.Sprintf(`%srpc %s (%s) returns (%s%s) {}`,
				comment,
				name,
				CamelCase(fun.getStructName(false, false)),
				streamQual,
				CamelCase(fun.getStructName(true, false)),
			),
		)
	}

	fmt.Fprintf(w, "\nservice %s {\n", CamelCase(pkg))
	for _, s := range services {
		fmt.Fprintf(w, "\t%s\n", s)
	}
	w.Write([]byte("}"))

	return nil
}

func (f Function) SaveProtobuf(dst io.Writer, seen map[string]struct{}) error {
	var buf bytes.Buffer
	if err := f.saveProtobufDir(&buf, seen, false); err != nil {
		return errors.Wrap(err, "input")
	}
	if err := f.saveProtobufDir(&buf, seen, true); err != nil {
		return errors.Wrap(err, "output")
	}
	_, err := dst.Write(buf.Bytes())
	return err
}
func (f Function) saveProtobufDir(dst io.Writer, seen map[string]struct{}, out bool) error {
	dirmap, dirname := DIR_IN, "input"
	if out {
		dirmap, dirname = DIR_OUT, "output"
	}
	args := make([]Argument, 0, len(f.Args)+1)
	for _, arg := range f.Args {
		if arg.Direction&dirmap > 0 {
			args = append(args, arg)
		}
	}
	// return variable for function out structs
	if out && f.Returns != nil {
		args = append(args, *f.Returns)
	}

	nm := f.name
	if f.alias != "" {
		nm = f.alias
	}
	return protoWriteMessageTyp(dst,
		CamelCase(dot2D.Replace(strings.ToLower(nm))+"__"+dirname),
		seen, args...)
}

var dot2D = strings.NewReplacer(".", "__")

func protoWriteMessageTyp(dst io.Writer, msgName string, seen map[string]struct{}, args ...Argument) error {
	for _, arg := range args {
		if arg.Flavor == FLAVOR_TABLE && arg.TableOf == nil {
			return errors.Wrapf(ErrMissingTableOf, "no table of data for %s.%s (%v)", msgName, arg, arg)
		}
	}

	var err error
	w := errWriter{Writer: dst, err: &err}
	fmt.Fprintf(w, "\nmessage %s {\n", msgName)

	buf := Buffers.Get()
	defer Buffers.Put(buf)
	for i, arg := range args {
		var rule string
		if strings.HasSuffix(arg.Name, "#") {
			arg.Name = replHidden(arg.Name)
		}
		if arg.Flavor == FLAVOR_TABLE {
			if arg.TableOf == nil {
				return errors.Wrapf(ErrMissingTableOf, "no table of data for %s.%s (%v)", msgName, arg, arg)
			}
			rule = "repeated "
		}
		aName := arg.Name
		got, err := arg.goType(false)
		if err != nil {
			return errors.WithMessage(err, msgName)
		}
		got = strings.TrimPrefix(got, "*")
		if strings.HasPrefix(got, "[]") {
			rule = "repeated "
			got = got[2:]
		}
		got = strings.TrimPrefix(got, "*")
		if got == "" {
			got = mkRecTypName(arg.Name)
		}
		typ, pOpts := protoType(got, arg.Name, arg.AbsType)
		var optS string
		if s := pOpts.String(); s != "" {
			optS = " " + s
		}
		if arg.Flavor == FLAVOR_SIMPLE || arg.Flavor == FLAVOR_TABLE && arg.TableOf.Flavor == FLAVOR_SIMPLE {
			fmt.Fprintf(w, "\t// %s\n\t%s%s %s = %d%s;\n", arg.AbsType, rule, typ, aName, i+1, optS)
			continue
		}
		typ = CamelCase(typ)
		if _, ok := seen[typ]; !ok {
			seen[typ] = struct{}{}
			//lName := strings.ToLower(arg.Name)
			subArgs := make([]Argument, 0, 16)
			if arg.TableOf == nil {
				for _, v := range arg.RecordOf {
					subArgs = append(subArgs, *v.Argument)
				}
			} else {
				if arg.TableOf.RecordOf == nil {
					subArgs = append(subArgs, *arg.TableOf)
				} else {
					for _, v := range arg.TableOf.RecordOf {
						subArgs = append(subArgs, *v.Argument)
					}
				}
			}
			if err = protoWriteMessageTyp(buf, typ, seen, subArgs...); err != nil {
				Log("msg", "protoWriteMessageTyp", "error", err)
				return err
			}
		}
		fmt.Fprintf(w, "\t%s%s %s = %d%s;\n", rule, typ, aName, i+1, optS)
	}
	io.WriteString(w, "}\n")
	w.Write(buf.Bytes())

	return err
}

func protoType(got, aName, absType string) (string, protoOptions) {
	switch trimmed := strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(got, "[]"), "*")); trimmed {
	case "string":
		return "string", nil

	case "int32":
		if NumberAsString {
			return "sint32", protoOptions{
				"gogoproto.jsontag": aName + ",string,omitempty",
			}
		}
		return "sint32", nil
	case "float64", "sql.nullfloat64":
		if NumberAsString {
			return "double", protoOptions{
				"gogoproto.jsontag": aName + ",string,omitempty",
			}
		}
		return "double", nil

	case "goracle.number":
		return "string", protoOptions{
			"gogoproto.jsontag": aName + ",omitempty",
		}

	case "custom.date", "time.time":
		return "google.protobuf.Timestamp", protoOptions{
			"gogoproto.stdtime": true,
		}
	case "n":
		return "string", nil
	case "raw":
		return "bytes", nil
	case "goracle.lob", "ora.lob":
		if absType == "CLOB" {
			return "string", nil
		}
		return "bytes", nil
	default:
		return trimmed, nil
	}
}

type protoOptions map[string]interface{}

func (opts protoOptions) String() string {
	if len(opts) == 0 {
		return ""
	}
	var buf bytes.Buffer
	buf.WriteByte('[')
	for k, v := range opts {
		if buf.Len() != 1 {
			buf.WriteString(", ")
		}
		fmt.Fprintf(&buf, "(%s)=", k)
		switch v.(type) {
		case bool:
			fmt.Fprintf(&buf, "%t", v)
		default:
			fmt.Fprintf(&buf, "%q", v)
		}
	}
	buf.WriteByte(']')
	return buf.String()
}

func CopyStruct(dest interface{}, src interface{}) error {
	ds := fstructs.New(dest)
	ss := fstructs.New(src)
	snames := ss.Names()
	svalues := ss.Values()
	for _, df := range ds.Fields() {
		dnm := df.Name()
		for i, snm := range snames {
			if snm == dnm || dnm == CamelCase(snm) || CamelCase(dnm) == snm {
				svalue := svalues[i]
				if err := df.Set(svalue); err != nil {
					return errors.Wrapf(err, "set %q to %q (%v %T)", dnm, snm, svalue, svalue)
				}
			}
		}
	}
	return nil
}
func mkRecTypName(name string) string { return strings.ToLower(name) + "_rek_typ" }
