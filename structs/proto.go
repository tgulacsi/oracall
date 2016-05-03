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
	"fmt"
	"io"

	"gopkg.in/errgo.v1"
	"gopkg.in/inconshreveable/log15.v2"
)

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
		fmt.Fprintf(w, `
service %s {
	rpc %s (%s) returns (%s) {}
}
`, fun.Name(), fun.Name(), fun.getStructName(false), fun.getStructName(true))
	}

	return nil
}

func (f Function) SaveProtobuf(dst io.Writer, out bool) error {
	dirmap, dirname := uint8(DIR_IN), "input"
	if out {
		dirmap, dirname = DIR_OUT, "output"
	}
	var (
		err        error
		aName, got string
	)
	args := make([]Argument, 0, len(f.Args))
	for _, arg := range f.Args {
		if arg.Direction&dirmap > 0 {
			args = append(args, arg)
		}
	}
	// return variable for function out structs
	if out && f.Returns != nil {
		args = append(args, *f.Returns)
	}

	buf := buffers.Get()
	defer buffers.Put(buf)
	w := errWriter{Writer: buf, err: &err}

	fmt.Fprintf(w, `
	// %s %s
	message %s {
		`, f.Name(), dirname,
	)

	Log.Debug("SaveStruct",
		"function", log15.Lazy{func() string { return fmt.Sprintf("%#v", f) }})
	for i, arg := range args {
		if arg.Flavor == FLAVOR_TABLE && arg.TableOf == nil {
			return errgo.WithCausef(nil, ErrMissingTableOf, "no table of data for %s.%s (%v)", f.Name(), arg, arg)
		}
		aName = capitalize(goName(arg.Name))
		got = arg.goType(f.types, arg.Flavor == FLAVOR_TABLE)
		//lName := strings.ToLower(arg.Name)
		fmt.Fprintf(w, "\t%s %s = %d;\n", got, aName, i)
	}
	io.WriteString(w, "}\n")

	if err != nil {
		return err
	}

	return nil
}
