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
	"os"
	"strings"
	"sync"
	"unicode"

	"github.com/pkg/errors"
)

var ErrMissingTableOf = errors.New("missing TableOf info")

func SaveFunctions(dst io.Writer, functions []Function, pkg string, skipFormatting bool) error {
	var err error
	w := errWriter{Writer: dst, err: &err}

	if pkg != "" {
		fmt.Fprintf(w,
			"package "+pkg+`
import (
    "encoding/json"
	"encoding/xml"
    "errors"
    "log"
    "fmt"
	"strings"
	"strconv"
    "time"    // for datetimes

    "gopkg.in/rana/ora.v3"    // Oracle
)

var DebugLevel = uint(0)

// against "unused import" error
var _ ora.Ses
var _ time.Time
var _ strconv.NumError
var _ strings.Reader
var _ = errors.New
var _ json.Marshaler
var _ xml.Name
var _ log.Logger
var _ = fmt.Printf

// FunctionCaller is a function which calls the stored procedure with
// the input struct, and returns the output struct as an interface{}
type FunctionCaller func(*ora.Ses, interface{}) (interface{}, error)

// Functions is the map of function name -> function
var Functions = make(map[string]FunctionCaller, %d)

type Unmarshaler interface {
    FromJSON([]byte) error
}

type Setter interface {
	Set(key string, value string) error
}

// InputFactories is the map of function name -> input factory
var InputFactories = make(map[string](func() Unmarshaler), %d)
`, len(functions), len(functions))
	}
	types := make(map[string]string, 16)
	inits := make([]string, 0, len(functions))
	var b []byte

FunLoop:
	for _, fun := range functions {
		fun.types = types
		for _, dir := range []bool{false, true} {
			if err := fun.SaveStruct(w, dir); err != nil {
				if errors.Cause(err) == ErrMissingTableOf {
					Log("msg", "SKIP function, missing TableOf info", "function", fun.Name())
					continue FunLoop
				}
				return err
			}
		}
		plsBlock, callFun := fun.PlsqlBlock()
		fmt.Fprintf(w, "\nconst %s = `", fun.getPlsqlConstName())
		io.WriteString(w, plsBlock)
		io.WriteString(w, "`\n\n")
		if b, err = format.Source([]byte(callFun)); err != nil {
			Log("msg", "saving function", "function", fun.Name(), "error", err)
			os.Stderr.WriteString("\n\n---------------------8<--------------------\n")
			os.Stderr.WriteString(callFun)
			os.Stderr.WriteString("\n--------------------->8--------------------\n\n")
			if !skipFormatting {
				return fmt.Errorf("error saving function %s: %s", fun.Name(), err)
			}
			b = []byte(callFun)
		}
		w.Write(b)
		//inpstruct := goName(fun.getStructName(false))
		inpstruct := fun.getStructName(false)
		inits = append(inits,
			fmt.Sprintf("\t"+`Functions["%s"] = func(cur *ora.Ses, input interface{}) (interface{}, error) {
        var inp %s
        switch x := input.(type) {
        case *%s: inp = *x
        case %s: inp = x
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
		if tn[0] == '+' { // REF CURSOR skip
			continue
		}
		if b, err = format.Source([]byte(text)); err != nil {
			return fmt.Errorf("error saving type %s: %s\n%s", tn, err, text)
		}
		w.Write(b)
	}

	io.WriteString(w, "\nfunc init() {\n")
	for _, text := range inits {
		io.WriteString(w, text)
		w.Write([]byte{'\n'})
	}
	if _, err = io.WriteString(w, "}\n"); err != nil {
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

var buffers = newBufPool(1 << 16)

func (f Function) SaveStruct(dst io.Writer, out bool) error {
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
		if arg.Direction&dirmap > 0 {
			args = append(args, arg)
		}
	}
	// return variable for function out structs
	if out && f.Returns != nil {
		args = append(args, *f.Returns)
	}

	if dirmap == uint8(DIR_IN) {
		checks = make([]string, 0, len(args)+1)
	}
	//structName = goName(f.getStructName(out))
	structName = f.getStructName(out)
	buf := buffers.Get()
	defer buffers.Put(buf)
	w := errWriter{Writer: buf, err: &err}

	fmt.Fprintf(w, `
	// %s %s
	type %s struct {
		XMLName xml.Name `+"`json:\"-\" xml:\"%s\"`"+`
		`, f.Name(), dirname, structName, strings.ToLower(structName[:1])+structName[1:],
	)

	//Log("msg","SaveStruct", "function", fmt.Sprintf("%#v", f) )
	for _, arg := range args {
		if arg.Flavor == FLAVOR_TABLE && arg.TableOf == nil {
			return errors.Wrapf(ErrMissingTableOf, "no table of data for %s.%s (%v)", f.Name(), arg, arg)
		}
		//aName = capitalize(goName(arg.Name))
		aName = capitalize(replHidden(arg.Name))
		got = arg.goType(f.types, arg.Flavor == FLAVOR_TABLE)
		lName := strings.ToLower(arg.Name)
		io.WriteString(w, "\t"+aName+" "+got+
			"\t`json:\""+lName+"\""+
			" xml:\""+lName+"\"`\n")
		if checks != nil {
			checks = genChecks(checks, arg, f.types, "s", false)
		}
	}
	io.WriteString(w, "}\n")

	if !out {
		fmt.Fprintf(w, `func (s *%s) FromJSON(data []byte) error {
			err := json.Unmarshal(data, &s)
			if DebugLevel > 0 {
				log.Printf("Unmarshal %%s: %%#v (%%v)", data, s, err)
			}
			return err
}`, structName)
	}
	if err != nil {
		return err
	}

	var b []byte
	if b, err = format.Source(buf.Bytes()); err != nil {
		return fmt.Errorf("error saving struct %q: %s\n%s", structName, err, buf.String())
	}
	dst.Write(b)

	if checks != nil {
		buf.Reset()
		fmt.Fprintf(w, "\n// Check checks input bounds for %s\nfunc (s %s) Check() error {\n", structName, structName)
		for _, line := range checks {
			fmt.Fprintf(w, line+"\n")
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

func genChecks(checks []string, arg Argument, types map[string]string, base string, parentIsTable bool) []string {
	//aName := capitalize(goName(arg.Name))
	aName := capitalize(replHidden(arg.Name))
	got := arg.goType(types, parentIsTable || arg.Flavor == FLAVOR_TABLE)
	var name string
	if aName == "" {
		name = base
	} else {
		name = base + "." + aName
	}
	switch arg.Flavor {
	case FLAVOR_SIMPLE:
		switch got {
		case "*string":
			checks = append(checks,
				fmt.Sprintf(`if %s != nil && len(*%s) > %d {
        return errors.New("%s is longer than accepted (%d)")
    }`,
					name, name, arg.Charlength,
					name, arg.Charlength))
		case "ora.String":
			checks = append(checks,
				fmt.Sprintf(`if !%s.IsNull && len(%s.Value) > %d {
        return errors.New("%s is longer than accepted (%d)")
    }`,
					name, name, arg.Charlength,
					name, arg.Charlength))
		case "NullString", "sql.NullString":
			checks = append(checks,
				fmt.Sprintf(`if %s.Valid && len(%s.String) > %d {
        return errors.New("%s is longer than accepted (%d)")
    }`,
					name, name, arg.Charlength,
					name, arg.Charlength))
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
		case "ora.Int32", "ora.Int64", "ora.Float32", "ora.Float64":
			if arg.Precision > 0 {
				cons := strings.Repeat("9", int(arg.Precision))
				checks = append(checks,
					fmt.Sprintf(`if !%s.IsNull && (%s.Value <= -%s || %s.Value > %s) {
        return errors.New("%s is out of bounds (-%s..%s)")
    }`,
						name, name, cons, name, cons,
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
		if parentIsTable || got[0] == '*' {
			checks = append(checks, "if "+name+" != nil {")
		}
		for _, sub := range arg.RecordOf {
			checks = genChecks(checks, sub, types, name, arg.Flavor == FLAVOR_TABLE) //parentIsTable || sub.Flavor == FLAVOR_TABLE)
		}
		if parentIsTable || got[0] == '*' {
			checks = append(checks, "}")
		}
	case FLAVOR_TABLE:
		if got[0] == '*' {
			checks = append(checks, "if "+name+" != nil {")
		}
		plus := strings.Join(
			genChecks(nil, *arg.TableOf, types, "v", true),
			"\n\t")
		if len(strings.TrimSpace(plus)) > 0 {
			checks = append(checks,
				fmt.Sprintf("\tfor _, v := range %s.%s {\n\t%s\n}",
					base, aName,
					plus))
		}
		if got[0] == '*' {
			checks = append(checks, "}")
		}
	default:
		Log("msg", "unknown flavor", "flavor", arg.Flavor)
		os.Exit(1)
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
func (arg *Argument) goType(typedefs map[string]string, isTable bool) (typName string) {
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
		case "CHAR", "VARCHAR2", "ROWID":
			if !isTable && arg.IsOutput() {
				return "*string"
			}
			return "ora.String" // NULL is the same as the empty string for Oracle
		case "NUMBER":
			if !isTable && arg.IsOutput() {
				return "*float64"
			}
			return "ora.Float64"
		case "INTEGER":
			if !isTable && arg.IsOutput() {
				return "*int64"
			}
			return "ora.Int64"
		case "PLS_INTEGER", "BINARY_INTEGER":
			if !isTable && arg.IsOutput() {
				return "*int32"
			}
			return "ora.Int32"
		case "BOOLEAN", "PL/SQL BOOLEAN":
			if !isTable && arg.IsOutput() {
				return "*bool"
			}
			return "sql.NullBool"
		case "DATE", "DATETIME", "TIME", "TIMESTAMP":
			if !isTable && arg.IsOutput() {
				return "*time.Time"
			}
			return "ora.Time"
		case "REF CURSOR":
			return "*ora.Rset"
		case "CLOB", "BLOB":
			return "ora.Lob"
		default:
			Log("msg", "unknown simple type", "type", arg.Type, "arg", arg)
			os.Exit(1)
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
	//typName = goName(capitalize(typName))
	typName = capitalize(typName)
	if td := typedefs[typName]; td != "" {
		return "*" + typName
	}

	buf := buffers.Get()
	defer buffers.Put(buf)

	if arg.Flavor == FLAVOR_TABLE {
		Log("msg", "TABLE", "arg", arg, "tableOf", arg.TableOf)
		targ := *arg.TableOf
		targ.Direction = DIR_IN
		tn := "[]" + targ.goType(typedefs, true)
		if arg.Type != "REF CURSOR" {
			if arg.IsOutput() && arg.TableOf.Flavor == FLAVOR_SIMPLE {
				return "*" + tn
			}
			return tn
		}
		cn := tn[2:] + "__cur"
		if cn[0] == '*' {
			cn = cn[1:]
		}
		if _, ok := typedefs[cn]; !ok {
			buf.WriteString("\ntype " + cn + " struct { *ora.Ses }\n")
			buf.WriteString("func New" + cn + "(cur *ora.Ses) (" + cn + ", error) {\n return " + cn + "{cur}, nil\n }")
			typedefs[cn] = buf.String()
			buf.Reset()
		}
		//typedefs["+"+arg.TableOf.Name] = "REF CURSOR"
		return cn
	}

	// FLAVOR_RECORD
	if arg.TypeName == "" {
		Log("msg", "arg has no TypeName", "arg", arg, "arg", fmt.Sprintf("%#v", arg))
		arg.TypeName = strings.ToLower(arg.Name)
	}
	buf.WriteString("\ntype " + typName + " struct {\n")
	for k, v := range arg.RecordOf {
		lName := strings.ToLower(k)
		//kName := capitalize(goName(k))
		kName := capitalize(replHidden(k))
		buf.WriteString("\t" + kName + " " +
			v.goType(typedefs, isTable) + "\t" +
			"`json:\"" + lName + ",omitempty\"" +
			" xml:\"" + lName + ",omitempty\"`\t\n")
	}
	buf.WriteString("}\n")

	typedefs[typName] = buf.String()
	return "*" + typName
}

func replHidden(text string) string {
	if text == "" {
		return text
	}
	if text[len(text)-1] == '#' {
		return text[:len(text)-1] + MarkHidden
	}
	return text
}

func goName(text string) string {
	text = replHidden(text)
	if text == "" {
		return text
	}
	var last rune
	return strings.Map(func(r rune) rune {
		if r == '_' {
			if last != '_' {
				last = '_'
				return -1
			}
			return '_'
		}
		if last == '_' {
			last = r
			return unicode.ToUpper(r)
		}
		if last == 0 {
			return r
		}
		return unicode.ToLower(r)
	},
		text,
	)
}

type errWriter struct {
	io.Writer
	err *error
}

func (ew errWriter) Write(p []byte) (int, error) {
	if ew.err != nil && *ew.err != nil {
		return 0, *ew.err
	}
	n, err := ew.Writer.Write(p)
	if err != nil {
		*ew.err = err
	}
	return n, err
}

type bufPool struct {
	sync.Pool
}

func newBufPool(size int) bufPool {
	return bufPool{sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, 1<<16)) }}}
}
func (bp bufPool) Get() *bytes.Buffer {
	return bp.Pool.Get().(*bytes.Buffer)
}
func (bp bufPool) Put(b *bytes.Buffer) {
	if b == nil {
		return
	}
	b.Reset()
	bp.Pool.Put(b)
}
