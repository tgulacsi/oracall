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
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/tgulacsi/go/orahlp"
)

// MaxTableSize is the maximum size of the array arguments
const MaxTableSize = 1000

//
// OracleArgument
//

var (
	stringTypes = make(map[string]struct{}, 16)
)

// SavePlsqlBlock saves the plsql block definition into writer
func (fun Function) PlsqlBlock(haveChecks bool) (plsql, callFun string) {
	decls, pre, call, post, convIn, convOut, err := fun.prepareCall()
	if err != nil {
		Log("msg", "error preparing", "function", fun, "error", err)
		os.Exit(1)
	}
	fn := strings.Replace(fun.name, ".", "__", -1)
	callBuf := buffers.Get()
	defer buffers.Put(callBuf)
	hasCursorOut := fun.HasCursorOut()
	if hasCursorOut {
		fmt.Fprintf(callBuf, `func (s *oracallServer) %s(input *%s, stream %s_%sServer) (err error) {
			output := new(%s)
			iterators := make([]iterator, 0, 1)
		`,
			CamelCase(fn),
			CamelCase(fun.getStructName(false, false)),
			CamelCase(fun.Package), CamelCase(fn),
			CamelCase(fun.getStructName(true, false)),
		)
	} else {
		fmt.Fprintf(callBuf, `func (s *oracallServer) %s(ctx context.Context, input *%s) (output *%s, err error) {
		output = new(%s)
		iterators := make([]iterator, 0, 1) // just temporary
		_ = iterators
    `,
			CamelCase(fn),
			CamelCase(fun.getStructName(false, false)),
			CamelCase(fun.getStructName(true, false)),
			CamelCase(fun.getStructName(true, false)),
		)
	}
	if haveChecks {
		callBuf.WriteString(
			`
	if err = input.Check(); err != nil {
        return
    }
	`)
	}
	for _, line := range convIn {
		io.WriteString(callBuf, line+"\n")
	}
	i := strings.Index(call, fun.Name())
	j := i + strings.Index(call[i:], ")") + 1
	//Log("msg","PlsqlBlock", "i", i, "j", j, "call", call)
	fmt.Fprintf(callBuf, "\nif true || DebugLevel > 0 { log.Printf(`calling %s\n\twith %%v`, params) }"+`
	ses, err := s.OraSesPool.Get()
	if err != nil {
		err = errors.Wrap(err, "Get")
		return
	}
	defer s.OraSesPool.Put(ses)
	qry := %s
	stmt, err := ses.Prep(qry)
	if err != nil {
		err =errors.Wrap(err, qry)
		return
	}
	if _, err =stmt.ExeP(params...); err != nil {
		err = errors.Wrapf(err, "%%s %%#v", qry, params)
		return
	}
	defer stmt.Close()
    `, call[i:j], fun.getPlsqlConstName())
	callBuf.WriteString("\nif true || DebugLevel > 0 { log.Printf(`result params: %v`, params) }\n")
	for _, line := range convOut {
		io.WriteString(callBuf, line+"\n")
	}
	if hasCursorOut {
		fmt.Fprintf(callBuf, `
		reseters := make([]func(), 0, len(iterators))
		iterators2 := make([]iterator, 0, len(iterators))
		IteratorsLoop:
		for len(iterators) > 0 {
			iterators2 = iterators2[:0]
			reseters = reseters[:0]
			for _, it := range iterators {
				if err = it.Iterate(); err != nil {
					if err != io.EOF {
						break IteratorsLoop
					}
					reseters = append(reseters, it.Reset)
					err = nil
					continue
				}
				iterators2 = append(iterators2, it)
			}
			if len(iterators) != len(iterators2) {
				iterators = append(iterators[:0], iterators2...)
			}
			if err = stream.Send(output); err != nil {
				return
			}
			// reset the all arrays
			for _, reset := range reseters {
				reset()
			}
		}
		if sendErr := stream.Send(output); sendErr != nil && err == nil {
			err = sendErr
		}
		`)
	}
	fmt.Fprintf(callBuf, `
        return
    }`)
	callFun = callBuf.String()

	plsBuf := callBuf
	plsBuf.Reset()
	if len(decls) > 0 {
		io.WriteString(plsBuf, "DECLARE\n")
		for _, line := range decls {
			fmt.Fprintf(plsBuf, "  %s\n", line)
		}
		plsBuf.Write([]byte{'\n'})
	}
	io.WriteString(plsBuf, "BEGIN\n")
	for _, line := range pre {
		fmt.Fprintf(plsBuf, "  %s\n", line)
	}
	fmt.Fprintf(plsBuf, "\n  %s;\n\n", call)
	for _, line := range post {
		fmt.Fprintf(plsBuf, "  %s\n", line)
	}
	io.WriteString(plsBuf, "\nEND;\n")
	plsql = plsBuf.String()

	plsql, callFun = demap(plsql, callFun)
	return
}

