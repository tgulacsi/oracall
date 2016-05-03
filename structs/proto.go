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
	"strings"

	"gopkg.in/errgo.v1"
)

//go:generate go get github.com/golang/protobuf/protoc-gen-go
// https://github.com/google/protobuf/releases/download/v3.0.0-beta-2/protoc-3.0.0-beta-2-linux-x86_64.zip

func SaveProtobuf(dst io.Writer, functions []Function, pkg string) error {
	var err error
	w := errWriter{Writer: dst, err: &err}

	io.WriteString(w, `syntax = "proto3";`+"\n\n")

	if pkg != "" {
		fmt.Fprintf(w, "package %s;", pkg)
	}
	types := make(map[string]string, 16)

FunLoop:
	for _, fun := range functions {
		fun.types = types
		for _, dir := range []bool{false, true} {
			if err := fun.SaveProtobuf(w, dir); err != nil {
				if errgo.Cause(err) == ErrMissingTableOf {
					Log.Warn("SKIP function, missing TableOf info", "function", fun.Name())
					continue FunLoop
				}
				return err
			}
		}
		name := dot2D.Replace(fun.Name())
		fmt.Fprintf(w, `
service %s {
	rpc %s (%s) returns (%s) {}
}
`, name, name, strings.ToLower(fun.getStructName(false)), strings.ToLower(fun.getStructName(true)))
	}

	return nil
}

func (f Function) SaveProtobuf(dst io.Writer, out bool) error {
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

	return protoWriteMessageTyp(dst, dot2D.Replace(strings.ToLower(f.Name()))+"__"+dirname, f.types, args...)
}

var dot2D = strings.NewReplacer(".", "__")

func protoWriteMessageTyp(dst io.Writer, msgName string, types map[string]string, args ...Argument) error {
	var err error
	w := errWriter{Writer: dst, err: &err}

	fmt.Fprintf(w, "\nmessage %s {\n", msgName)

	seen := make(map[string]struct{}, 16)
	buf := buffers.Get()
	defer buffers.Put(buf)
	for i, arg := range args {
		if strings.HasSuffix(arg.Name, "#") {
			continue
		}
		if arg.Flavor == FLAVOR_TABLE && arg.TableOf == nil {
			return errgo.WithCausef(nil, ErrMissingTableOf, "no table of data for %s.%s (%v)", msgName, arg, arg)
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
		typ := protoType(got)
		if arg.Flavor == FLAVOR_SIMPLE || arg.Flavor == FLAVOR_TABLE && arg.TableOf.Flavor == FLAVOR_SIMPLE {
			fmt.Fprintf(w, "\t%s%s %s = %d;\n", rule, typ, aName, i+1)
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
			buf.Reset()
			if err := protoWriteMessageTyp(buf, typ, types, subArgs...); err != nil {
				return err
			}
			w.Write(bytes.Replace(buf.Bytes(), []byte("\n"), []byte("\n\t"), -1))
			seen[typ] = struct{}{}
		}
		fmt.Fprintf(w, "\n\t%s%s %s = %d;\n", rule, typ, aName, i+1)
	}
	io.WriteString(w, "}")

	return err
}

func protoType(got string) string {
	switch strings.ToLower(got) {
	case "ora.date":
		return "string"
	case "ora.string":
		return "string"
	case "int32":
		return "sint32"
	case "ora.int32":
		return "sint32"
	case "float64":
		return "double"
	case "ora.float64":
		return "double"
	default:
		return strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(got, "[]"), "*"))
	}
}
