// Copyright 2026
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package custom

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// ToSQLInt32 converts the presence of an optional protobuf int32 into an
// Oracle-compatible nullable value. In particular, a present zero remains valid.
func ToSQLInt32(p *int32) sql.NullInt32 {
	if p == nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: *p, Valid: true}
}

// FromSQLInt32 converts an Oracle nullable integer to a protobuf optional
// scalar. Invalid values become nil; a valid zero becomes a non-nil pointer.
func FromSQLInt32(n sql.NullInt32) *int32 {
	if !n.Valid {
		return nil
	}
	v := n.Int32
	return &v
}

const xsiNamespace = "http://www.w3.org/2001/XMLSchema-instance"

// SOAPInt32 is the XML Schema nillable form of an optional protobuf sint32.
// Use it at the SOAP boundary (not as the protobuf-generated Go field type).
// A nil Value is emitted as xsi:nil="true"; a present zero is emitted as 0.
//
// Example:
//
//	type Element struct {
//	    Count SOAPInt32 `xml:"count"`
//	}
//	element := Element{Count: SOAPInt32{Value: message.Count}}
//
// The XML Schema declaration for this field should use nillable="true".
type SOAPInt32 struct {
	Value *int32
}

func (n SOAPInt32) MarshalXML(enc *xml.Encoder, start xml.StartElement) error {
	if n.Value != nil {
		return enc.EncodeElement(*n.Value, start)
	}
	start.Attr = append(start.Attr,
		xml.Attr{Name: xml.Name{Local: "xmlns:xsi"}, Value: xsiNamespace},
		xml.Attr{Name: xml.Name{Local: "xsi:nil"}, Value: "true"},
	)
	return enc.EncodeElement("", start)
}

func (n *SOAPInt32) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	isNil := false
	for _, attr := range start.Attr {
		if attr.Name.Local != "nil" && attr.Name.Local != "xsi:nil" {
			continue
		}
		if attr.Name.Space == xsiNamespace || attr.Name.Local == "xsi:nil" {
			switch attr.Value {
			case "true", "1":
				isNil = true
			case "false", "0":
			default:
				return fmt.Errorf("invalid xsi:nil value %q", attr.Value)
			}
		}
	}
	var body string
	if err := dec.DecodeElement(&body, &start); err != nil {
		return err
	}
	if isNil {
		if strings.TrimSpace(body) != "" {
			return fmt.Errorf("xsi:nil integer element has nonempty content %q", body)
		}
		n.Value = nil
		return nil
	}
	v, err := strconv.ParseInt(strings.TrimSpace(body), 10, 32)
	if err != nil {
		return fmt.Errorf("invalid SOAP int32 %q: %w", body, err)
	}
	i := int32(v)
	n.Value = &i
	return nil
}
