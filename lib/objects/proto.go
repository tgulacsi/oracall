// Copyright 2024 Tamás Gulácsi. All rights reserved.
//
// SPDX-Licens-Identifier: Apache-2.0

package objects

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

var ProtoImports = protoImports{
	"google/protobuf/timestamp.proto",
	"google/protobuf/descriptor.proto",
}

type protoImport string

func (p protoImport) String() string {
	return fmt.Sprintf("import %q;\n", string(p))
}

type protoImports []protoImport

func (p protoImports) String() string {
	var buf strings.Builder
	for _, s := range p {
		buf.WriteString(s.String())
	}
	return buf.String()
}

const (
	// a number between 1 and 536,870,911 with the following restrictions:
	// The given number must be unique among all fields for that message.
	// Field numbers 19,000 to 19,999 are reserved for the Protocol Buffers implementation. The protocol buffer compiler will complain if you use one of these reserved field numbers in your message.
	bigRandBetween_20e_536m = 79396128
	MessageTypeExtension    = bigRandBetween_20e_536m
	FieldTypeExtension      = bigRandBetween_20e_536m

	protoExtendTypeName  = "oracall_object_type"
	protoExtendFieldName = "oracall_field_type"
)

var ProtoExtends = protoExtends{
	`extend google.protobuf.MessageOptions {
  string ` + protoExtendTypeName + `= ` + strconv.Itoa(bigRandBetween_20e_536m) + `;
}`,
	`extend google.protobuf.FieldOptions {
  string ` + protoExtendFieldName + ` = ` + strconv.Itoa(bigRandBetween_20e_536m) + `;
}`,
}

type protoExtends []string

func (p protoExtends) String() string {
	return strings.Join(p, "\n")
}

type (
	Protobuf struct {
		Services []ProtoService
	}

	ProtoService struct {
		Name string
		RPCs []ProtoRPC
	}

	ProtoRPC struct {
		Name          string
		Input, Output ProtoMessage
	}

	ProtoMessage struct {
		Name       string
		Extensions []ProtoExtension
		Fields     []ProtoField
	}

	ProtoField struct {
		Extensions []ProtoExtension
		Name       string
		Type       ProtoType
		Number     int
		Repeated   bool
	}

	ProtoExtension struct {
		Name, Value string
	}

	ProtoType struct {
		Simple     string
		Message    *ProtoMessage
		Extensions []ProtoExtension
	}
)

func (p Protobuf) Print(w protoWriter) {
	seen := make(map[string]struct{})
	for _, svc := range p.Services {
		svc.print(seen, w)
	}
}

func (s ProtoService) Requires() []*ProtoMessage {
	seen := make(map[string]struct{})
	var msgs []*ProtoMessage
	for _, x := range s.RPCs {
		for _, m := range x.Requires() {
			if _, ok := seen[m.Name]; ok {
				continue
			}
			seen[m.Name] = struct{}{}
			msgs = append(msgs, m)
		}
	}
	more := true
	for more {
		more = false
		for _, m := range msgs {
			for _, f := range m.Fields {
				if f.Type.Simple == "" && f.Type.Message != nil {
					if _, ok := seen[f.Type.Message.Name]; !ok {
						more = true
						seen[f.Type.Message.Name] = struct{}{}
						msgs = append(msgs, f.Type.Message)
					}
				}
			}
		}
	}
	return msgs
}

func (s ProtoService) Print(w protoWriter) {
	s.print(nil, w)
}
func (s ProtoService) print(seen map[string]struct{}, w protoWriter) {
	fmt.Fprintf(w, "\n//// %s\n", s.Name)
	// service DbMmb2Ablak {
	for _, m := range s.Requires() {
		if seen != nil {
			if _, ok := seen[m.Name]; ok {
				continue
			}
			seen[m.Name] = struct{}{}
		}
		m.Print(w)
	}

	fmt.Fprintf(w, "service %s {\n", s.Name)
	for _, x := range s.RPCs {
		x.Print(w)
	}
	w.WriteString("}\n\n")
}

func (m ProtoRPC) Requires() []*ProtoMessage {
	return []*ProtoMessage{&m.Input, &m.Output}
}

func (m ProtoRPC) Print(w protoWriter) {
	//rpc DbMmb2Ablak_KontaktUjKarok(DbMmb2Ablak_KontaktUjKarok_Input) returns (DbMmb2Ablak_KontaktUjKarok_Output) {}
	fmt.Fprintf(w, "rpc %s(%s) returns (%s) {}\n", m.Name, m.Input.Name, m.Output.Name)
}

func (m ProtoMessage) Print(w protoWriter) {
	// message DbMmb2Ablak_KontaktIgfbErtHiba_Input {
	fmt.Fprintf(w, "message %s {\n", m.Name)
	for _, e := range m.Extensions {
		w.WriteString("option ")
		e.Print(w)
		w.WriteString(";\n")
	}
	for _, f := range m.Fields {
		f.Print(w)
	}
	w.WriteString("}\n")
}

func (f ProtoField) Print(w protoWriter) {
	// google.protobuf.Timestamp p_hataly = 2 [(oracall_field_type) = "DATE"];
	typ := f.Type.Simple
	if typ == "" {
		typ = f.Type.Message.Name
	}
	fmt.Fprintf(w, "\t%s %s = %d", typ, f.Name, f.Number)
	if len(f.Extensions) != 0 {
		w.WriteString(" [")
		for i, e := range f.Extensions {
			if i != 0 {
				w.WriteString(", ")
			}
			e.Print(w)
		}
		w.WriteString("]")
	}
	w.WriteString(";\n")
}

func (e ProtoExtension) Print(w protoWriter) {
	fmt.Fprintf(w, "(%s) = %q", e.Name, e.Value)
}

type protoWriter interface {
	io.Writer
	io.StringWriter
}
