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

package structs

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"

	fstructs "github.com/fatih/structs"
	"github.com/pkg/errors"
)

//go:generate sh ./download-protoc.sh
//   go:generate go get -u github.com/golang/protobuf/protoc-gen-go
//go:generate go get -u github.com/gogo/protobuf/protoc-gen-gofast

// build: protoc --gofast_out=plugins=grpc:. my.proto

func SaveProtobuf(dst io.Writer, functions []Function, pkg string) error {
	var err error
	w := errWriter{Writer: dst, err: &err}

	io.WriteString(w, `syntax = "proto3";`+"\n\n")

	if pkg != "" {
		fmt.Fprintf(w, "package %s;\n", pkg)
	}
	io.WriteString(w, `
	import "github.com/gogo/protobuf/gogoproto/gogo.proto";
`)
	types := make(map[string]string, 16)
	seen := make(map[string]struct{}, 16)

FunLoop:
	for _, fun := range functions {
		name := strings.ToLower(dot2D.Replace(fun.Name()))
		fmt.Fprintf(w, `
service %s {
	rpc %s (%s) returns (%s) {}
}
`, name, name, strings.ToLower(fun.getStructName(false)), strings.ToLower(fun.getStructName(true)))
		fun.types = types
		for _, dir := range []bool{false, true} {
			if err := fun.SaveProtobuf(w, seen, dir); err != nil {
				if errors.Cause(err) == ErrMissingTableOf {
					Log("msg", "SKIP function, missing TableOf info", "function", fun.Name())
					continue FunLoop
				}
				return err
			}
		}
	}

	return nil
}

func (f Function) SaveProtobuf(dst io.Writer, seen map[string]struct{}, out bool) error {
	dirmap, dirname := uint8(DIR_IN), "input"
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

	return protoWriteMessageTyp(dst,
		dot2D.Replace(strings.ToLower(f.Name()))+"__"+dirname,
		f.types, seen, args...)
}

var dot2D = strings.NewReplacer(".", "__")

func protoWriteMessageTyp(dst io.Writer, msgName string, types map[string]string, seen map[string]struct{}, args ...Argument) error {
	for _, arg := range args {
		if strings.HasSuffix(arg.Name, "#") {
			continue
		}
		if arg.Flavor == FLAVOR_TABLE && arg.TableOf == nil {
			return errors.Wrapf(ErrMissingTableOf, "no table of data for %s.%s (%v)", msgName, arg, arg)
		}
	}

	var err error
	w := errWriter{Writer: dst, err: &err}
	fmt.Fprintf(w, "\nmessage %s {\n", msgName)

	buf := buffers.Get()
	defer buffers.Put(buf)
	for i, arg := range args {
		if strings.HasSuffix(arg.Name, "#") {
			continue
		}
		if arg.Flavor == FLAVOR_TABLE && arg.TableOf == nil {
			return errors.Wrapf(ErrMissingTableOf, "no table of data for %s.%s (%v)", msgName, arg, arg)
		}
		aName := arg.Name
		got := arg.goType(types, false)
		var rule string
		if strings.HasPrefix(got, "[]") {
			rule = "repeated "
			got = got[2:]
		}
		if strings.HasPrefix(got, "*") {
			got = got[1:]
		}
		typ, opts := protoType(got)
		if arg.Flavor == FLAVOR_SIMPLE || arg.Flavor == FLAVOR_TABLE && arg.TableOf.Flavor == FLAVOR_SIMPLE {
			fmt.Fprintf(w, "\t%s%s %s = %d %s;\n", rule, typ, aName, i+1, opts)
			continue
		}
		if _, ok := seen[typ]; !ok {
			//lName := strings.ToLower(arg.Name)
			subArgs := make([]Argument, 0, 16)
			if arg.TableOf != nil {
				if arg.TableOf.RecordOf == nil {
					subArgs = append(subArgs, *arg.TableOf)
				} else {
					for _, v := range arg.TableOf.RecordOf {
						subArgs = append(subArgs, v)
					}
				}
			} else {
				for _, v := range arg.RecordOf {
					subArgs = append(subArgs, v)
				}
			}
			if err := protoWriteMessageTyp(buf, typ, types, seen, subArgs...); err != nil {
				log.Printf("protoWriteMessage: %v", err)
				return err
			}
			seen[typ] = struct{}{}
		}
		fmt.Fprintf(w, "\t%s%s %s = %d %s;\n", rule, typ, aName, i+1, opts)
	}
	io.WriteString(w, "}\n")
	w.Write(buf.Bytes())

	return err
}

func protoType(got string) (string, protoOptions) {
	switch strings.ToLower(got) {
	case "ora.date":
		return "string", protoOptions(map[string]interface{}{
			"gogoproto.nullable":   false,
			"gogoproto.customtype": "github.com/tgulacsi/oracall/custom.Date",
		})
	case "ora.time", "time.time":
		return "string", nil
	case "ora.string":
		return "string", nil
	case "int32":
		return "sint32", nil
	case "ora.int32":
		return "sint32", nil
	case "float64":
		return "double", nil
	case "ora.float64":
		return "double", nil
	case "n", "ora.n":
		return "string", protoOptions(map[string]interface{}{
			"gogoproto.nullable":   false,
			"gogoproto.customtype": "github.com/tgulacsi/oracall/custom.Number",
		})
	case "ora.lob":
		return "bytes", protoOptions(map[string]interface{}{
			"gogoproto.nullable":   false,
			"gogoproto.customtype": "github.com/tgulacsi/oracall/custom.Lob",
		})
	default:
		return strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(got, "[]"), "*")), nil
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
			fmt.Fprintf(&buf, "%s", v)
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
			if snm == dnm || dnm == goName(snm) || goName(dnm) == snm {
				svalue := svalues[i]
				if err := df.Set(svalue); err != nil {
					return errors.Wrapf(err, "set %q to %q (%v %T)", dnm, snm, svalue, svalue)
				}
			}
		}
	}
	return nil
}