func demap(plsql, callFun string) (string, string) {
	type repl struct {
		ParamsArrLen int
	}
	paramsMap := make(map[string][]int, 16)

	var i int
	//old := plsql
	plsql, paramsArr := orahlp.MapToSlice(
		plsql,
		func(key string) interface{} {
			paramsMap[key] = append(paramsMap[key], i)
			i++
			return key
		})
	//Log("msg","MapToSlice", "old", old, "new", plsql, "params", paramsMap, "arr", paramsArr)

	opts := repl{
		ParamsArrLen: len(paramsArr),
	}
	var callBuf bytes.Buffer
	//fmt.Fprintln(os.Stderr, callFun)
	tpl, err := template.New("callFun").
		Funcs(
			map[string]interface{}{
				"paramsIdx": func(key string) int {
					if strings.HasSuffix(key, MarkHidden) {
						key = key[:len(key)-len(MarkHidden)] + "#"
					}
					arr := paramsMap[key]
					if len(arr) == 0 {
						Log("msg", "paramsIdx", "key", key, "val", arr, "map", paramsMap)
					}
					i = arr[0]
					if len(arr) > 1 {
						paramsMap[key] = arr[1:]
					}
					return i
				},
			}).
		Parse(callFun)
	if err != nil {
		fmt.Fprintf(os.Stderr, callFun)
		panic(err)
	}
	if err := tpl.Execute(&callBuf, opts); err != nil {
		panic(err)
	}
	return plsql, callBuf.String()
}

