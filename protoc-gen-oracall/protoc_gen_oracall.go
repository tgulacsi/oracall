// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	oracall "github.com/tgulacsi/oracall/lib"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	// "google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/pluginpb"
)

//go:generate bash -c "sed -e '/go_package =/ s,lib/objects,protoc-gen-oracall,' ../lib/objects/testdata/x.proto > testdata/x.proto"
func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	// Protoc passes pluginpb.CodeGeneratorRequest in via stdin
	// marshalled with Protobuf
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("slurp stdin: %w", err)
	}
	var req pluginpb.CodeGeneratorRequest
	proto.Unmarshal(input, &req)

	// Initialise our plugin with default options
	opts := protogen.Options{}
	plugin, err := opts.New(&req)
	if err != nil {
		panic(err)
	}

	// https://github.com/golang/protobuf/issues/1260
	// The type information for all extensions is in the source files,
	// so we need to extract them into a dynamically created protoregistry.Types.
	extTypes := new(protoregistry.Types)
	for _, file := range plugin.Files {
		if err := registerAllExtensions(extTypes, file.Desc); err != nil {
			return fmt.Errorf("registerAllExtensions: %w", err)
		}
	}

	// Protoc passes a slice of File structs for us to process
	for _, file := range plugin.Files {
		// log.Printf("? %q", file.GoImportPath.String())
		if strings.HasPrefix(file.GoImportPath.String(), `"google.golang.org/`) {
			log.Println("skip", file.GoImportPath.String())
			continue
		}

		// Time to generate code...!
		var buf, tBuf bytes.Buffer

		// 2. Write the package name
		buf.WriteString(`// Code generated by protoc-gen-oracall. DO NOT EDIT!
package ` + string(file.GoPackageName) + `

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
		
	"github.com/godror/godror"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	_ context.Context
	_ = io.ReadAll
	_ = strconv.FormatFloat
	_ strings.Builder
	_ time.Time
	_ = timestamppb.New
	_ godror.Number
)

func getString(d *godror.Data) string {
	switch d.NativeTypeNum {
	case 3004:  //godror.NativeTypeBytes 
		return string(d.GetBytes()) 
	case 3003: //godror.NativeTypeDouble
		return strconv.FormatFloat(d.GetFloat64(), 'f', -1, 64)
	default:
		fmt.Printf("Get=%#v, ntt=%d\n", d.Get(), d.NativeTypeNum )
		return fmt.Sprintf("%v", d.Get())	
	}
}

`)

		tBuf.WriteString(`// Code generated by protoc-gen-oracall. DO NOT EDIT!
package ` + string(file.GoPackageName) + `_test

import (
	"context"
	"database/sql"
	"flag"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
		
	"github.com/google/go-cmp/cmp"
	"github.com/UNO-SOFT/zlog/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	godror "github.com/godror/godror"

	` + file.GoImportPath.String() + `
)

var (
	_ context.Context
	_ = timestamppb.New
	_ = cmp.Diff
	_ = zlog.NewT

	pjMO = protojson.MarshalOptions{Indent:"  "}
)

var (
	flagConnect = flag.String("connect", os.Getenv("ORACALL_DSN"), "DSN to connect with")
	db *sql.DB
	dbOnce sync.Once
)
func getTx(ctx context.Context, t *testing.T) (*sql.Tx, error) {
	var err error
	dbOnce.Do(func() { 
		if db, err = sql.Open("godror", *flagConnect); err == nil {
			cv, _ := godror.ClientVersion(ctx, db)
			sv, _ := godror.ServerVersion(ctx, db)
			t.Logf("Client: %+v; Server: %+v", cv, sv)
		} 
	})
	if err != nil { return nil, err }
	return db.BeginTx(ctx, nil)
}
func fakeData(t *testing.T, dest any, fieldType string) {
    // t.Logf("fakeData: %T (%q)", dest, fieldType)
    var typ string
    var length, precision, scale int
    if fieldType != "" {
		if typ, rest, ok := strings.Cut(strings.TrimSuffix(fieldType, ")"), "("); ok {
			if strings.Contains(typ, "CHAR") {
				length, _ = strconv.Atoi(strings.TrimSuffix(rest, ")"))
			} else if precS, scaleS, ok := strings.Cut(rest, ","); ok {
				precision, _ = strconv.Atoi(precS)
				scale, _ = strconv.Atoi(scaleS)
			} else {
				precision, _ = strconv.Atoi(rest)
			}
		}
	}
	const num38 = "1234567890" + "1234567890" + "1234567890" + "12345678"
	switch x := dest.(type) {
		case *time.Time: *x = time.Now().UTC().Truncate(time.Second); return
		case *timestamppb.Timestamp: *x = *(timestamppb.New(time.Now().UTC().Truncate(time.Second))); return
		case **timestamppb.Timestamp: *x = timestamppb.New(time.Now().UTC().Truncate(time.Second)); return
		case *string: 
			if length > 0 {
				*x = strings.Repeat("x", length)
			} else if precision > 0 {
				if scale > 0 { 
					*x = num38[:precision-scale]+"."+num38[precision-scale:precision] 
				} else { 
					*x = num38[:precision] 
				}
			} else {
				if strings.HasSuffix(fieldType, "ROWID") {
					*x = ""
				} else {
					*x = "-16" 
				}
			} 
			return
	}
	rv := reflect.ValueOf(dest)
	if rv.IsNil() {
		rv.Set(reflect.New(rv.Type().Elem()))
	}
	rv = rv.Elem()
	rt := rv.Type()
	switch rt.Kind() {
		case reflect.Pointer: fakeData(t, rv.Interface(), fieldType)
		case reflect.Bool: rv.SetBool(true) 
		case reflect.Int:   rv.SetInt(-999999999) 
		case reflect.Int8:  rv.SetInt(-118) 
		case reflect.Int16: rv.SetInt(-32716) 
		case reflect.Int32: if precision == 0 || precision >= 9 { 
			rv.SetInt(-999999932) 
		} else if precision >= 6 {
			rv.SetInt(-999932) 
		} else {
			rv.SetInt(-932)
		} 
		case reflect.Int64: rv.SetInt(-1234567890123456764) 
		case reflect.Uint:   rv.SetUint(999999999) 
		case reflect.Uint8:  rv.SetUint(118)
		case reflect.Uint16: rv.SetUint(32716)
		case reflect.Uint32: rv.SetUint(999999932)
		case reflect.Uint64: rv.SetUint(1234567890123456764)
		case reflect.Float32: rv.SetFloat(3.1432) 
		case reflect.Float64: rv.SetFloat(3.1464000000000003) 
		case reflect.String: 
			var s string
			if strings.Contains(typ, "CHAR") {
				if length > 0 {
					s = strings.Repeat("X", length)
				}
			} else if precision > 0 {
				if scale > 0 {
					s = num38[:precision-scale] + "." + num38[precision-scale:precision]
				} else {
					s = num38[:precision]
				}
			}
			if s == "" { s = "X" }
			rv.SetString(s)
			
		case reflect.Slice: 
			ss := reflect.MakeSlice(rt, 0, 3)
			ret := rt.Elem()
			ptr := ret.Kind() == reflect.Pointer
			if ptr {
				ret = ret.Elem()
			}
			re := reflect.New(ret)
			fakeData(t, re.Interface(), fieldType)
			if !ptr {
				re = re.Elem()
			}
			rv.Set(reflect.Append(ss, re, re))
			
		case reflect.Struct:
		    ftr, _ := dest.(interface{ FieldTypeName(string) string })
		    if ftr == nil {
			    ftr, _ = (rv.Interface()).(interface{ FieldTypeName(string) string })
		    }
			for i := range rt.NumField() {
				ft := rt.Field(i)
				if !ft.IsExported() {
					continue
				}
				var fieldType string
				if ftr == nil { 
					t.Logf("No ftr for %#v", dest)
				} else {
					if fieldType = ftr.FieldTypeName(ft.Name); fieldType == "" {
						t.Logf("empty %T.FieldTypeName(%s)", dest, ft.Name)
					}
				}
				var vv reflect.Value
				ptr := ft.Type.Kind() == reflect.Pointer
				if ptr {
					vv = reflect.New(ft.Type.Elem())
				} else {
					vv = reflect.New(ft.Type)
				}
				fakeData(t, vv.Interface(), fieldType)
				if ptr {
					rv.FieldByIndex(ft.Index).Set(vv)
				} else {
					rv.FieldByIndex(ft.Index).Set(vv.Elem())
				}
			}
	}
}
`)

		msgs := make([]message, 0, len(file.Messages))
		for _, msg := range file.Messages {
			objectType, err := getCustomOption(extTypes, msg.Desc.Options(), "oracall_object_type")
			if err != nil {
				return err
			}
			fields := msg.Desc.Fields()
			m := message{
				Name: string(msg.Desc.Name()), DBType: objectType.String(),
				Fields: make([]nameType, 0, fields.Len()),
			}
			for i := range fields.Len() {
				f := fields.Get(i)
				fieldType, err := getCustomOption(extTypes, f.Options(), "oracall_field_type")
				if err != nil {
					return err
				}
				nativeType := f.Kind().String()
				switch nativeType {
				case "float":
					nativeType = "float32"
				case "double":
					nativeType = "float64"
				case "message":
					switch nativeType = string(f.Message().FullName()); nativeType {
					case "google.protobuf.Timestamp":
						nativeType = "*timestamppb.Timestamp"
					default:
						nativeType = strings.TrimPrefix(nativeType, "objects.")
					}
				}
				m.Fields = append(m.Fields, nameType{
					Name: string(f.Name()), DBType: fieldType.String(),
					NativeType: nativeType,
					Repeated:   f.Cardinality() == protoreflect.Repeated,
				})
			}
			msgs = append(msgs, m)
		}

		for _, msg := range msgs {
			msg.writeTypeNames(&buf)
			msg.writeToFrom(&buf, &tBuf, string(file.GoPackageName))
		}

		for _, it := range []struct {
			fn  string
			buf *bytes.Buffer
		}{
			{"oracall", &buf}, {"oracall_test", &tBuf},
		} {
			// 4. Specify the output filename, in this case test.foo.go
			filename := file.GeneratedFilenamePrefix + "." + it.fn + ".go"
			file := plugin.NewGeneratedFile(filename, ".")

			// 5. Pass the data from our buffer to the plugin file struct
			out, err := format.Source(it.buf.Bytes())
			if err != nil {
				return fmt.Errorf("%s\n%w", it.buf.String(), err)
			}
			if _, err := file.Write(out); err != nil {
				return fmt.Errorf("write %q: %w", filename, err)
			}
		}
	}

	// Generate a response from our plugin and marshall as protobuf
	stdout := plugin.Response()
	out, err := proto.Marshal(stdout)
	if err != nil {
		return err
	}

	// Write the response to stdout, to be picked up by protoc
	_, err = os.Stdout.Write(out)
	return err
}

