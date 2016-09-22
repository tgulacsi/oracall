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

package structs

import "fmt"

type Arg struct {
	ora string
}

// NewArg returns a new argument to ease arument conversions.
func NewArg(ora string) Arg {
	return Arg{ora: ora}
}

// FromOra retrieves the value of the argument with arg type, from src variable to dst variable.
func (arg Arg) FromOra(dst, src string) string {
	switch arg.ora {
	case "DATE":
		return fmt.Sprintf("%s = string(%s)", dst, src)
	default:
		return fmt.Sprintf("%s = %s", dst, src)
	}
}

// ToOra adds the value of the argument with arg type, from src variable to dst variable.
func (arg Arg) ToOra(dst, src string) string {
	return fmt.Sprintf("%s = %s", dst, src)
}
