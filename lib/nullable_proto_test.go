// Copyright 2026
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package oracall

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestProtobufPLSIntegerPresence(t *testing.T) {
	prev := NullablePLSInteger
	NullablePLSInteger = true
	t.Cleanup(func() { NullablePLSInteger = prev })
	args := []Argument{
		{Name: "p_count", Type: "PLS_INTEGER", AbsType: "INTEGER(10)", Flavor: FLAVOR_SIMPLE, Direction: DIR_IN},
		{Name: "p_values", Flavor: FLAVOR_TABLE, TableOf: &Argument{Type: "PLS_INTEGER", AbsType: "INTEGER(10)", Flavor: FLAVOR_SIMPLE, Direction: DIR_IN}, Direction: DIR_IN},
	}
	var buf strings.Builder
	if err := protoWriteMessageTyp(&buf, "TestMessage", make(map[string]struct{}), argDocs{}, args...); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "optional sint32 p_count = 1;") {
		t.Errorf("PLS_INTEGER must have presence: %s", got)
	}
	if !strings.Contains(got, "repeated sint32 p_values = 2;") {
		t.Errorf("repeated scalar must not become optional: %s", got)
	}
}

// This tests the ProtoJSON contract independently from protoc. A synthetic
// descriptor matches the optional sint32 produced by the generator.
func TestProtoJSONOptionalSint32(t *testing.T) {
	schema := &descriptorpb.FileDescriptorProto{
		Name:   proto.String("nullable_int32.proto"),
		Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name:      proto.String("Example"),
			OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("_value")}},
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name:           proto.String("value"),
				Number:         proto.Int32(1),
				Label:          descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:           descriptorpb.FieldDescriptorProto_TYPE_SINT32.Enum(),
				Proto3Optional: proto.Bool(true),
				OneofIndex:     proto.Int32(0),
			}},
		}},
	}
	file, err := protodesc.NewFile(schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	desc := file.Messages().ByName("Example")
	fd := desc.Fields().ByName("value")
	m := dynamicpb.NewMessage(desc)
	check := func(want string) {
		t.Helper()
		out, err := protojson.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != want {
			t.Fatalf("protojson: got %s, want %s", out, want)
		}
	}
	check("{}")
	m.Set(fd, protoreflect.ValueOfInt32(0))
	if !m.Has(fd) {
		t.Fatal("explicit zero must be present")
	}
	check(`{"value":0}`)
	m.Set(fd, protoreflect.ValueOfInt32(-17))
	check(`{"value":-17}`)

	for _, test := range []struct {
		input   string
		present bool
		value   int32
	}{
		{`{"value":null}`, false, 0},
		{`{}`, false, 0},
		{`{"value":0}`, true, 0},
		{`{"value":42}`, true, 42},
	} {
		m.Reset()
		if err := protojson.Unmarshal([]byte(test.input), m); err != nil {
			t.Fatal(err)
		}
		if m.Has(fd) != test.present {
			t.Errorf("%s: presence mismatch", test.input)
		}
		if test.present && m.Get(fd).Int() != int64(test.value) {
			t.Errorf("%s: got %d, want %d", test.input, m.Get(fd).Int(), test.value)
		}
	}
}

func TestPLSIntegerGeneratedBinds(t *testing.T) {
	prev := NullablePLSInteger
	NullablePLSInteger = true
	t.Cleanup(func() { NullablePLSInteger = prev })
	for _, dir := range []direction{DIR_IN, DIR_OUT, DIR_INOUT} {
		arg := Argument{Name: "p_count", Type: "PLS_INTEGER", Flavor: FLAVOR_SIMPLE, Direction: dir}
		in, out := arg.getConvSimple(nil, nil, "PCount", "params[0]")
		generated := strings.Join(append(in, out...), "\n")
		if !strings.Contains(generated, "sql.NullInt32") {
			t.Fatalf("dir=%v: missing nullable bind: %s", dir, generated)
		}
		if dir.IsInput() && !strings.Contains(generated, "custom.ToSQLInt32(input.PCount)") {
			t.Errorf("dir=%v: input loses nullable value: %s", dir, generated)
		}
		if dir.IsOutput() && !strings.Contains(generated, "custom.FromSQLInt32") {
			t.Errorf("dir=%v: output loses nullable value: %s", dir, generated)
		}
	}
}

// Opting in is deliberate: old generated schemas and signatures remain the
// default, avoiding accidental changes to existing clients.
func TestPLSIntegerDefaultUnchanged(t *testing.T) {
	prev := NullablePLSInteger
	NullablePLSInteger = false
	t.Cleanup(func() { NullablePLSInteger = prev })

	arg := Argument{Name: "p_count", Type: "PLS_INTEGER", AbsType: "INTEGER(10)", Flavor: FLAVOR_SIMPLE, Direction: DIR_IN}
	var buf strings.Builder
	if err := protoWriteMessageTyp(&buf, "TestMessage", make(map[string]struct{}), argDocs{}, arg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "sint32 p_count = 1;") || strings.Contains(buf.String(), "optional sint32") {
		t.Errorf("opt-out must emit the old scalar schema, got: %s", buf.String())
	}
	gotType, err := arg.goType(false)
	if err != nil {
		t.Fatal(err)
	}
	if gotType != "int32" {
		t.Errorf("opt-out Go type: got %q, want int32", gotType)
	}
	in, out := arg.getConvSimple(nil, nil, "PCount", "params[0]")
	generated := strings.Join(append(in, out...), "\n")
	if strings.Contains(generated, "custom.ToSQLInt32") || strings.Contains(generated, "custom.FromSQLInt32") {
		t.Errorf("opt-out unexpectedly emitted nullable bindings: %s", generated)
	}
}