func (fun Function) prepareCall() (decls, pre []string, call string, post []string, convIn, convOut []string, err error) {
	tableTypes := make(map[string]string, 4)
	callArgs := make(map[string]string, 16)

	getTableType := func(absType string) string {
		typ, ok := tableTypes[absType]
		if !ok {
			typ = strings.Map(func(c rune) rune {
				switch c {
				case '(', ',':
					return '_'
				case ' ', ')':
					return -1
				default:
					return c
				}
			}, absType) + "_tab_typ"
			decls = append(decls, "TYPE "+typ+" IS TABLE OF "+absType+" INDEX BY BINARY_INTEGER;")
			tableTypes[absType] = typ
		}
		return typ
	}
	//fStructIn, fStructOut := fun.getStructName(false), fun.getStructName(true)
	var (
		vn, tmp, typ string
		ok           bool
	)
	decls = append(decls, "i1 PLS_INTEGER;", "i2 PLS_INTEGER;")
	convIn = append(convIn, "params := make([]interface{}, {{.ParamsArrLen}})", "var x, v interface{}\n _,_ = x,v")

	args := make([]Argument, 0, len(fun.Args)+1)
	for _, arg := range fun.Args {
		arg.Name = replHidden(arg.Name)
		args = append(args, arg)
	}
	if fun.Returns != nil {
		args = append(args, *fun.Returns)
	}
	addParam := func(paramName string) string {
		if paramName == "" {
			panic("empty param name")
		}
		return `params[{{paramsIdx "` + paramName + `"}}]`
	}
	for _, arg := range args {
		switch arg.Flavor {
		case FLAVOR_SIMPLE:
			name := (CamelCase(arg.Name))
			//name := capitalize(replHidden(arg.Name))
			convIn, convOut = arg.getConvSimple(convIn, convOut,
				name, addParam(arg.Name))

		case FLAVOR_RECORD:
			vn = getInnerVarName(fun.Name(), arg.Name)
			decls = append(decls, vn+" "+arg.TypeName+"; --E:"+arg.Name)
			callArgs[arg.Name] = vn
			aname := (CamelCase(arg.Name))
			//aname := capitalize(replHidden(arg.Name))
			if arg.IsOutput() {
				if arg.IsInput() {
					convIn = append(convIn, fmt.Sprintf(`
					output.%s = new(%s)
					if input.%s != nil { *output.%s = *input.%s }
					`, aname, CamelCase(arg.goType(false)[1:]),
						aname, aname, aname))
				} else {
					convOut = append(convOut, fmt.Sprintf(`
                    if output.%s == nil {
                        output.%s = new(%s)
                    }`, aname,
						aname, CamelCase(arg.goType(false)[1:])))
				}
			}
			for _, a := range arg.RecordOf {
				k, v := a.Name, a.Argument
				tmp = getParamName(fun.Name(), vn+"."+k)
				kName := (CamelCase(k))
				//kName := capitalize(replHidden(k))
				name := aname + "." + kName
				if arg.IsInput() {
					pre = append(pre, vn+"."+k+" := :"+tmp+";")
				}
				if arg.IsOutput() {
					post = append(post, ":"+tmp+" := "+vn+"."+k+";")
				}
				convIn, convOut = v.getConvRec(convIn, convOut,
					name, addParam(tmp),
					0, arg, k)
			}
		case FLAVOR_TABLE:
			if arg.Type == "REF CURSOR" {
				if arg.IsInput() {
					Log("msg", "cannot use IN cursor variables", "arg", arg)
					os.Exit(1)
				}
				name := (CamelCase(arg.Name))
				//name := capitalize(replHidden(arg.Name))
				convIn, convOut = arg.getConvSimpleTable(convIn, convOut,
					name, addParam(arg.Name), MaxTableSize)
			} else {
				switch arg.TableOf.Flavor {
				case FLAVOR_SIMPLE: // like simple, but for the arg.TableOf
					typ = getTableType(arg.TableOf.AbsType)
					setvar := ""
					if arg.IsInput() {
						setvar = " := :" + arg.Name
					}
					decls = append(decls, arg.Name+" "+typ+setvar+"; --A:"+arg.Name)

					vn = getInnerVarName(fun.Name(), arg.Name)
					callArgs[arg.Name] = vn
					decls = append(decls, vn+" "+arg.TypeName+"; --B:"+arg.Name)
					if arg.IsInput() {
						pre = append(pre,
							vn+".DELETE;",
							"i1 := "+arg.Name+".FIRST;",
							"WHILE i1 IS NOT NULL LOOP",
							"  "+vn+"(i1) := "+arg.Name+"(i1);",
							"  i1 := "+arg.Name+".NEXT(i1);",
							"END LOOP;")
					}
					if arg.IsOutput() {
						post = append(post,
							arg.Name+".DELETE;",
							"i1 := "+vn+".FIRST;",
							"WHILE i1 IS NOT NULL LOOP",
							"  "+arg.Name+"(i1) := "+vn+"(i1);",
							"  i1 := "+vn+".NEXT(i1);",
							"END LOOP;",
							":"+arg.Name+" := "+arg.Name+";")
					}
					name := (CamelCase(arg.Name))
					//name := capitalize(replHidden(arg.Name))
					convIn, convOut = arg.getConvSimpleTable(convIn, convOut,
						name, addParam(arg.Name), MaxTableSize)

				case FLAVOR_RECORD:
					vn = getInnerVarName(fun.Name(), arg.Name+"."+arg.TableOf.Name)
					callArgs[arg.Name] = vn
					decls = append(decls, vn+" "+arg.TypeName+"; --C:"+arg.Name)

					aname := (CamelCase(arg.Name))
					//aname := capitalize(replHidden(arg.Name))
					if arg.IsOutput() {
						convOut = append(convOut, fmt.Sprintf(`
                    if output.%s == nil { // %s
                        output.%s = make([]%s, 0, %d)
                    }`, aname, arg.TableOf.goType(true),
							aname, CamelCase(arg.TableOf.goType(true)), MaxTableSize))
					}
					if !arg.IsInput() {
						pre = append(pre, vn+".DELETE;")
					}

					// declarations go first
					for _, a := range arg.TableOf.RecordOf {
						k, v := a.Name, a.Argument
						typ = getTableType(v.AbsType)
						decls = append(decls, getParamName(fun.Name(), vn+"."+k)+" "+typ+"; --D:"+arg.Name)

						tmp = getParamName(fun.Name(), vn+"."+k)
						if arg.IsInput() {
							pre = append(pre, tmp+" := :"+tmp+";")
						} else {
							pre = append(pre, tmp+".DELETE;")
						}
					}

					// here comes the loops
					var idxvar string
					for _, a := range arg.TableOf.RecordOf {
						k, v := a.Name, a.Argument
						typ = getTableType(v.AbsType)

						tmp = getParamName(fun.Name(), vn+"."+k)

						if idxvar == "" {
							idxvar = getParamName(fun.Name(), vn+"."+k)
							if arg.IsInput() {
								pre = append(pre, "",
									"i1 := "+idxvar+".FIRST;",
									"WHILE i1 IS NOT NULL LOOP")
							}
							if arg.IsOutput() {
								post = append(post, "",
									"i1 := "+vn+".FIRST; i2 := 1;",
									"WHILE i1 IS NOT NULL LOOP")
							}
						}
						kName := (CamelCase(k))
						//kName := capitalize(replHidden(k))
						//name := aname + "." + kName

						convIn, convOut = v.getConvTableRec(
							convIn, convOut,
							[2]string{aname, kName},
							addParam(tmp),
							MaxTableSize,
							k, *arg.TableOf)

						if arg.IsInput() {
							pre = append(pre,
								"  "+vn+"(i1)."+k+" := "+tmp+"(i1);")
						}
						if arg.IsOutput() {
							post = append(post,
								"  "+tmp+"(i2) := "+vn+"(i1)."+k+";")
						}
					}
					if arg.IsInput() {
						pre = append(pre,
							"  i1 := "+idxvar+".NEXT(i1);",
							"END LOOP;")
					}
					if arg.IsOutput() {
						post = append(post,
							"  i1 := "+vn+".NEXT(i1); i2 := i2 + 1;",
							"END LOOP;")
						for _, a := range arg.TableOf.RecordOf {
							k := a.Name
							tmp = getParamName(fun.Name(), vn+"."+k)
							post = append(post, ":"+tmp+" := "+tmp+";")
						}
					}
				default:
					Log("msg", "Only table of simple or record types are allowed (no table of table!)", "function", fun.Name(), "arg", arg.Name)
					os.Exit(1)
				}
			}
		default:
			Log("msg", "unkown flavor", "flavor", arg.Flavor)
			os.Exit(1)
		}
	}

	callb := bytes.NewBuffer(nil)
	if fun.Returns != nil {
		callb.WriteString(":ret := ")
	}
	//Log("msg","prepareCall", "callArgs", callArgs)
	callb.WriteString(fun.Name() + "(")
	for i, arg := range fun.Args {
		if i > 0 {
			callb.WriteString(",\n\t\t")
		}
		if vn, ok = callArgs[arg.Name]; !ok {
			vn = ":" + arg.Name
		}
		fmt.Fprintf(callb, "%s=>%s", arg.Name, vn)
	}
	callb.WriteString(")")
	call = callb.String()
	return
}

func (arg Argument) getIsValidCheck(name string) string {
	got := arg.goType(false)
	if got[0] == '*' {
		return name + " != nil"
	}
	if strings.HasPrefix(got, "ora.") {
		return "!" + name + ".IsNull"
	}
	if strings.HasPrefix(got, "sql.Null") {
		return name + ".Valid"
	}
	return name + " != nil /*" + got + "*/"
}

func (arg Argument) getConvSimple(
	convIn, convOut []string,
	name, paramName string,
) ([]string, []string) {
	if arg.IsOutput() {
		got := arg.goType(false)
		if got[0] == '*' {
			convIn = append(convIn, fmt.Sprintf("output.%s = new(%s) // %s", name, got[1:], got))
			if arg.IsInput() {
				convIn = append(convIn, fmt.Sprintf(`if input.%s != nil { *output.%s = *input.%s }`, name, name, name))
			}
		} else if arg.IsInput() {
			convIn = append(convIn, fmt.Sprintf(`output.%s = input.%s`, name, name))
		}
		src := "output." + name
		in, varName := arg.ToOra(paramName, "&"+src)
		convIn = append(convIn, in)
		if varName != "" {
			convOut = append(convOut, arg.FromOra(src, paramName, varName))
		}
	} else {
		in, _ := arg.ToOra(paramName, "input."+name)
		convIn = append(convIn, in)
	}
	return convIn, convOut
}

func (arg Argument) getConvSimpleTable(
	convIn, convOut []string,
	name, paramName string,
	tableSize int,
) ([]string, []string) {
	if arg.IsOutput() {
		got := arg.goType(true)
		if arg.Type == "REF CURSOR" {
			return arg.getConvRefCursor(convIn, convOut, name, paramName, tableSize)
		}
		if got == "*[]string" {
			got = "[]string"
		}
		if got[0] == '*' {
			convIn = append(convIn, fmt.Sprintf(`
		if output.%s == nil { // %#v
			x := make(%s, 0, %d)
			output.%s = &x
		} else if cap((*output.%s)) < %d { // simpletable
			*output.%s = make(%s, 0, %d)
		} else {
			*(output.%s) = (*output.%s)[:0]
		}`, name, arg,
				strings.TrimLeft(got, "*"), tableSize,
				name,
				name, tableSize,
				name, strings.TrimLeft(got, "*"), tableSize,
				name, name))
			if arg.IsInput() {
				convIn = append(convIn, fmt.Sprintf(`*output.%s = append(*output.%s, input.%s)`, name, name, name))
			}
		} else {
			if arg.IsInput() {
				convIn = append(convIn, fmt.Sprintf("output.%s = input.%s", name, name))
			} else {
				got = CamelCase(got)
				convIn = append(convIn, fmt.Sprintf("output.%s = make(%s, 0, %d) // gcst3", name, got, tableSize))
			}
		}
		convIn = append(convIn, fmt.Sprintf(`%s = output.%s // gcst1`, paramName, name))
	} else {
		convIn = append(convIn, fmt.Sprintf("%s = input.%s // gcst2", paramName, name))
	}
	return convIn, convOut
}

func (arg Argument) getConvRefCursor(
	convIn, convOut []string,
	name, paramName string,
	tableSize int,
) ([]string, []string) {
	got := arg.goType(true)
	GoT := CamelCase(got)
	convIn = append(convIn, fmt.Sprintf(`output.%s = make([]%s, 0, %d)
				%s = new(ora.Rset) // gcrf1 %q`,
		name, GoT, tableSize,
		paramName, got))

	convOut = append(convOut, fmt.Sprintf(`
	{
		rset := %s.(*ora.Rset)
		if rset.IsOpen() {
		iterators = append(iterators, iterator{
			Reset: func() { output.%s = nil },
			Iterate: func() error {
		a := output.%s[:0]
		var err error
		for i := 0; i < %d; i++ {
			if !rset.Next() {
				if err = rset.Err; err == io.EOF {
					err = nil
				}
				break
			}
			a = append(a, %s)
		}
		output.%s = a
		return err
		},
		})
		}
	}`,
		paramName,
		name,
		name,
		tableSize,
		arg.getFromRset("rset.Row"),
		name,
	))
	return convIn, convOut
}

func (arg Argument) getFromRset(rsetRow string) string {
	buf := buffers.Get()
	defer buffers.Put(buf)

	GoT := CamelCase(arg.goType(true))
	if GoT[0] == '*' {
		GoT = "&" + GoT[1:]
	}
	fmt.Fprintf(buf, "%s{\n", GoT)
	for i, a := range arg.TableOf.RecordOf {
		fmt.Fprintf(buf, "\t%s: %s[%d].(%s),\n", CamelCase(a.Name), rsetRow, i, a.Argument.goType(true))
	}
	fmt.Fprintf(buf, "}")
	return buf.String()
}

func getOutConvTSwitch(name, pTyp string) string {
	parse := ""
	if strings.HasPrefix(pTyp, "int") {
		bits := "32"
		if len(pTyp) == 5 {
			bits = pTyp[3:5]
		}
		parse = "ParseInt(xi, 10, " + bits + ")"
	} else if strings.HasPrefix(pTyp, "float") {
		bits := pTyp[5:7]
		parse = "ParseFloat(xi, " + bits + ")"
	}
	if parse != "" {
		return fmt.Sprintf(`
			var y `+pTyp+`
			err = nil
			switch xi := x.(type) {
				case int: y = `+pTyp+`(xi)
				case int8: y = `+pTyp+`(xi)
				case int16: y = `+pTyp+`(xi)
				case int32: y = `+pTyp+`(xi)
				case int64: y = `+pTyp+`(xi)
				case float32: y = `+pTyp+`(xi)
				case float64: y = `+pTyp+`(xi)
				case string:
					//log.Printf("converting %%q to `+pTyp+`", xi)
					z, e := strconv.`+parse+`
					y, err = `+pTyp+`(z), e
				default:
					err = fmt.Errorf("out parameter %s is bad type: awaited %s, got %%T", x)
			}
			if err != nil {
				return
			}`, name, pTyp)
	}
	return fmt.Sprintf(`
				y, ok := x.(%s)
				if !ok {
					err = fmt.Errorf("out parameter %s is bad type: awaited %s, got %%T", x)
					return
				}`, pTyp, name, pTyp)
}

func (arg Argument) getConvRec(
	convIn, convOut []string,
	name, paramName string,
	tableSize uint,
	parentArg Argument,
	key string,
) ([]string, []string) {

	if arg.IsInput() {
		parts := strings.Split(name, ".")
		convIn = append(convIn,
			fmt.Sprintf("if input.%s != nil { %s = input.%s } // gcr1", parts[0], paramName, name))
	}
	if arg.IsOutput() {
		convIn = append(convIn,
			fmt.Sprintf("%s = output.%s // gcr2", paramName, name))
	}
	return convIn, convOut
}

func (arg Argument) getConvTableRec(
	convIn, convOut []string,
	name [2]string,
	paramName string,
	tableSize uint,
	key string,
	parent Argument,
) ([]string, []string) {
	lengthS := "0"
	absName := "x__" + name[0] + "__" + name[1]
	typ := arg.goType(true)
	if arg.IsInput() {
		if !arg.IsOutput() {
			lengthS = "len(input." + name[0] + ")"
		}
		convIn = append(convIn, fmt.Sprintf(`
			%s := make([]%s, %s, %d)
			for i,v := range input.%s { %s[i] = %s(v.%s); } // gctr1
			%s = %s`,
			absName,
			typ, lengthS, tableSize,
			name[0], absName, typ, name[1],
			paramName, absName))
	}
	if arg.IsOutput() {
		if !arg.IsInput() {
			convIn = append(convIn,
				fmt.Sprintf(`%s := make([]%s, %s, %d)
			%s = %s // gctr2`,
					absName, typ, lengthS, tableSize,
					paramName, absName))
		}
		convOut = append(convOut,
			fmt.Sprintf(`if m := len(%s)-cap(output.%s); m > 0{
			output.%s = append(output.%s, make([]%s, m)...)
		}
		output.%s = output.%s[:len(%s)]
		for i, v := range %s {
			if output.%s[i] != nil {
				%s
			}
		}`,
				absName, name[0],
				name[0], name[0], CamelCase(parent.goType(true)),
				name[0], name[0], absName,
				absName,
				name[0],
				arg.FromOra(
					fmt.Sprintf("output.%s[i].%s", name[0], name[1]),
					"v",
					"",
				)))
	}
	return convIn, convOut
}

var varNames = make(map[string]map[string]string, 4)

func getVarName(funName, varName, prefix string) string {
	m, ok := varNames[funName]
	if !ok {
		m = make(map[string]string, 16)
		varNames[funName] = m
	}
	x, ok := m[varName]
	if !ok {
		length := len(m)
		if i := strings.LastIndex(varName, "."); i > 0 && i < len(varName)-1 {
			x = getVarName(funName, varName[:i], prefix) + "#" + varName[i+1:]
		}
		if x == "" || len(x) > 30 {
			x = fmt.Sprintf("%s%03d", prefix, length+1)
		}
		m[varName] = x
	}
	return x
}

func getInnerVarName(funName, varName string) string {
	return getVarName(funName, varName, "v")
}

func getParamName(funName, paramName string) string {
	return getVarName(funName, paramName, "p")
}
