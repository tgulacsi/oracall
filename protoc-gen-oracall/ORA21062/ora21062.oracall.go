// Code generated by protoc-gen-oracall. DO NOT EDIT!
package ORA21062

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/godror/godror"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	_ context.Context
	_ = io.ReadAll
	_ strings.Builder
	_ time.Time
	_ = timestamppb.New
	_ godror.Number
)

// "Test_Ora21062_FontRt"="TEST_ORA21062.FONT_RT"
func (Test_Ora21062_FontRt) ObjecTypeName() string { return "TEST_ORA21062.FONT_RT" }
func (Test_Ora21062_FontRt) FieldTypeName(f string) string {
	switch f {
	// "Test_Ora21062_FontRt"."Bold" = "PL/SQL BOOLEAN"
	case "bold", "Bold":
		return "PL/SQL BOOLEAN"
	// "Test_Ora21062_FontRt"."Italic" = "PL/SQL BOOLEAN"
	case "italic", "Italic":
		return "PL/SQL BOOLEAN"
	// "Test_Ora21062_FontRt"."Underline" = "VARCHAR2(30)"
	case "underline", "Underline":
		return "VARCHAR2(30)"
	// "Test_Ora21062_FontRt"."Family" = "VARCHAR2(30)"
	case "family", "Family":
		return "VARCHAR2(30)"
	// "Test_Ora21062_FontRt"."Size" = "BINARY_DOUBLE"
	case "size_", "Size":
		return "BINARY_DOUBLE"
	// "Test_Ora21062_FontRt"."Strike" = "PL/SQL BOOLEAN"
	case "strike", "Strike":
		return "PL/SQL BOOLEAN"
	// "Test_Ora21062_FontRt"."Color" = "VARCHAR2(30)"
	case "color", "Color":
		return "VARCHAR2(30)"
	// "Test_Ora21062_FontRt"."Colorindexed" = "PL/SQL PLS INTEGER"
	case "colorindexed", "Colorindexed":
		return "PL/SQL PLS INTEGER"
	// "Test_Ora21062_FontRt"."Colortheme" = "PL/SQL PLS INTEGER"
	case "colortheme", "Colortheme":
		return "PL/SQL PLS INTEGER"
	// "Test_Ora21062_FontRt"."Colortint" = "BINARY_DOUBLE"
	case "colortint", "Colortint":
		return "BINARY_DOUBLE"
	// "Test_Ora21062_FontRt"."Vertalign" = "VARCHAR2(30)"
	case "vertalign", "Vertalign":
		return "VARCHAR2(30)"
	}
	return ""
}
func (x Test_Ora21062_FontRt) ToObject(ctx context.Context, ex godror.Execer) (*godror.Object, error) {
	objT, err := godror.GetObjectType(ctx, ex, "TEST_ORA21062.FONT_RT")
	if err != nil {
		return nil, fmt.Errorf("GetObjectType(TEST_ORA21062.FONT_RT): %w", err)
	}
	obj, err := objT.NewObject()
	if err != nil {
		objT.Close()
		return nil, err
	}
	var d godror.Data
	if err := func() error {
		d.SetBool(x.Bold)

		if err := obj.SetAttribute("BOLD", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=BOLD, data=%+v): %w", d, err)
		}
		d.SetBool(x.Italic)

		if err := obj.SetAttribute("ITALIC", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=ITALIC, data=%+v): %w", d, err)
		}
		d.SetBytes([]byte(x.Underline))

		if err := obj.SetAttribute("UNDERLINE", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=UNDERLINE, data=%+v): %w", d, err)
		}
		d.SetBytes([]byte(x.Family))

		if err := obj.SetAttribute("FAMILY", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=FAMILY, data=%+v): %w", d, err)
		}
		d.SetFloat64(x.Size_)

		if err := obj.SetAttribute("SIZE_", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=SIZE_, data=%+v): %w", d, err)
		}
		d.SetBool(x.Strike)

		if err := obj.SetAttribute("STRIKE", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=STRIKE, data=%+v): %w", d, err)
		}
		d.SetBytes([]byte(x.Color))

		if err := obj.SetAttribute("COLOR", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=COLOR, data=%+v): %w", d, err)
		}
		d.SetInt64(int64(x.Colorindexed))

		if err := obj.SetAttribute("COLORINDEXED", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=COLORINDEXED, data=%+v): %w", d, err)
		}
		d.SetInt64(int64(x.Colortheme))

		if err := obj.SetAttribute("COLORTHEME", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=COLORTHEME, data=%+v): %w", d, err)
		}
		d.SetFloat64(x.Colortint)

		if err := obj.SetAttribute("COLORTINT", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=COLORTINT, data=%+v): %w", d, err)
		}
		d.SetBytes([]byte(x.Vertalign))

		if err := obj.SetAttribute("VERTALIGN", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=VERTALIGN, data=%+v): %w", d, err)
		}

		return nil
	}(); err != nil {
		obj.Close()
		objT.Close()
		return nil, err
	}
	return obj, nil
}
func (x *Test_Ora21062_FontRt) FromObject(obj *godror.Object) error {
	var d godror.Data
	x.Reset()
	if err := obj.GetAttribute(&d, "BOLD"); err != nil {
		return fmt.Errorf("Get(BOLD): %w", err)
	}
	if !d.IsNull() {
		v := (d.GetBool())
		x.Bold = v
	}
	if err := obj.GetAttribute(&d, "ITALIC"); err != nil {
		return fmt.Errorf("Get(ITALIC): %w", err)
	}
	if !d.IsNull() {
		v := (d.GetBool())
		x.Italic = v
	}
	if err := obj.GetAttribute(&d, "UNDERLINE"); err != nil {
		return fmt.Errorf("Get(UNDERLINE): %w", err)
	}
	if !d.IsNull() {
		var v string
		if d.NativeTypeNum == 3004 { //godror.NativeTypeBytes
			v = string(d.GetBytes())
		} else {
			v = fmt.Sprintf("%v", d.Get())
		}

		x.Underline = v
	}
	if err := obj.GetAttribute(&d, "FAMILY"); err != nil {
		return fmt.Errorf("Get(FAMILY): %w", err)
	}
	if !d.IsNull() {
		var v string
		if d.NativeTypeNum == 3004 { //godror.NativeTypeBytes
			v = string(d.GetBytes())
		} else {
			v = fmt.Sprintf("%v", d.Get())
		}

		x.Family = v
	}
	if err := obj.GetAttribute(&d, "SIZE_"); err != nil {
		return fmt.Errorf("Get(SIZE_): %w", err)
	}
	if !d.IsNull() {
		v := (d.GetFloat64())
		x.Size_ = v
	}
	if err := obj.GetAttribute(&d, "STRIKE"); err != nil {
		return fmt.Errorf("Get(STRIKE): %w", err)
	}
	if !d.IsNull() {
		v := (d.GetBool())
		x.Strike = v
	}
	if err := obj.GetAttribute(&d, "COLOR"); err != nil {
		return fmt.Errorf("Get(COLOR): %w", err)
	}
	if !d.IsNull() {
		var v string
		if d.NativeTypeNum == 3004 { //godror.NativeTypeBytes
			v = string(d.GetBytes())
		} else {
			v = fmt.Sprintf("%v", d.Get())
		}

		x.Color = v
	}
	if err := obj.GetAttribute(&d, "COLORINDEXED"); err != nil {
		return fmt.Errorf("Get(COLORINDEXED): %w", err)
	}
	if !d.IsNull() {
		v := int32(d.GetInt64())
		x.Colorindexed = v
	}
	if err := obj.GetAttribute(&d, "COLORTHEME"); err != nil {
		return fmt.Errorf("Get(COLORTHEME): %w", err)
	}
	if !d.IsNull() {
		v := int32(d.GetInt64())
		x.Colortheme = v
	}
	if err := obj.GetAttribute(&d, "COLORTINT"); err != nil {
		return fmt.Errorf("Get(COLORTINT): %w", err)
	}
	if !d.IsNull() {
		v := (d.GetFloat64())
		x.Colortint = v
	}
	if err := obj.GetAttribute(&d, "VERTALIGN"); err != nil {
		return fmt.Errorf("Get(VERTALIGN): %w", err)
	}
	if !d.IsNull() {
		var v string
		if d.NativeTypeNum == 3004 { //godror.NativeTypeBytes
			v = string(d.GetBytes())
		} else {
			v = fmt.Sprintf("%v", d.Get())
		}

		x.Vertalign = v
	}
	return nil
}

