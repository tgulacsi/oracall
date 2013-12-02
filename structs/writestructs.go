/*
Copyright 2013 Tamás Gulácsi

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

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"log"
	"strings"

	"github.com/golang/glog"
)

var _ = glog.Infof

func SaveFunctions(dst io.Writer, functions []Function, pkg string, skipFormatting bool) error {
	var err error

	if pkg != "" {
		if _, err = fmt.Fprintf(dst,
			"package "+pkg+`
import (
    "encoding/json"
    "errors"
    "log"
    "fmt"
    "time"    // for datetimes

    "github.com/tgulacsi/goracle/oracle"    // Oracle
)

var DebugLevel = uint(0)

var _ time.Time
var _ oracle.Cursor
var _ = errors.New

// FunctionCaller is a function which calls the stored procedure with
// the input struct, and returns the output struct as an interface{}
type FunctionCaller func(*oracle.Cursor, interface{}) (interface{}, error)

// Functions is the map of function name -> function
var Functions = make(map[string]FunctionCaller, %d)

type Unmarshaler interface {
    FromJSON([]byte) error
}

// InputFactories is the map of function name -> input factory
var InputFactories = make(map[string](func() Unmarshaler), %d)
`, len(functions), len(functions)); err != nil {
			return err
		}
	}
	types := make(map[string]string, 16)
	inits := make([]string, 0, len(functions))
	var b []byte
	for _, fun := range functions {
		fun.types = types
		for _, dir := range []bool{false, true} {
			if err = fun.SaveStruct(dst, dir); err != nil {
				return err
			}
		}
		plsBlock, callFun := fun.PlsqlBlock()
		if _, err = fmt.Fprintf(dst, "\nconst %s = `", fun.getPlsqlConstName()); err != nil {
			return err
		}
		if _, err = io.WriteString(dst, plsBlock); err != nil {
			return err
		}
		if _, err = io.WriteString(dst, "`\n"); err != nil {
			return err
		}

		if _, err = io.WriteString(dst, "\n"); err != nil {
			return err
		}
		if b, err = format.Source([]byte(callFun)); err != nil {
			if !skipFormatting {
				return fmt.Errorf("error saving function %s: %s\n%s", fun.Name(), err, callFun)
			}
			log.Printf("error saving function %s: %s", fun.Name(), err)
			b = []byte(callFun)
		}
		if _, err = dst.Write(b); err != nil {
			return err
		}
		inpstruct := fun.getStructName(false)
		inits = append(inits,
			fmt.Sprintf("\t"+`Functions["%s"] = func(cur *oracle.Cursor, input interface{}) (interface{}, error) {
        var inp %s
        switch x := input.(type) {
        case *%s: input = *x
        case %s: input = x
        default:
            return nil, fmt.Errorf("to call %s, the input must be %s, not %%T", input)
        }
        return Call_%s(cur, inp)
    }
    InputFactories["%s"] = func() Unmarshaler { return new(%s) }
            `,
				fun.Name(), inpstruct, inpstruct, inpstruct, fun.Name(), inpstruct,
				strings.Replace(fun.Name(), ".", "__", -1),
				fun.Name(), inpstruct))
	}
	for tn, text := range types {
		if b, err = format.Source([]byte(text)); err != nil {
			return fmt.Errorf("error saving type %s: %s\n%s", tn, err, text)
		}
		//if _, err = io.WriteString(dst, text); err != nil {
		if _, err = dst.Write(b); err != nil {
			return err
		}
	}

	if _, err = io.WriteString(dst, "func init() {\n"); err != nil {
		return err
	}
	for _, text := range inits {
		if _, err = io.WriteString(dst, text); err != nil {
			return err
		}
		if _, err = dst.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	if _, err = io.WriteString(dst, "}\n"); err != nil {
		return err
	}

	return nil
}

func (f Function) getPlsqlConstName() string {
	return capitalize(f.Package + "__" + f.name + "__plsql")
}

func (f Function) getStructName(out bool) string {
	dirname := "input"
	if out {
		dirname = "output"
	}
	return capitalize(f.Package + "__" + f.name + "__" + dirname)
}

func (f Function) SaveStruct(dst io.Writer, out bool) error {
	//glog.Infof("f=%s", f)
	dirmap, dirname := uint8(DIR_IN), "input"
	if out {
		dirmap, dirname = DIR_OUT, "output"
	}
	var (
		err                    error
		aName, structName, got string
		checks                 []string
	)
	args := make([]Argument, 0, len(f.Args))
	for _, arg := range f.Args {
		//glog.Infof("dir=%d map=%d => %d", arg.Direction, dirmap, arg.Direction&dirmap)
		if arg.Direction&dirmap > 0 {
			args = append(args, arg)
		}
	}
	//glog.Infof("args[%d]: %s", dirmap, args)

	if dirmap == uint8(DIR_IN) {
		checks = make([]string, 0, len(args)+1)
	}
	structName = f.getStructName(out)
	buf := bytes.NewBuffer(make([]byte, 0, 65536))
	if _, err = io.WriteString(buf,
		"\n// "+f.Name()+" "+dirname+"\ntype "+structName+" struct {\n"); err != nil {
		return err
	}

	for _, arg := range args {
		aName = capitalize(goName(arg.Name))
		got = arg.goType(f.types)
		lName := strings.ToLower(arg.Name)
		if _, err = io.WriteString(buf, "\t"+aName+" "+got+
			"\t`json:\""+lName+",omitempty\""+
			" xml:\""+lName+",omitempty\"`\n"); err != nil {
			return err
		}
		if checks != nil {
			checks = genChecks(checks, arg, f.types, "s")
		}
	}

	if _, err = io.WriteString(buf, "}\n"); err != nil {
		return err
	}

	if !out {
		if _, err = fmt.Fprintf(buf, `func (s *%s) FromJSON(data []byte) error {
    return json.Unmarshal(data, s)
}`, structName); err != nil {
			return err
		}
	}

	var b []byte
	if b, err = format.Source(buf.Bytes()); err != nil {
		return fmt.Errorf("error saving struct %q: %s\n%s", structName, err, buf.String())
	}
	dst.Write(b)

	if checks != nil {
		buf.Reset()
		if _, err = fmt.Fprintf(buf, "\n// Check checks input bounds for %s\nfunc (s %s) Check() error {\n", structName, structName); err != nil {
			return err
		}
		for _, line := range checks {
			if _, err = fmt.Fprintf(buf, line+"\n"); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(buf, "\n\treturn nil\n}\n"); err != nil {
			return err
		}
		if b, err = format.Source(buf.Bytes()); err != nil {
			return fmt.Errorf("error writing check of %s: %s\n%s", structName, err, buf.String())
		}
		if _, err = dst.Write(b); err != nil {
			return err
		}
	}
	return nil
}

func genChecks(checks []string, arg Argument, types map[string]string, base string) []string {
	aName := capitalize(goName(arg.Name))
	got := arg.goType(types)
	var name string
	if aName == "" {
		name = base
	} else {
		name = base + "." + aName
	}
	switch arg.Flavor {
	case FLAVOR_SIMPLE:
		switch got {
		case "string":
			checks = append(checks,
				fmt.Sprintf(`if len(%s) > %d {
        return errors.New("%s is longer then accepted (%d)")
    }`,
					name, arg.Charlength,
					name, arg.Charlength))
		case "*string":
			checks = append(checks,
				fmt.Sprintf(`if %s != nil && len(*%s) > %d {
        return errors.New("%s is longer then accepted (%d)")
    }`,
					name, name, arg.Charlength,
					name, arg.Charlength))
		case "NullString", "sql.NullString":
			checks = append(checks,
				fmt.Sprintf(`if %s.Valid && len(%s.String) > %d {
        return errors.New("%s is longer then accepted (%d)")
    }`,
					name, name, arg.Charlength,
					name, arg.Charlength))
		case "*int32", "*int64", "*float32", "*float64":
			if arg.Precision > 0 {
				cons := strings.Repeat("9", int(arg.Precision))
				checks = append(checks,
					fmt.Sprintf(`if %s != nil && (*%s <= -%s || *%s > %s) {
        return errors.New("%s is out of bounds (-%s..%s)")
    }`,
						name, name, cons, name, cons,
						name, cons, cons))
			}
		case "int64", "float64":
			if arg.Precision > 0 {
				cons := strings.Repeat("9", int(arg.Precision))
				checks = append(checks,
					fmt.Sprintf(`if (%s <= -%s || %s > %s) {
        return errors.New("%s is out of bounds (-%s..%s)")
    }`,
						name, cons, name, cons,
						name, cons, cons))
			}
		case "NullInt64", "NullFloat64", "sql.NullInt64", "sql.NullFloat64":
			if arg.Precision > 0 {
				vn := got[strings.Index(got, "Null")+4:]
				cons := strings.Repeat("9", int(arg.Precision))
				checks = append(checks,
					fmt.Sprintf(`if %s.Valid && (%s.%s <= -%s || %s.%s > %s) {
        return errors.New("%s is out of bounds (-%s..%s)")
    }`,
						name, name, vn, cons, name, vn, cons,
						name, cons, cons))
			}
		}
	case FLAVOR_RECORD:
		//checks = append(checks, "if "+name+MarkValid+" {")
		checks = append(checks, "if "+name+" != nil {")
		for k, sub := range arg.RecordOf {
			_ = k
			checks = genChecks(checks, sub, types, name)
		}
		checks = append(checks, "}")
	case FLAVOR_TABLE:
		plus := strings.Join(genChecks(nil, *arg.TableOf, types, "v"), "\n\t")
		if len(strings.TrimSpace(plus)) > 0 {
			checks = append(checks,
				fmt.Sprintf("\tfor _, v := range %s.%s {\n\t%s\n}", base, aName, plus))
		}
	default:
		log.Fatalf("unknown flavor %q", arg.Flavor)
	}
	return checks
}

func capitalize(text string) string {
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + strings.ToLower(text[1:])
}

func unocap(text string) string {
	i := strings.Index(text, "_")
	if i == 0 {
		return capitalize(text)
	}
	return strings.ToUpper(text[:i]) + "_" + strings.ToLower(text[i+1:])
}

// returns a go type for the argument's type
func (arg *Argument) goType(typedefs map[string]string) (typName string) {
	defer func() {
		if strings.HasPrefix(typName, "**") {
			typName = typName[1:]
		}
	}()
	if arg.goTypeName != "" {
		if strings.Index(arg.goTypeName, "__") > 0 {
			return "*" + arg.goTypeName
		}
		return arg.goTypeName
	}
	defer func() {
		arg.goTypeName = typName
	}()
	if arg.Flavor == FLAVOR_SIMPLE {
		switch arg.Type {
		case "CHAR", "VARCHAR2":
			return "*string"
		case "NUMBER":
			return "*float64"
		case "INTEGER":
			return "*int64"
		case "PLS_INTEGER", "BINARY_INTEGER":
			return "*int32"
		case "BOOLEAN", "PL/SQL BOOLEAN":
			return "*bool"
		case "DATE", "DATETIME", "TIME", "TIMESTAMP":
			return "*time.Time"
		case "REF CURSOR":
			return "*oracle.Cursor"
		case "CLOB", "BLOB":
			return "*oracle.ExternalLobVar"
		default:
			log.Fatalf("unknown simple type %s (%s)", arg.Type, arg)
		}
	}
	typName = arg.TypeName
	chunks := strings.Split(typName, ".")
	switch len(chunks) {
	case 1:
	case 2:
		typName = chunks[1] + "__" + chunks[0]
	default:
		typName = strings.Join(chunks[1:], "__") + "__" + chunks[0]
	}
	typName = capitalize(typName)
	if _, ok := typedefs[typName]; ok {
		return "*" + typName
	}
	if arg.Flavor == FLAVOR_TABLE {
		glog.Infof("arg=%s tof=%s", arg, arg.TableOf)
		return "[]" + arg.TableOf.goType(typedefs)
	}
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	if arg.TypeName == "" {
		glog.Warningf("arg has no TypeName: %#v\n%s", arg, arg)
		arg.TypeName = strings.ToLower(arg.Name)
	}
	buf.WriteString("\ntype " + typName + " struct {\n")
	for k, v := range arg.RecordOf {
		lName := strings.ToLower(k)
		buf.WriteString("\t" + capitalize(goName(k)) + " " +
			v.goType(typedefs) + "\t" +
			"`json:\"" + lName + ",omitempty\"" +
			" xml:\"" + lName + ",omitempty\"`\t\n")
	}
	buf.WriteString("}\n")
	typedefs[typName] = buf.String()
	return "*" + typName
}

func goName(text string) string {
	if text == "" {
		return text
	}
	if text[len(text)-1] == '#' {
		return text[:len(text)-1] + MarkHidden
	}
	return text
}
