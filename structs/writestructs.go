/*
Copyright 2013 Tamás Gulácsi

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
	"io"
	"log"
	"strings"

	"github.com/golang/glog"
)

var _ = glog.Infof

func SaveFunctions(dst io.Writer, functions []Function, pkg string) error {
	var err error
	if pkg != "" {
		if _, err = io.WriteString(dst,
			"package "+pkg+"\n\nimport (\n\t\"time\"\n)\n\n"); err != nil {
			return err
		}
	}
	types := make(map[string]string)
	for _, fun := range functions {
		for _, dir := range []bool{false, true} {
			if err = fun.SaveStruct(dst, types, dir); err != nil {
				return err
			}
			if err = fun.SavePlsqlBlock(dst); err != nil {
				return err
			}
		}
	}
	for _, text := range types {
		if _, err = io.WriteString(dst, text); err != nil {
			return err
		}
	}

	return nil
}

func (f Function) SaveStruct(dst io.Writer, types map[string]string, out bool) error {
	//glog.Infof("f=%s", f)
	dirmap, dirname := uint8(DIR_IN), "input"
	if out {
		dirmap, dirname = DIR_OUT, "output"
	}
	var err error
	args := make([]Argument, 0, len(f.Args))
	for _, arg := range f.Args {
		//glog.Infof("dir=%d map=%d => %d", arg.Direction, dirmap, arg.Direction&dirmap)
		if arg.Direction&dirmap > 0 {
			args = append(args, arg)
		}
	}
	//glog.Infof("args[%d]: %s", dirmap, args)

	if len(args) == 0 { // no args
		return nil
	}

	if _, err = io.WriteString(dst,
		"\n// "+f.Name()+" "+dirname+"\ntype "+
			capitalize(f.Package+"__"+f.name+"__"+dirname)+" struct {\n"); err != nil {
		return err
	}

	for _, arg := range args {
		if _, err = io.WriteString(dst, "\t"+capitalize(goName(arg.Name))+" "+arg.goType(types)+"\n"); err != nil {
			return err
		}
	}

	if _, err = io.WriteString(dst, "}\n"); err != nil {
		return err
	}
	return nil
}

func capitalize(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + strings.ToLower(text[1:])
}

func unocap(text string) string {
	i := strings.Index(text, "_")
	if i == 0 {
		return capitalize(text)
	}
	return strings.ToUpper(text[:i]) + "_" + strings.ToLower(text[i+1:])
}

// returns a go type for the argument's type
func (arg Argument) goType(typedefs map[string]string) string {
	if arg.Flavor == FLAVOR_SIMPLE {
		switch arg.Type {
		case "CHAR", "VARCHAR2":
			return "string"
		case "NUMBER":
			return "float64"
		case "INTEGER":
			return "int64"
		case "PLS_INTEGER", "BINARY_INTEGER":
			return "int32"
		case "DATE", "DATETIME", "TIME", "TIMESTAMP":
			return "time.Time"
		case "BOOLEAN", "PL/SQL BOOLEAN":
			return "bool"
		case "REF CURSOR":
			return "*goracle.Cursor"
		case "CLOB":
			return "*goracle.Clob"
		case "BLOB":
			return "*goracle.Blob"
		default:
			log.Fatalf("unknown simple type %s (%s)", arg.Type, arg)
		}
	}
	typName := arg.TypeName
	chunks := strings.Split(typName, ".")
	switch len(chunks) {
	case 1:
	case 2:
		typName = chunks[1] + "__" + chunks[0]
	default:
		typName = strings.Join(chunks[1:], "__") + "__" + chunks[0]
	}
	typName = capitalize(typName)
	if _, ok := typedefs[typName]; ok {
		return typName
	}
	if arg.Flavor == FLAVOR_TABLE {
		glog.Infof("arg=%s tof=%s", arg, arg.TableOf)
		return "[]" + arg.TableOf.goType(typedefs)
	}
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	buf.WriteString("\n// " + arg.TypeName + "\n")
	buf.WriteString("type " + typName + " struct {\n")
	for k, v := range arg.RecordOf {
		buf.WriteString("\t" + capitalize(goName(k)) + " " + v.goType(typedefs) + "\n")
	}
	buf.WriteString("}\n")
	typedefs[typName] = buf.String()
	return typName
}

func goName(text string) string {
	if text == "" {
		return text
	}
	if text[len(text)-1] == '#' {
		return text[:len(text)-1] + "匿" // 0x533f = hide
	}
	return text
}