// "Test_Ora21062_RichTextRt"="TEST_ORA21062.RICH_TEXT_RT"
func (Test_Ora21062_RichTextRt) ObjecTypeName() string {
	return "TEST_ORA21062.RICH_TEXT_RT"
}
func (Test_Ora21062_RichTextRt) FieldTypeName(f string) string {
	switch f {
	// "Test_Ora21062_RichTextRt"."Font" = "TEST_ORA21062.FONT_RT"
	case "font", "Font":
		return "TEST_ORA21062.FONT_RT"
	// "Test_Ora21062_RichTextRt"."Text" = "VARCHAR2(1000)"
	case "text", "Text":
		return "VARCHAR2(1000)"
	}
	return ""
}
func (x Test_Ora21062_RichTextRt) ToObject(ctx context.Context, ex godror.Execer) (*godror.Object, error) {
	objT, err := godror.GetObjectType(ctx, ex, "TEST_ORA21062.RICH_TEXT_RT")
	if err != nil {
		return nil, fmt.Errorf("GetObjectType(TEST_ORA21062.RICH_TEXT_RT): %w", err)
	}
	obj, err := objT.NewObject()
	if err != nil {
		objT.Close()
		return nil, err
	}
	var d godror.Data
	if err := func() error {
		if x.Font == nil {
			d.SetNull()
		} else {
			sub, err := x.Font.ToObject(ctx, ex)
			if err != nil {
				return err
			}
			d.SetObject(sub)
		}
		if err := obj.SetAttribute("FONT", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=FONT, data=%+v): %w", d, err)
		}
		d.SetBytes([]byte(x.Text))

		if err := obj.SetAttribute("TEXT", &d); err != nil {
			return fmt.Errorf("SetAttribute(attr=TEXT, data=%+v): %w", d, err)
		}

		return nil
	}(); err != nil {
		obj.Close()
		objT.Close()
		return nil, err
	}
	return obj, nil
}
func (x *Test_Ora21062_RichTextRt) FromObject(obj *godror.Object) error {
	var d godror.Data
	x.Reset()
	if err := obj.GetAttribute(&d, "FONT"); err != nil {
		return fmt.Errorf("Get(FONT): %w", err)
	}
	if !d.IsNull() {
		{
			var sub Test_Ora21062_FontRt
			if err := sub.FromObject(d.GetObject()); err != nil {
				return err
			}
			x.Font = &sub
		}
	}
	if err := obj.GetAttribute(&d, "TEXT"); err != nil {
		return fmt.Errorf("Get(TEXT): %w", err)
	}
	if !d.IsNull() {
		var v string
		if d.NativeTypeNum == 3004 { //godror.NativeTypeBytes
			v = string(d.GetBytes())
		} else {
			v = fmt.Sprintf("%v", d.Get())
		}

		x.Text = v
	}
	return nil
}