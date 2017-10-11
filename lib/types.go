/*
Copyright 2016 Tamás Gulácsi

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
		return fmt.Sprintf("%s = %s.Int64", dst, src)
	case "NUMBER":
		return fmt.Sprintf("%s = custom.AsFloat64(%s)", dst, src)
	}
	return fmt.Sprintf("%s = %s // %s", dst, src, arg.ora)
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
				return fmt.Sprintf(`%s := custom.Date(%s).Get() // toOra D
			%s = sql.Out{Dest:%s%s, In:true}`,
						dstVar, strings.TrimPrefix(src, "&"),
						dst, pointer, dstVar,
					),
					dstVar
			}
			return fmt.Sprintf(`%s := custom.Date(%s).Get() // toOra D
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
			return fmt.Sprintf("var %s sql.NullInt64; if %s != 0 { %s.Int64, %s.Valid = int64(%s), true }; %s = %s", dstVar, src, dstVar, dstVar, src, dst, dstVar), dstVar
		}
	case "NUMBER":
		if src[0] != '&' {
			return fmt.Sprintf("%s := goracle.Number(%s); %s = %s", dstVar, src, dst, dstVar), dstVar
		}
	}
	if isOutput {
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

// vim: set fileencoding=utf-8 noet:
