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
	"google.golang.org/protobuf/types/pluginpb"
)

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

	// Protoc passes a slice of File structs for us to process
	for _, file := range plugin.Files {
		// log.Printf("? %q", file.GoImportPath.String())
		if strings.HasPrefix(file.GoImportPath.String(), `"google.golang.org/`) {
			log.Println("skip", file.GoImportPath.String())
			continue
		}
		// var messageOption, fieldOption *protogen.Message
		// for _, e := range file.Extensions {
		// 	switch e.Extendee.Desc.FullName() {
		// 	case "google.protobuf.MessageOptions":
		// 		messageOption = e.Extendee
		// 	case "google.protobuf.FieldOptions":
		// 		fieldOption = e.Extendee
		// 	}
		// }

		// Time to generate code...!
		var buf bytes.Buffer

		// 2. Write the package name
		fmt.Fprintf(&buf, "// Code generated by protoc-gen-oracall. DO NOT EDIT!\n\npackage %s\n\n", file.GoPackageName)

		for _, msg := range file.Proto.MessageType {
			// FIXME[tgulacs]: this is a hack, but I couldn't find out how to get the extensions' options' values
			optS := msg.Options.String()
			opts, err := parseOption(optS)
			if err != nil {
				return fmt.Errorf("parse %q: %w", optS, err)
			}
			objectType := opts["objects.oracall_object_type"]
			fmt.Fprintf(&buf, "\n// %q=%q\nfunc(%s) ObjecTypeName() string { return %q }\n",
				*msg.Name, objectType,
				*msg.Name, objectType,
			)
			fmt.Fprintf(&buf, "func (%s) FieldTypeName(f string) string{\n\tswitch f {\n", msg.GetName())
			for _, f := range msg.Field {
				optS := f.Options.String()
				opts, err := parseOption(optS)
				if err != nil {
					return fmt.Errorf("parse %q: %w", optS, err)
				}
				fieldType := opts["objects.oracall_field_type"]
				if fieldType == "" {
					log.Printf("opts=%q fields=%+v", optS, opts)
				}
				fmt.Fprintf(&buf, "\t\t// %q.%q = %q\n\t\tcase %q, %q: return %q\n",
					*msg.Name, oracall.CamelCase(f.GetName()), fieldType,
					f.GetName(), oracall.CamelCase(f.GetName()), fieldType,
				)
			}
			fmt.Fprintf(&buf, "}\n\treturn \"\"\n}\n")
		}

		// 4. Specify the output filename, in this case test.foo.go
		filename := file.GeneratedFilenamePrefix + ".oracall.go"
		file := plugin.NewGeneratedFile(filename, ".")

		// 5. Pass the data from our buffer to the plugin file struct
		out, err := format.Source(buf.Bytes())
		if err != nil {
			return err
		}
		file.Write(out)
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

// parseOption parses the [option.name]:"value" stringified option.
func parseOption(s string) (map[string]string, error) {
	k, v, ok := strings.Cut(s, ":")
	if !ok {
		return nil, fmt.Errorf("no : in %q", s)
	}
	if w, err := strconv.Unquote(v); err != nil {
		return nil, fmt.Errorf("unquote %q: %w", v, err)
	} else {
		v = w
	}
	return map[string]string{strings.Trim(k, "[]"): v}, nil
}
