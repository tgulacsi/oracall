/*
Copyright 2017 Tamás Gulácsi

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

package oracall

import (
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"io"
	"strings"
)

type PlsType struct {
	ora string
}

func (arg PlsType) String() string { return arg.ora }

// NewArg returns a new argument to ease arument conversions.
func NewPlsType(ora string) PlsType {
	return PlsType{ora: ora}
}

// FromOra retrieves the value of the argument with arg type, from src variable to dst variable.
func (arg PlsType) FromOra(dst, src, varName string) string {
	if Gogo {
		if varName != "" {
			switch arg.ora {
			case "DATE", "TIMESTAMP":
				return fmt.Sprintf("%s = &custom.DateTime{Time:%s}", dst, varName)
			}
		}
	}
	switch arg.ora {
	case "BLOB":
		if varName != "" {
			return fmt.Sprintf("{%s, err = ioutil.ReadAll(%s)", dst, varName)
		}
		return fmt.Sprintf("%s = goracle.Lob{Reader:bytes.NewReader(%s)}", dst, src)
	case "CLOB":
		if varName != "" {
			return fmt.Sprintf("{var b []byte; b, err = ioutil.ReadAll(%s); %s = string(b)}", varName, dst)
		}
		return fmt.Sprintf("%s = goracle.Lob{IsClob:true, Reader:strings.NewReader(%s)}", dst, src)
	case "DATE", "TIMESTAMP":
		return fmt.Sprintf("%s = (%s)", dst, src)
	case "PLS_INTEGER":
		return fmt.Sprintf("%s = int32(%s)", dst, src)
	case "NUMBER":
		return fmt.Sprintf("%s = string(%s)", dst, src)
	case "":
		panic(fmt.Sprintf("empty \"ora\" type: %#v", arg))
	}
	return fmt.Sprintf("%s = %s // %s fromOra", dst, src, arg.ora)
}

func (arg PlsType) GetOra(src, varName string) string {
	if Gogo {
		switch arg.ora {
		case "DATE":
			if varName != "" {
				return fmt.Sprintf("%s.Format(time.RFC3339)", varName)
			}
			return fmt.Sprintf("custom.AsDate(%s)", src)
		}
	}
	switch arg.ora {
	case "NUMBER":
		if varName != "" {
			//return fmt.Sprintf("string(%s.(goracle.Number))", varName)
			return fmt.Sprintf("custom.AsString(%s)", varName)
		}
		//return fmt.Sprintf("string(%s.(goracle.Number))", src)
		return fmt.Sprintf("custom.AsString(%s)", src)
	}
	return src
}

// ToOra adds the value of the argument with arg type, from src variable to dst variable.
func (arg PlsType) ToOra(dst, src string, dir direction) (expr string, variable string) {
	dstVar := mkVarName(dst)
	var inTrue string
	if dir.IsInput() {
		inTrue = ",In:true"
	}
	if Gogo {
		switch arg.ora {
		case "DATE": // custom.Date = *time.Time
			//var pointer string
			//if src[0] == '&' {
			//pointer = "&"
			//}
			np := strings.TrimPrefix(src, "&")
			if dir.IsOutput() {
				if !strings.HasPrefix(dst, "params[") {
					return fmt.Sprintf(`%s = *custom.AsDate(%s)`, dst, np), ""
				}
				return fmt.Sprintf(`if %s == nil { %s = new(custom.DateTime) }
					%s = sql.Out{Dest:&%s.Time%s}`,
						np, np,
						dst, strings.TrimPrefix(src, "&"), inTrue,
					),
					""
			}
			return fmt.Sprintf(`%s = *custom.AsDate(%s) // toOra D`, dst, np), ""
		}
	}
	switch arg.ora {
	case "PLS_INTEGER":
		if src[0] != '&' {
			return fmt.Sprintf("var %s sql.NullInt64; if %s != 0 { %s.Int64, %s.Valid = int64(%s), true }; %s = int32(%s.Int64)", dstVar, src, dstVar, dstVar, src, dst, dstVar), dstVar
		}
	case "NUMBER":
		if src[0] != '&' {
			return fmt.Sprintf("%s := goracle.Number(%s); %s = %s", dstVar, src, dst, dstVar), dstVar
		}
	case "CLOB":
		if dir.IsOutput() {
			return fmt.Sprintf("%s := goracle.Lob{IsClob:true}; %s = sql.Out{Dest:&%s}", dstVar, dst, dstVar), dstVar
		}
		return fmt.Sprintf("%s := goracle.Lob{IsClob:true,Reader:strings.NewReader(%s)}; %s = %s", dstVar, src, dst, dstVar), dstVar
	}
	if dir.IsOutput() && !(strings.HasSuffix(dst, "]") && !strings.HasPrefix(dst, "params[")) {
		if arg.ora == "NUMBER" {
			return fmt.Sprintf("%s = sql.Out{Dest:(*goracle.Number)(unsafe.Pointer(%s))%s} // NUMBER",
				dst, src, inTrue), ""
		}
		return fmt.Sprintf("%s = sql.Out{Dest:%s%s} // %s", dst, src, inTrue, arg.ora), ""
	}
	return fmt.Sprintf("%s = %s // %s", dst, src, arg.ora), ""
}

func mkVarName(dst string) string {
	h := fnv.New64()
	io.WriteString(h, dst)
	var raw [8]byte
	var enc [8 * 2]byte
	hex.Encode(enc[:], h.Sum(raw[:0]))
	return fmt.Sprintf("var_%s", enc[:])
}

func ParseDigits(s string, precision, scale int) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if s[0] == '-' || s[0] == '+' {
		s = s[1:]
	}
	var dotSeen bool
	bucket := precision
	if precision == 0 && scale == 0 {
		bucket = 38
	}
	for _, r := range s {
		if !dotSeen && r == '.' {
			dotSeen = true
			if !(precision == 0 && scale == 0) {
				bucket = scale
			}
			continue
		}
		if '0' <= r && r <= '9' {
			bucket--
			if bucket < 0 {
				return fmt.Errorf("want NUMBER(%d,%d), has %q", precision, scale, s)
			}
		} else {
			return fmt.Errorf("want number, has %c in %q", r, s)
		}
	}
	return nil
}

// vim: set fileencoding=utf-8 noet:
