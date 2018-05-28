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
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/pkg/errors"
)

var ErrMissingTableOf = errors.New("missing TableOf info")
var ErrInvalidArgument = errors.New("invalid argument")

func SaveFunctions(dst io.Writer, functions []Function, pkg, pbImport string, saveStructs bool) error {
	var err error
	w := errWriter{Writer: dst, err: &err}

	if pkg != "" {
		if pbImport != "" {
			pbImport = `pb "` + pbImport + `"`
		}
		io.WriteString(w,
			"package "+pkg+`
import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"io/ioutil"
    "fmt"
	"strings"
	"database/sql"
	"database/sql/driver"
	"os"
	"strconv"
    "time"    // for datetimes
	"unsafe"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

    goracle "gopkg.in/goracle.v2" // Oracle
	"github.com/tgulacsi/oracall/custom"	// custom.Date
	oracall "github.com/tgulacsi/oracall/lib"	// ErrInvalidArgument
	`+pbImport+`
)

var DebugLevel = uint(0)

// against "unused import" error
var _ json.Marshaler
var _ = io.EOF
var _ context.Context
var _ custom.Date
var _ strconv.NumError
var _ time.Time
var _ strings.Reader
var _ xml.Name
var Log = func(keyvals ...interface{}) error { return nil } // logger.Log of github.com/go-kit/kit/log
var _ = errors.Wrap
var _ = fmt.Printf
var _ goracle.Lob
var _ unsafe.Pointer
var _ = spew.Sdump
var _ = os.Stdout
var _ driver.Rows
var _ = oracall.ErrInvalidArgument
var _ = ioutil.ReadAll

type iterator struct {
	Reset func()
	Iterate func() error
}

type oracallServer struct {
	db *sql.DB
}

func NewServer(db *sql.DB) *oracallServer {
	return &oracallServer{db: db}
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
		var checkName string
		for _, dir := range []bool{false, true} {
			if err = fun.SaveStruct(structW, dir); err != nil {
				if SkipMissingTableOf && errors.Cause(err) == ErrMissingTableOf {
					Log("msg", "SKIP function, missing TableOf info", "function", fun.Name(), "error", err)
					continue FunLoop
				}
				return err
			}
		}
		if checkName, err = fun.GenChecks(w); err != nil {
			return err
		}
		plsBlock, callFun := fun.PlsqlBlock(checkName)
		fmt.Fprintf(w, "\nconst %s = `", fun.getPlsqlConstName())
		io.WriteString(w, plsBlock)
		io.WriteString(w, "`\n\n")
		if b, err = format.Source([]byte(callFun)); err != nil {
			Log("msg", "saving function", "function", fun.Name(), "error", err)
			os.Stderr.WriteString("\n\n---------------------8<--------------------\n")
			os.Stderr.WriteString(callFun)
			os.Stderr.WriteString("\n--------------------->8--------------------\n\n")
			return fmt.Errorf("error saving function %s: %s", fun.Name(), err)
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
	_, err = io.WriteString(w, "}\n")
	return err
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

func (f Function) SaveStruct(dst io.Writer, out bool) error {
	dirmap, dirname := DIR_IN, "input"
	if out {
		dirmap, dirname = DIR_OUT, "output"
	}
	var (
		err                    error
		aName, structName, got string
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
		if got == "" || got == "*" {
			got = got + mkRecTypName(arg.Name)
		}
		lName := strings.ToLower(arg.Name)
		io.WriteString(w, "\t"+aName+" "+got+
			"\t`json:\""+lName+"\""+
			" xml:\""+lName+"\"`\n")
	}
	io.WriteString(w, "}\n")

	if !out {
		fmt.Fprintf(w, `func (s *%s) FromJSON(data []byte) error {
			err := json.Unmarshal(data, &s)
			if DebugLevel > 0 {
				Log("msg", "unmarshal", "data", data, "into", s, "error", err)
			}
			return err
}`, structName)
	}
	if err != nil {
		return err
	}

	var b []byte
	if b, err = format.Source(buf.Bytes()); err != nil {
		return errors.Wrapf(err, "save struct %q (%s)", structName, buf.String())
	}
	_, err = dst.Write(b)

	return err
}

func (f Function) GenChecks(w io.Writer) (string, error) {
	args := make([]Argument, 0, len(f.Args))
	for _, arg := range f.Args {
		if arg.IsInput() {
			args = append(args, arg)
		}
	}
	checks := make([]string, 0, len(args)+1)
	for _, arg := range args {
		checks = genChecks(checks, arg, "s", false)
	}
	if len(checks) == 0 {
		return "", nil
	}
	structName := CamelCase(strings.SplitN(f.getStructName(false, true), "__", 2)[1])
	buf := buffers.Get()
	defer buffers.Put(buf)
	nm := "Check" + structName
	fmt.Fprintf(buf, `
// %s checks input bounds for pb.%s
func %s(s *pb.%s) error {
	`,
		nm, structName,
		nm, structName,
	)
	for _, line := range checks {
		fmt.Fprintf(buf, line+"\n")
	}
	if _, err := io.WriteString(buf, "\n\treturn nil\n}\n"); err != nil {
		return "", err
	}
	b, err := format.Source(buf.Bytes())
	if err != nil {
		return nm, errors.Wrapf(err, "write check of %s (%s)", structName, buf.String())
	}
	_, err = w.Write(b)
	return nm, err
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
		case "string":
			checks = append(checks,
				fmt.Sprintf(`if len(%s) > %d {
        return errors.Wrap(oracall.ErrInvalidArgument, "%s is longer than accepted (%d)")
    }`,
					name, arg.Charlength, name, arg.Charlength))
		case "*string":
			checks = append(checks,
				fmt.Sprintf(`if %s != nil && len(*%s) > %d {
        return errors.Wrap(oracall.ErrInvalidArgument, "%s is longer than accepted (%d)")
    }`,
					name, name, arg.Charlength,
					name, arg.Charlength))
		case "sql.NullString":
			checks = append(checks,
				fmt.Sprintf(`if %s.Valid && len(%s.String) > %d {
        return errors.Wrap(oracall.ErrInvalidArgument, "%s is longer than accepted (%d)")
    }`,
					name, name, arg.Charlength,
					name, arg.Charlength))
		case "NullString":
			checks = append(checks,
				fmt.Sprintf(`if %s.Valid && len(%s.String) > %d {
        return errors.Wrap(oracall.ErrInvalidArgument, "%s is longer than accepted (%d)")
    }`,
					name, name, arg.Charlength,
					name, arg.Charlength))
		case "goracle.Number":
			checks = append(checks,
				fmt.Sprintf(`if err := oracall.ParseDigits(%s, %d, %d); err != nil { return errors.Wrap(oracall.ErrInvalidArgument, %s) }`, name, arg.Precision, arg.Scale, name))

		case "int32": // no check is needed
		case "int64", "float64":
			if arg.Precision > 0 {
				cons := strings.Repeat("9", int(arg.Precision))
				checks = append(checks,
					fmt.Sprintf(`if (%s <= -%s || %s > %s) {
        return errors.Wrap(oracall.ErrInvalidArgument, "%s is out of bounds (-%s..%s)")
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
        return errors.Wrap(oracall.ErrInvalidArgument, "%s is out of bounds (-%s..%s)")
    }`,
						name, name, vn, cons, name, vn, cons,
						name, cons, cons))
			}

		case "custom.Date":
			checks = append(checks,
				fmt.Sprintf(`if _, err := custom.Date(%s).Get(); err != nil {
				  return errors.Wrapf(oracall.ErrInvalidArgument, "%s has bad format: " + err.Error())
			  }`,
					name, name))
		default:
			checks = append(checks, fmt.Sprintf("// No check for %q (%q)", arg.Name, got))
		}
	case FLAVOR_RECORD:
		if parentIsTable || got[0] == '*' {
			checks = append(checks, "if "+name+" != nil {")
		}
		for _, sub := range arg.RecordOf {
			checks = genChecks(checks, sub.Argument, name, arg.Flavor == FLAVOR_TABLE) //parentIsTable || sub.Flavor == FLAVOR_TABLE)
		}
		if parentIsTable || got[0] == '*' {
			checks = append(checks, "}")
		}
	case FLAVOR_TABLE:
		if got[0] == '*' {
			checks = append(checks, fmt.Sprintf("if %s != nil {  // genChecks[T] %q", name, got))
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
	if arg.mu == nil {
		arg.mu = new(sync.Mutex)
	}
	arg.mu.Lock()
	defer arg.mu.Unlock()
	// cached?
	if arg.goTypeName != "" {
		if strings.Index(arg.goTypeName, "__") > 0 {
			return "*" + arg.goTypeName
		}
		return arg.goTypeName
	}
	defer func() {
		// cache it
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
		case "RAW":
			return "string"
		case "NUMBER":
			return "goracle.Number"
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
			return "*sql.Rows"
		case "CLOB", "BLOB":
			return "goracle.Lob"
		case "BFILE":
			return "ora.Bfile"
		default:
			Log("error", fmt.Sprintf("%+v", errors.New("unknown simple type")), "type", arg.Type, "arg", fmt.Sprintf("%#v", arg))
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
		//Log("msg", "TABLE", "arg", arg, "tableOf", arg.TableOf)
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
	if false && arg.TypeName == "" {
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
		if last == 0 || last == '_' || last == '.' || '0' <= last && last <= '9' {
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

func newBufPool(size int) *bufPool {
	return &bufPool{sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, 1<<16)) }}}
}
func (bp *bufPool) Get() *bytes.Buffer {
	return bp.Pool.Get().(*bytes.Buffer)
}
func (bp *bufPool) Put(b *bytes.Buffer) {
	if b == nil {
		return
	}
	b.Reset()
	bp.Pool.Put(b)
}

var rIdentifier = regexp.MustCompile(`:([0-9a-zA-Z][a-zA-Z0-9_]*)`)

func ReplOraPh(s string, params []interface{}) string {
	var i int
	return rIdentifier.ReplaceAllStringFunc(
		s,
		func(_ string) string {
			i++
			return fmt.Sprintf("%v", params[i-1])
		},
	)
}

/*
func replOraPhMeta(text, sliceName string) string {
	var i int
	return rIdentifier.ReplaceAllStringFunc(
		text,
		func(_ string) string {
			i++
			return fmt.Sprintf("`"+`+fmt.Sprintf("%%v", %s[%d])+`+"`", sliceName, i-1)
		},
	)
}
*/

// vim: se noet fileencoding=utf-8:
