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
	} else {
		switch arg.ora {
		case "DATE":
			return fmt.Sprintf("%s = string(%s)", dst, src)
		}
	}
	return fmt.Sprintf("%s = %s", dst, src)
}

// ToOra adds the value of the argument with arg type, from src variable to dst variable.
func (arg PlsType) ToOra(dst, src string) (expr string, variable string) {
	if Gogo {
		switch arg.ora {
		case "DATE": // custom.Date
			dstVar := mkVarName(dst)
			return fmt.Sprintf("%s := %s.Get(); %s = &%s", dstVar, strings.TrimPrefix(src, "&"), dst, dstVar), dstVar
		}
	}
	return fmt.Sprintf("%s = %s", dst, src), ""
}

func mkVarName(dst string) string {
	return "var_" + strings.Map(func(r rune) rune {
		if r == '[' || r == ']' {
			return '_'
		}
		return r
	}, dst)
}