func (msg message) writeToFrom(w, tW io.Writer, pkg string) {
	FieldName := func(f nameType) string {
		fieldName := oracall.CamelCase(f.Name)
		if fieldName == "Size" {
			fieldName += "_"
		}
		return fieldName
	}
	// ToObject
	fmt.Fprintf(w, `func (x %s) ToObject(ctx context.Context, ex godror.Execer) (*godror.Object, error) {
	objT, err := godror.GetObjectType(ctx, ex, %q)
	if err != nil {
		return nil, fmt.Errorf("GetObjectType(%s): %%w", err)
	}
	obj, err := objT.NewObject()
	if err != nil {
		objT.Close(); 
		return nil, err
	}
	var d godror.Data
	if err := func() error {
`,
		msg.Name,
		msg.DBType,
		strings.ReplaceAll(msg.DBType, "%", "%%"),
	)
	for _, f := range msg.Fields {
		var fun, conv, getValue string
		switch typ, post, _ := strings.Cut(f.DBType, "("); typ {
		case "DATE", "TIMESTAMP":
			fun, conv = "Time", "%s.AsTime().In(time.Local)"
		case "CHAR", "VARCHAR2", "LONG", "PL/SQL ROWID", "CLOB":
			fun, conv = "Bytes", "[]byte(%s)"
		case "RAW", "LONG RAW", "BLOB":
			fun = "Bytes"
		case "PL/SQL PLS INTEGER", "PL/SQL BINARY INTEGER":
			fun, conv = "Int64", "int64(%s)"
		case "PL/SQL BOOLEAN":
			fun = "Bool"
		case "BINARY_DOUBLE":
			fun = "Float64"
		case "NUMBER", "INTEGER":
			if post != "" {
				precS, scaleS, _ := strings.Cut(post[:len(post)-1], ",")
				if precS != "*" {
					prec, err := strconv.Atoi(precS)
					if err != nil {
						panic(fmt.Errorf("parse %q as precision from %q: %w", precS, f.DBType, err))
					}
					if scaleS == "0" || scaleS == "" {
						if prec < 10 {
							fun, conv = "Int64", "int64(%s)"
						} else if prec < 20 {
							fun = "Int64"
						}
					} else if _, err := strconv.Atoi(scaleS); err != nil {
						panic(fmt.Errorf("parse %q as scale from %q: %w", scaleS, f.DBType, err))
					}
				}
			}
			if fun == "" {
				fun, conv = "Bytes", "[]byte(%s)"
			}
		case "PUBLIC.XMLTYPE", "SYS.XMLTYPE", "XMLTYPE":
			fun, conv = "Bytes", "[]byte(%s)"

		default:
			if strings.IndexByte(typ, '.') < 0 { // Object
				panic(fmt.Errorf("unknown type %q (%q)", typ, f.DBType))
			}
		}

		if conv == "" {
			conv = "%s"
		}
		fieldName := FieldName(f)
		if !f.Repeated {
			if fun == "" {
				getValue = `if x.%[1]s == nil { d.SetNull() } else {
				sub, err := x.%[1]s.ToObject(ctx, ex)
				if err != nil {return err}
				d.SetObject(sub)
				}`
			} else {
				if getValue == "" {
					getValue = fmt.Sprintf(conv, "x.%s")
				}
				getValue = fmt.Sprintf("d.Set%s(%s)\n", fun, getValue)
			}
			fmt.Fprintf(w, "%s\n", fmt.Sprintf(getValue, fieldName))
		} else {
			if fun == "" {
				switch f.NativeType {
				case "int32":
					getValue = "d.SetInt64(int64(e))"
				case "string":
					getValue = "d.SetBytes([]byte(e))"
				case "*timestamppb.Timestamp":
					getValue = "d.SetTime(e.AsTime().In(time.Local))"
				default:
					getValue = `{
					// ` + f.NativeType + `
				sub, err := e.ToObject(ctx, ex)
				if err != nil {return err}
				d.SetObject(sub)}`
				}
			} else {
				if getValue == "" {
					getValue = fmt.Sprintf(conv, "e")
				} else {
					getValue = fmt.Sprintf(getValue, "e")
				}
				getValue = fmt.Sprintf("d.Set%s(%s)", fun, getValue)
			}
			fmt.Fprintf(w, `
		{
		OT, err := godror.GetObjectType(ctx, ex, %q)
		if err != nil {return fmt.Errorf("NewObjectType(%s): %%w", err)}
		O, err := OT.NewObject()
		if err != nil {OT.Close(); return fmt.Errorf("NewObject(%s): %%w", err)}
		C := O.Collection()
		for _, e := range x.%s {
			%s
			if err = C.AppendData(&d); err != nil {
				O.Close()
				OT.Close()
				return fmt.Errorf("AppendData(%s): %%w", err)
			}
		}
		d.SetObject(C.Object)
		}
	`,
				f.DBType, f.DBType,
				strings.ReplaceAll(f.DBType, "%", "%%"),
				fieldName,
				getValue,
				strings.ReplaceAll(f.DBType, "%", "%%"),
			)
		}
		Name := strings.ToUpper(f.Name)
		fmt.Fprintf(w, `if err := godror.SetAttribute(ctx, ex, obj, %q, &d); err != nil {
		return fmt.Errorf("SetAttribute(attr=%s, data=%%+v): %%w", d, err)
	}
`,
			Name,
			Name,
		)
	}

	fmt.Fprintf(w, `
		return nil
	}(); err != nil { 
		obj.Close(); objT.Close(); 
		return nil, err 
	}
	return obj, nil
}
`)

	// FromObject
	fmt.Fprintf(w, "func (x *%s) FromObject(obj *godror.Object) error {\nvar d godror.Data\nx.Reset()\n", msg.Name)
	for _, f := range msg.Fields {
		nm := strings.ToUpper(oracall.SnakeCase(f.Name))
		fmt.Fprintf(w, "\tif err := obj.GetAttribute(&d, %q); err != nil {return fmt.Errorf(\"Get(%s): %%w\", err)}\n\tif !d.IsNull() {\n", nm, nm)
		var fun, conv, getValue string
		switch typ, post, _ := strings.Cut(f.DBType, "("); typ {
		case "DATE", "TIMESTAMP":
			fun, conv = "Time", "timestamppb.New"
		case "CHAR", "VARCHAR2", "LONG", "PL/SQL ROWID":
			fun, conv = "Bytes", "string"
		case "RAW", "LONG RAW":
			fun = "Bytes"
		case "BLOB":
			getValue = "v, err := io.ReadAll(d.GetLob()); if err != nil {return err}"
		case "CLOB":
			getValue = "var buf strings.Builder; if _, err := io.Copy(&buf, d.GetLob()); err != nil {return err}; v := buf.String()"
		case "PL/SQL PLS INTEGER", "PL/SQL BINARY INTEGER":
			fun, conv = "Int64", "int32"
		case "PL/SQL BOOLEAN":
			fun = "Bool"
		case "BINARY_DOUBLE":
			fun = "Float64"
		case "NUMBER", "INTEGER":
			if post != "" {
				precS, scaleS, _ := strings.Cut(post[:len(post)-1], ",")
				if precS != "*" {
					prec, err := strconv.Atoi(precS)
					if err != nil {
						panic(fmt.Errorf("parse %q as precision from %q: %w", precS, f.DBType, err))
					}
					if scaleS == "0" || scaleS == "" {
						if prec < 10 {
							fun, conv = "Int64", "int32"
						} else if prec < 20 {
							fun = "Int64"
						}
					} else if _, err := strconv.Atoi(scaleS); err != nil {
						panic(fmt.Errorf("parse %q as scale from %q: %w", scaleS, f.DBType, err))
					}
				}
			}
			if fun == "" {
				fun, conv = "Bytes", "string"
			}
		case "PUBLIC.XMLTYPE", "SYS.XMLTYPE", "XMLTYPE":
			fun, conv = "Bytes", "string"

		default:
			if strings.IndexByte(typ, '.') < 0 { // Object
				panic(fmt.Errorf("unknown type %q (%q)", typ, f.DBType))
			}
		}
		fieldName := FieldName(f)
		if !(getValue == "" && fun == "") {
			if getValue == "" {
				switch f.NativeType {
				case "string":
					getValue = "v := getString(&d)"
				case "*timestamppb.Timestamp":
					getValue = "v := timestamppb.New(d.GetTime())"
				default:
					getValue = fmt.Sprintf("v := %s(d.Get%s())", conv, fun)
				}
			} else if strings.Contains(getValue, "%") {
				getValue = fmt.Sprintf(getValue, fieldName)
			}
			if !f.Repeated {
				fmt.Fprintf(w, "%s\nx.%s = v\n", getValue, fieldName)
			} else {
				fmt.Fprintf(w, `{
	O := d.GetObject().Collection()
	length, err := O.Len()
	if err != nil { return err }
	x.%s = make([]%s, 0, length)
	for i, err := O.First(); err == nil; i, err = O.Next(i) {
		if O.CollectionOf.IsObject() {
			d.ObjectType = O.CollectionOf
		}
		if err = O.GetItem(&d, i); err != nil {
			return fmt.Errorf("%s.GetItem[%%d]: %%w", i, err)
		}
		%s
		x.%s = append(x.%s, v)
	}
}`,
					fieldName, f.NativeType,
					getValue,
					fieldName,
					fieldName, fieldName,
				)
			}
		} else if strings.IndexByte(f.DBType, '.') >= 0 {
			if !f.Repeated {
				fmt.Fprintf(w, `{
				var sub %s; 
				if err := sub.FromObject(d.GetObject()); err != nil {
					return err 
				}
				x.%s = &sub; 
				}`,
					f.NativeType, fieldName,
				)
			} else {
				nativeType, at := "*"+f.NativeType, '&'
				get := `if err := sub.FromObject(d.GetObject()); err != nil {
			return err
		}`
				switch f.NativeType {
				case "int32":
					nativeType, at = f.NativeType, ' '
					get = "sub = int32(d.GetInt64())"
				case "*timestamppb.Timestamp":
					nativeType, at = f.NativeType, ' '
					get = "sub = timestamppb.New(d.GetTime())"
				case "string":
					nativeType, at = f.NativeType, ' '
					get = `sub = string(d.GetBytes())`
				}
				fmt.Fprintf(w, `{
	O := d.GetObject().Collection()
	length, err := O.Len()
	if err != nil { return fmt.Errorf("%s.Len: %%w", err) }
	x.%s = make([]%s, 0, length)
	for i, err := O.First(); err == nil; i, err = O.Next(i) {
		if O.CollectionOf.IsObject() {
			d.ObjectType = O.CollectionOf
		}
		if err = O.GetItem(&d, i); err != nil {
			return fmt.Errorf("%s.GetItem[%%d]: %%w", i, err)
		}
		var sub %s
		%s
		x.%s = append(x.%s, %csub)
	}
				}`,
					fieldName,
					fieldName, nativeType,
					fieldName,
					f.NativeType,
					get,
					fieldName, fieldName, at,
				)
			}

		} else {
			// panic(fmt.Errorf("not implemented (%q)", f.Type))
		}
		fmt.Fprintf(w, "}\n")
	}
	fmt.Fprintf(w, "\treturn nil\n}\n")

	fmt.Fprintf(tW, `func TestToFromObject_%s(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	logger := zlog.NewT(t).SLog()
	godror.SetLogger(logger.With("lib", "godror"))
	tx, err := getTx(ctx, t)
	if err != nil { t.Fatal(err) }
	defer tx.Rollback()
	var want, got %s.%s
	fakeData(t, &want, "")
	wantJ, err := pjMO.Marshal(&want)
	if err != nil { t.Fatalf("protojson.Marshal(%s): %%+v", err)}
	obj, err := want.ToObject(ctx, tx)
	if err != nil { 
		t.Fatalf("%s=%%s.ToObject: %%+v", wantJ, err)
	}
	if err = got.FromObject(obj); err != nil { t.Fatalf("%s.FromObject: %%+v", err) }
	gotJ, err := pjMO.Marshal(&got)
	if err != nil { t.Fatalf("protojson.Marshal(%s): %%+v", err)}
	t.Logf("got=%%s", gotJ)
	if d := cmp.Diff(string(wantJ), string(gotJ)); d != "" { 
		t.Error(d, "want="+string(wantJ), "\n", "got="+string(gotJ)) 
	}
	`,
		msg.Name,
		pkg, msg.Name,
		msg.Name,
		msg.Name,
		msg.Name,
		msg.Name,
	)
	fmt.Fprintf(tW, "}\n")
}

