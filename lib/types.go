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
	"fmt"
	"strings"
)

type PlsType struct {
	ora string
}

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
				return fmt.Sprintf("%s.Set(%s)", dst, varName)
			}
		}
	}
	switch arg.ora {
	case "DATE":
		return fmt.Sprintf("%s = string(%s)", dst, src)
	case "PLS_INTEGER":
		return fmt.Sprintf("%s = %s.Value", dst, src)
	case "NUMBER":
		return fmt.Sprintf("%s = %s.Value", dst, src)
	}
	return fmt.Sprintf("%s = %s // %s", dst, src, arg.ora)
}

func (arg PlsType) GetOra(src, varName string) string {
	if Gogo {
		switch arg.ora {
		case "DATE":
			if varName != "" {
				return fmt.Sprintf("custom.NewDate(%s)", varName)
			}
			return fmt.Sprintf("custom.AsDate(%s)", src)
		}
	}
	return src
}

// ToOra adds the value of the argument with arg type, from src variable to dst variable.
func (arg PlsType) ToOra(dst, src string) (expr string, variable string) {
	if Gogo {
		switch arg.ora {
		case "DATE": // custom.Date
			dstVar := mkVarName(dst)
			var pointer string
			if src[0] == '&' {
				pointer = "&"
			}
			return fmt.Sprintf(`%s := %s.Get()
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
			dstVar := mkVarName(dst)
			return fmt.Sprintf("%s := ora.Int32{IsNull:true}; if %s != 0 { %s.Value, %s.IsNull = %s, false }; %s = %s", dstVar, src, dstVar, dstVar, src, dst, dstVar), dstVar
		}
	case "NUMBER":
		if src[0] != '&' {
			dstVar := mkVarName(dst)
			return fmt.Sprintf("%s := ora.Float64{IsNull:true}; if %s != 0 { %s.Value, %s.IsNull = %s, false }; %s = %s", dstVar, src, dstVar, dstVar, src, dst, dstVar), dstVar
		}
	}
	return fmt.Sprintf("%s = %s // %s", dst, src, arg.ora), ""
}

func mkVarName(dst string) string {
	return "var_" + strings.Map(func(r rune) rune {
		if r == '[' || r == ']' {
			return '_'
		}
		return r
	}, dst)
}
