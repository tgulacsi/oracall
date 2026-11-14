// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package objects

import (
	"fmt"
	"strings"
	"unicode"
)

// Shape is the argument shape the generator distinguishes.
type Shape int

const (
	ShapeUnknown Shape = iota
	ShapeSimple
	ShapeRecord
	ShapeCollection
)

func (s Shape) String() string {
	switch s {
	case ShapeSimple:
		return "simple"
	case ShapeRecord:
		return "record"
	case ShapeCollection:
		return "collection"
	default:
		return "unknown"
	}
}

// ShapeOf classifies t into simple, record, collection, or unknown.
func (t *Type) ShapeOf() Shape {
	if t == nil {
		return ShapeUnknown
	}
	if t.IsColl() {
		return ShapeCollection
	}
	if len(t.Arguments) > 0 {
		return ShapeRecord
	}
	if t.Elem == nil && t.Name != "" {
		return ShapeSimple
	}
	return ShapeUnknown
}

// QualifiedName returns the fully qualified Oracle type name as
// OWNER.PACKAGE.NAME, omitting empty components (e.g. OWNER.NAME for a
// schema-level type).
func (t *Type) QualifiedName() string {
	if t == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	for _, p := range [...]string{t.Owner, t.Package, t.Name} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ".")
}

// GoTypeNameFromQualified derives a stable, exported Go identifier from a
// qualified Oracle type name, e.g. "BRUNO.PKG.REC_TYP" -> "BrunoPkgRecTyp".
func GoTypeNameFromQualified(qname string) string {
	var b strings.Builder
	upper := true
	for _, r := range qname {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if upper {
				b.WriteRune(unicode.ToUpper(r))
				upper = false
			} else {
				b.WriteRune(unicode.ToLower(r))
			}
		} else {
			upper = true
		}
	}
	s := b.String()
	if s != "" && unicode.IsDigit(rune(s[0])) {
		s = "T" + s
	}
	return s
}

// TypeRegistry maps qualified Oracle type names to unique Go type names, so a
// shared type is generated once and name collisions are resolved
// deterministically.
type TypeRegistry struct {
	byQual map[string]string
	byGo   map[string]string
	order  []string
}

// NewTypeRegistry returns an empty registry.
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{byQual: make(map[string]string), byGo: make(map[string]string)}
}

// NameOf registers qname (on first sight) and returns its Go type name.
// A colliding base name gets a numeric suffix (_2, _3, ...).
func (r *TypeRegistry) NameOf(qname string) string {
	if n, ok := r.byQual[qname]; ok {
		return n
	}
	base := GoTypeNameFromQualified(qname)
	if base == "" {
		base = "OraType"
	}
	name := base
	for i := 2; ; i++ {
		if owner, ok := r.byGo[name]; !ok || owner == qname {
			break
		}
		name = fmt.Sprintf("%s_%d", base, i)
	}
	r.byQual[qname] = name
	r.byGo[name] = qname
	r.order = append(r.order, qname)
	return name
}

// Registered returns the qualified names in first-registration order.
func (r *TypeRegistry) Registered() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// ValidateShape checks that t and its nested types use only the supported
// shapes: simple, record-of-simple, array-of-simple, and
// array-of-record-of-simple. It returns a clear error for the unsupported
// array-of-array, array-in-record, and nested-record shapes.
func (t *Type) ValidateShape() error {
	if t == nil {
		return fmt.Errorf("nil type")
	}
	return validateShape(t, t.QualifiedName())
}

func validateShape(t *Type, path string) error {
	switch t.ShapeOf() {
	case ShapeSimple:
		return nil
	case ShapeRecord:
		for _, a := range t.Arguments {
			if a.Type == nil {
				return fmt.Errorf("%s.%s: unresolved record field type", path, a.Name)
			}
			switch a.Type.ShapeOf() {
			case ShapeSimple:
				// supported
			case ShapeCollection:
				return fmt.Errorf("%s.%s: array in record is not supported", path, a.Name)
			case ShapeRecord:
				return fmt.Errorf("%s.%s: nested record is not supported", path, a.Name)
			default:
				return fmt.Errorf("%s.%s: unsupported field type %q", path, a.Name, a.Type.Name)
			}
		}
		return nil
	case ShapeCollection:
		if t.Elem == nil {
			return fmt.Errorf("%s: collection has no element type", path)
		}
		switch t.Elem.ShapeOf() {
		case ShapeSimple:
			return nil
		case ShapeCollection:
			return fmt.Errorf("%s: array of array is not supported", path)
		case ShapeRecord:
			return validateShape(t.Elem, path+"[]")
		default:
			return fmt.Errorf("%s: unsupported collection element type %q", path, t.Elem.Name)
		}
	default:
		return fmt.Errorf("%s: unsupported/unknown type shape (%q)", path, t.Name)
	}
}
