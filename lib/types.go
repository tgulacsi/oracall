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
			case "DATE":
				return fmt.Sprintf("%s = string(custom.NewDate(%s))", dst, varName)
			}
		}
	}
	switch arg.ora {
	case "DATE":
		return fmt.Sprintf("%s = string(%s)", dst, src)
	case "PLS_INTEGER":
		return fmt.Sprintf("%s = int32(%s)", dst, src)
	case "NUMBER":
		return fmt.Sprintf("%s = string(%s)", dst, src)
	}
	return fmt.Sprintf("%s = %s // %s fromOra", dst, src, arg.ora)
}

func (arg PlsType) GetOra(src, varName string) string {
	if Gogo {
		switch arg.ora {
		case "DATE":
			if varName != "" {
				return fmt.Sprintf("string(custom.NewDate(%s))", varName)
			}
			return fmt.Sprintf("string(custom.AsDate(%s))", src)
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
func (arg PlsType) ToOra(dst, src string, isOutput bool) (expr string, variable string) {
	dstVar := mkVarName(dst)
	if Gogo {
		switch arg.ora {
		case "DATE": // custom.Date
			var pointer string
			if src[0] == '&' {
				pointer = "&"
			}
			if isOutput {
				return fmt.Sprintf(`%s, convErr := custom.Date(%s).Get()
			if convErr != nil { // toOra D
				err = errors.Wrap(oracall.ErrInvalidArgument, convErr.Error())
				return
			}
			%s = sql.Out{Dest:%s%s, In:true}`,
						dstVar, strings.TrimPrefix(src, "&"),
						dst, pointer, dstVar,
					),
					dstVar
			}
			return fmt.Sprintf(`%s, convErr := custom.Date(%s).Get() // toOra D
			if convErr != nil {
				err = errors.Wrap(oracall.ErrInvalidArgument, convErr.Error())
				return
			}
			%s = %s%s`,
					dstVar, strings.TrimPrefix(src, "&"),
					dst, pointer, dstVar,
				),
				dstVar
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
	}
	if isOutput && !(strings.HasSuffix(dst, "]") && !strings.HasPrefix(dst, "params[")) {
		if arg.ora == "NUMBER" {
			return fmt.Sprintf("%s = sql.Out{Dest:(*goracle.Number)(unsafe.Pointer(%s)),In:true} // NUMBER",
				dst, src), ""
		}
		return fmt.Sprintf("%s = sql.Out{Dest:%s,In:true} // %s", dst, src, arg.ora), ""
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
