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

package oracall

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"unicode"

	"github.com/pkg/errors"
)

var ErrMissingTableOf = errors.New("missing TableOf info")

func SaveFunctions(dst io.Writer, functions []Function, pkg string, skipFormatting, saveStructs bool) error {
	var err error
	w := errWriter{Writer: dst, err: &err}

	if pkg != "" {
		fmt.Fprintf(w,
			"package "+pkg+`
import (
	"encoding/xml"
    "log"
    "fmt"
	"strings"
	"strconv"
    "time"    // for datetimes

	"golang.org/x/net/context"

	"github.com/pkg/errors"
    "gopkg.in/rana/ora.v3"    // Oracle
	"github.com/tgulacsi/oracall/custom"	// custom.Date
)

var DebugLevel = uint(0)

// against "unused import" error
var _ ora.Ses
var _ context.Context
var _ custom.Date
var _ strconv.NumError
var _ time.Time
var _ strings.Reader
var _ xml.Name
var _ log.Logger
var _ = errors.Wrap
var _ = fmt.Printf

type OraSesPool interface {
	Get() (*ora.Ses, error)
	Put(*ora.Ses)
}

type oracallServer struct {
	OraSesPool
}

func NewServer(p OraSesPool) *oracallServer {
	return &oracallServer{OraSesPool: p}
}

`)
	}
	types := make(map[string]string, 16)
	inits := make([]string, 0, len(functions))
	var b []byte

FunLoop:
	for _, fun := range functions {
		structW := io.Writer(w)
		if !saveStructs {
			structW = ioutil.Discard
		}
		for _, dir := range []bool{false, true} {
			if err := fun.SaveStruct(structW, dir, true); err != nil {
				if errors.Cause(err) == ErrMissingTableOf {
					Log("msg", "SKIP function, missing TableOf info", "function", fun.Name(), "error", err)
					continue FunLoop
				}
				return err
			}
		}
		plsBlock, callFun := fun.PlsqlBlock(saveStructs)
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

func (f Function) getStructName(out, withPackage bool) string {
	dirname := "input"
	if out {
		dirname = "output"
	}
	if !withPackage {
		return f.name + "__" + dirname
	}
	return capitalize(f.Package + "__" + f.name + "__" + dirname)
}

var buffers = newBufPool(1 << 16)

func (f Function) SaveStruct(dst io.Writer, out, generateChecks bool) error {
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
	structName = CamelCase(f.getStructName(out, true))
	//structName = f.getStructName(out)
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
		got = arg.goType(arg.Flavor == FLAVOR_TABLE)
		lName := strings.ToLower(arg.Name)
		io.WriteString(w, "\t"+aName+" "+got+
			"\t`json:\""+lName+"\""+
			" xml:\""+lName+"\"`\n")
		if generateChecks && checks != nil {
			checks = genChecks(checks, arg, "s", false)
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

func genChecks(checks []string, arg Argument, base string, parentIsTable bool) []string {
	aName := (CamelCase(arg.Name))
	//aName := capitalize(replHidden(arg.Name))
	got := arg.goType(parentIsTable || arg.Flavor == FLAVOR_TABLE)
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
			checks = genChecks(checks, sub, name, arg.Flavor == FLAVOR_TABLE) //parentIsTable || sub.Flavor == FLAVOR_TABLE)
		}
		if parentIsTable || got[0] == '*' {
			checks = append(checks, "}")
		}
	case FLAVOR_TABLE:
		if got[0] == '*' {
			checks = append(checks, "if "+name+" != nil {")
		}
		plus := strings.Join(
			genChecks(nil, *arg.TableOf, "v", true),
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

// returns a go type for the argument's type
func (arg *Argument) goType(isTable bool) (typName string) {
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
				//return "*string"
				return "string"
			}
			return "string" // NULL is the same as the empty string for Oracle
		case "NUMBER":
			return "float64"
			if !isTable && arg.IsOutput() {
				return "*float64"
			}
			return "float64"
		case "INTEGER":
			if !isTable && arg.IsOutput() {
				return "*int64"
			}
			return "int64"
		case "PLS_INTEGER", "BINARY_INTEGER":
			if !isTable && arg.IsOutput() {
				//return "*int32"
				return "int32"
			}
			return "int32"
		case "BOOLEAN", "PL/SQL BOOLEAN":
			if !isTable && arg.IsOutput() {
				return "*bool"
			}
			return "bool"
		case "DATE", "DATETIME", "TIME", "TIMESTAMP":
			return "custom.Date"
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

	if arg.Flavor == FLAVOR_TABLE {
		Log("msg", "TABLE", "arg", arg, "tableOf", arg.TableOf)
		targ := *arg.TableOf
		targ.Direction = DIR_IN
		tn := "[]" + targ.goType(true)
		if arg.Type != "REF CURSOR" {
			if arg.IsOutput() && arg.TableOf.Flavor == FLAVOR_SIMPLE {
				return "*" + tn
			}
			return tn
		}
		cn := tn[2:]
		if cn[0] == '*' {
			cn = cn[1:]
		}
		return cn
	}

	// FLAVOR_RECORD
	if arg.TypeName == "" {
		Log("msg", "arg has no TypeName", "arg", arg, "arg", fmt.Sprintf("%#v", arg))
		arg.TypeName = strings.ToLower(arg.Name)
	}
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

var digitUnder = strings.NewReplacer(
	"_0", "__0",
	"_1", "__1",
	"_2", "__2",
	"_3", "__3",
	"_4", "__4",
	"_5", "__5",
	"_6", "__6",
	"_7", "__7",
	"_8", "__8",
	"_9", "__9",
)

func CamelCase(text string) string {
	text = replHidden(text)
	if text == "" {
		return text
	}
	var prefix string
	if text[0] == '*' {
		prefix, text = "*", text[1:]
	}

	text = digitUnder.Replace(text)
	var last rune
	return prefix + strings.Map(func(r rune) rune {
		defer func() { last = r }()
		if r == '_' {
			if last != '_' {
				return -1
			}
			return '_'
		}
		if last == 0 || last == '_' || '0' <= last && last <= '9' {
			return unicode.ToUpper(r)
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