func (msg message) writeTypeNames(w io.Writer) {
	fmt.Fprintf(w, "\n// %q=%q\nfunc(%s) ObjecTypeName() string { return %q }\n",
		msg.Name, msg.DBType,
		msg.Name, msg.DBType,
	)
	fmt.Fprintf(w, "func (%s) FieldTypeName(f string) string{\n\tswitch f {\n", msg.Name)
	for _, field := range msg.Fields {
		fmt.Fprintf(w, "\t\t// %q.%q = %q\n\t\tcase %q, %q: return %q\n",
			msg.Name, oracall.CamelCase(field.Name), field.DBType,
			field.Name, oracall.CamelCase(field.Name), field.DBType,
		)
	}
	fmt.Fprintf(w, "}\n\treturn \"\"\n}\n")
}

func getCustomOption(extTypes *protoregistry.Types, options protoreflect.ProtoMessage, name string) (protoreflect.Value, error) {
	var value protoreflect.Value
	err := iterCustomOptions(extTypes, options, func(fd protoreflect.FieldDescriptor, v protoreflect.Value) error {
		if string(fd.Name()) == name {
			value = v
		}
		return nil
	})
	return value, err
}
func iterCustomOptions(extTypes *protoregistry.Types, options protoreflect.ProtoMessage, f func(protoreflect.FieldDescriptor, protoreflect.Value) error) error {
	// The MessageOptions as provided by protoc does not know about
	// dynamically created extensions, so they are left as unknown fields.
	// We round-trip marshal and unmarshal the options with
	// a dynamically created resolver that does know about extensions at runtime.
	// options := msg.Desc.Options().(*descriptorpb.MessageOptions)
	b, err := proto.Marshal(options)
	if err != nil {
		return fmt.Errorf("Marshal(%#v): %w", options, err)
	}
	options.(interface{ Reset() }).Reset()
	err = proto.UnmarshalOptions{Resolver: extTypes}.Unmarshal(b, options)
	if err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	// Use protobuf reflection to iterate over all the extension fields,
	// looking for the ones that we are interested in.
	options.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if !fd.IsExtension() {
			return true
		}
		if err = f(fd, v); err != nil {
			return false
		}
		// Make use of fd and v based on their reflective properties.
		return true
	})
	return err
}

// https://github.com/golang/protobuf/issues/1260
func registerAllExtensions(extTypes *protoregistry.Types, descs interface {
	Messages() protoreflect.MessageDescriptors
	Extensions() protoreflect.ExtensionDescriptors
}) error {
	mds := descs.Messages()
	for i := 0; i < mds.Len(); i++ {
		registerAllExtensions(extTypes, mds.Get(i))
	}
	xds := descs.Extensions()
	for i := 0; i < xds.Len(); i++ {
		if err := extTypes.RegisterExtension(dynamicpb.NewExtensionType(xds.Get(i))); err != nil {
			return err
		}
	}
	return nil
}

type (
	nameType struct {
		Name, NativeType, DBType string
		Repeated                 bool
	}

	message struct {
		Name, DBType string
		Fields       []nameType
	}
)
