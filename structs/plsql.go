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
	"io"
	"os"
	"strings"
)

// MaxTableSize is the maximum size of the array arguments
const MaxTableSize = 1000

//
// OracleArgument
//

var stringTypes = make(map[string]struct{}, 16)

func oracleVarTypeName(typ string) string {
	switch typ {
	case "BINARY_INTEGER", "PLS_INTEGER":
		return "ora.Int32"
	case "BINARY_FLOAT":
		return "ora.Float32"
	case "BINARY_DOUBLE", "NUMBER":
		return "ora.Float64"
	case "STRING", "VARCHAR2", "ROWID", "CHAR":
		return "ora.String"
	case "INTEGER":
		return "ora.Int64"
	case "REF CURSOR":
		return "oracle.CursorVarType"
	case "DATE", "TIMESTAMP":
		return "ora.Time"
	case "BLOB", "CLOB":
		return "ora.Lob"
	case "BFILE":
		return "ora.Bfile"
	case "BOOLEAN", "PL/SQL BOOLEAN":
		return "ora.Int8"
	default:
		Log.Crit("oracleVarTypeName: unknown variable type", "type", typ)
		os.Exit(1)
	}
	return ""
}

// SavePlsqlBlock saves the plsql block definition into writer
func (fun Function) PlsqlBlock() (plsql, callFun string) {
	decls, pre, call, post, convIn, convOut, err := fun.prepareCall()
	if err != nil {
		Log.Crit("error preparing", "function", fun, "error", err)
		os.Exit(1)
	}
	fn := strings.Replace(fun.Name(), ".", "__", -1)
	callBuf := buffers.Get()
	defer buffers.Put(callBuf)
	fmt.Fprintf(callBuf, `func Call_%s(ses *ora.Ses, input %s) (output %s, err error) {
    if err = input.Check(); err != nil {
        return
    }
    params := make([]interface{}, %d)
    `, fn, fun.getStructName(false), fun.getStructName(true), len(fun.Args)+1)
	for _, line := range convIn {
		io.WriteString(callBuf, line+"\n")
	}
	i := strings.Index(call, fun.Name())
	j := i + strings.Index(call[i:], ")") + 1
	Log.Debug("PlsqlBlock", "i", i, "j", j, "call", call)
	fmt.Fprintf(callBuf, "\nif true || DebugLevel > 0 { log.Printf(`calling %s\n\twith %%s`, params) }"+`
    if _, err = ses.PrepAndExe(%s, params...); err != nil { return }
    `, call[i:j], fun.getPlsqlConstName())
	fmt.Fprintf(callBuf, "\nif true || DebugLevel > 0 { log.Printf(`result params: %%s`, params) }\n")
	for _, line := range convOut {
		io.WriteString(callBuf, line+"\n")
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
	return
}

func (fun Function) prepareCall() (decls, pre []string, call string, post []string, convIn, convOut []string, err error) {
	if fun.types == nil {
		Log.Info("nil types", "function", fun)
		fun.types = make(map[string]string, 4)
	}
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
	convIn = append(convIn, "var x, v interface{}\n _,_ = x,v")

	var args []Argument
	if fun.Returns != nil {
		args = make([]Argument, 0, len(fun.Args)+1)
		for _, arg := range fun.Args {
			args = append(args, arg)
		}
		args = append(args, *fun.Returns)
	} else {
		args = fun.Args
	}
	for _, arg := range args {
		switch arg.Flavor {
		case FLAVOR_SIMPLE:
			name := capitalize(goName(arg.Name))
			convIn, convOut = arg.getConvSimple(convIn, convOut, fun.types, name, arg.Name, 0)

		case FLAVOR_RECORD:
			vn = getInnerVarName(fun.Name(), arg.Name)
			decls = append(decls, vn+" "+arg.TypeName+";")
			callArgs[arg.Name] = vn
			aname := capitalize(goName(arg.Name))
			if arg.IsOutput() {
				convOut = append(convOut, fmt.Sprintf(`
                    if output.%s == nil {
                        output.%s = new(%s)
                    }`, aname, aname, arg.goType(fun.types)[1:]))
			}
			for k, v := range arg.RecordOf {
				tmp = getParamName(fun.Name(), vn+"."+k)
				name := aname + "." + capitalize(goName(k))
				if arg.IsInput() {
					pre = append(pre, vn+"."+k+" := :"+tmp+";")
				}
				if arg.IsOutput() {
					post = append(post, ":"+tmp+" := "+vn+"."+k+";")
				}
				convIn, convOut = v.getConvRec(convIn, convOut, fun.types, name, tmp,
					0, &arg, k)
			}
		case FLAVOR_TABLE:
			if arg.Type == "REF CURSOR" {
				if arg.IsInput() {
					Log.Crit("cannot use IN cursor variables", "arg", arg)
					os.Exit(1)
				}
				name := capitalize(goName(arg.Name))
				convIn, convOut = arg.getConvSimple(convIn, convOut, fun.types, name, arg.Name, 0)
			} else {
				switch arg.TableOf.Flavor {
				case FLAVOR_SIMPLE: // like simple, but for the arg.TableOf
					typ = getTableType(arg.TableOf.AbsType)
					setvar := ""
					if arg.IsInput() {
						setvar = " := :" + arg.Name
					}
					decls = append(decls, arg.Name+" "+typ+setvar+";")

					vn = getInnerVarName(fun.Name(), arg.Name)
					callArgs[arg.Name] = vn
					decls = append(decls, vn+" "+arg.TypeName+";")
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
					name := capitalize(goName(arg.Name))
					convIn, convOut = arg.TableOf.getConvSimple(convIn, convOut,
						fun.types, name, arg.Name, MaxTableSize)

				case FLAVOR_RECORD:
					vn = getInnerVarName(fun.Name(), arg.Name+"."+arg.TableOf.Name)
					callArgs[arg.Name] = vn
					decls = append(decls, vn+" "+arg.TypeName+";")

					aname := capitalize(goName(arg.Name))
					if arg.IsOutput() {
						convOut = append(convOut, fmt.Sprintf(`
                    if output.%s == nil {
                        output.%s = make([]%s, 0, %d)
                    }`, aname, aname, arg.TableOf.goType(fun.types), MaxTableSize))
					}
					/* // PLS-00110: a(z) 'P038.DELETE' hozzárendelt változó ilyen környezetben nem használható
					if arg.IsOutput() {
						// DELETE out tables
						for k := range arg.TableOf.RecordOf {
							post = append(post,
								":"+getParamName(fun.Name(), vn+"."+k)+".DELETE;")
						}
					}
					*/
					if !arg.IsInput() {
						pre = append(pre, vn+".DELETE;")
					}

					// declarations go first
					for k, v := range arg.TableOf.RecordOf {
						typ = getTableType(v.AbsType)
						decls = append(decls, getParamName(fun.Name(), vn+"."+k)+" "+typ+";")

						tmp = getParamName(fun.Name(), vn+"."+k)
						if arg.IsInput() {
							pre = append(pre, tmp+" := :"+tmp+";")
						} else {
							pre = append(pre, tmp+".DELETE;")
						}
					}

					// here comes the loops
					var idxvar string
					for k, v := range arg.TableOf.RecordOf {
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
						//name := aname + "." + capitalize(goName(k))

						convIn, convOut = v.getConvRec(
							convIn, convOut, fun.types, aname, tmp, MaxTableSize,
							arg.TableOf, k)

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
						for k := range arg.TableOf.RecordOf {
							tmp = getParamName(fun.Name(), vn+"."+k)
							post = append(post, ":"+tmp+" := "+tmp+";")
						}
					}
				default:
					Log.Crit("Only table of simple or record types are allowed (no table of table!)", "function", fun.Name(), "arg", arg.Name)
					os.Exit(1)
				}
			}
		default:
			Log.Crit("unkown flavor", "flavor", arg.Flavor)
			os.Exit(1)
		}
	}
	callb := bytes.NewBuffer(nil)
	if fun.Returns != nil {
		callb.WriteString(":ret := ")
	}
	Log.Debug("prepareCall", "callArgs", callArgs)
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

func (arg Argument) getConvSimple(
	convIn, convOut []string,
	types map[string]string,
	name, paramName string,
	tableSize uint,
) ([]string, []string) {

	got := arg.goType(types)
	preconcept, preconcept2 := "", ""
	if strings.Count(name, ".") >= 1 {
		preconcept = "input." + name[:strings.LastIndex(name, ".")] + " != nil &&"
		preconcept2 = "if " + preconcept[:len(preconcept)-3] + " {"
	}
	nilS, deRef := `nil`, "*"
	if got == "string" {
		nilS, deRef = `""`, ""
	}
	if arg.IsOutput() {
		if arg.IsInput() {
			convIn = append(convIn, fmt.Sprintf(`output.%s = input.%s`, name, name))
		}
		convIn = append(convIn, fmt.Sprintf(`v = &output.%s`, name))
		pTyp := got
		if pTyp[0] == '*' {
			pTyp = pTyp[1:]
		}
		var outConv string
		if tableSize == 0 {
			if strings.HasSuffix(got, "__cur") {
				outConv = fmt.Sprintf(`output.%s = %s{y}`, name, got)
				if arg.Type == "REF CURSOR" {
					pTyp = "*ora.Ses"
				}
			} else if got == "string" {
				outConv = fmt.Sprintf(`output.%s = y`, name)
			} else if got[0] != '*' {
				outConv = fmt.Sprintf(`output.%s = &y`, name)
			} else {
				outConv = fmt.Sprintf(`z := %s(y)
            output.%s = &z`, pTyp, name)
			}
			convOut = append(convOut,
				fmt.Sprintf(`if params["%s"] != nil {
                v = params["%s"].(*oracle.Variable)
                if err = v.GetValueInto(&x, 0); err != nil {
                    err = fmt.Errorf("error getting value of %s: %%s", err)
                    return
                } else if x != `+nilS+` {
					%s
                    %s
                }}
                `, paramName, paramName, name, getOutConvTSwitch(name, pTyp), outConv))
		} else {
			if got == "string" {
				outConv = fmt.Sprintf(`output.%s[i] = y`, name)
			} else if got[0] != '*' {
				outConv = fmt.Sprintf(`output.%s[i] = &y`, name)
			} else {
				outConv = fmt.Sprintf(`z := %s(y)
            output.%s[i] = &z`, pTyp, name)
			}
			mk := ""
			mk = fmt.Sprintf(`output.%s = output.%s[:cap(output.%s)]
                if cap(output.%s) < v.Len() {
                    output.%s = append(output.%s, make([]%s, v.Len()-cap(output.%s))...)
                }`, name, name, name, name, name, name, got, name)
			convOut = append(convOut,
				fmt.Sprintf(`if params["%s"] != nil {
                v = params["%s"].(*oracle.Variable)
                %s
                for i := 0; i < v.Len(); i++ {
                    if err = v.GetValueInto(&x, uint(i)); err != nil {
                        err = fmt.Errorf("error getting %%d. value into %s: %%s", i, err)
                        return
                    } else if x != `+nilS+` {
						%s
                        %s
                    }
                }
            }
            `, paramName, paramName,
					mk,
					name,
					getOutConvTSwitch(name, pTyp), outConv))
		}
		if arg.IsInput() {
			if tableSize == 0 {
				if preconcept2 != "" {
					convIn = append(convIn, preconcept2)
				}
				convIn = append(convIn,
					fmt.Sprintf(`if input.%s != `+nilS+` {
                            if err = v.SetValue(0, `+deRef+`input.%s); err != nil {
                                err = fmt.Errorf("error setting value %%v from %s: %%s", v, err)
                                return
                            }
                        }`,
						name, name, name))
				if preconcept2 != "" {
					convIn = append(convIn, "}")
				}
			} else {
				if preconcept2 != "" {
					convIn = append(convIn, preconcept2)
				}
				convIn = append(convIn,
					fmt.Sprintf(`for i, x := range input.%s {
                            if err = v.SetValue(uint(i), x); err != nil {
                                err = fmt.Errorf("error setting value %%v[%%d] from %s: %%s", v, i, err)
                                return
                            }
                        }`, name, name))
				if preconcept2 != "" {
					convIn = append(convIn, "}")
				}
			}
		}
	} else { // just and only input
		if tableSize == 0 {
			if preconcept2 != "" {
				convIn = append(convIn, preconcept2)
			}
			convIn = append(convIn,
				fmt.Sprintf(`v = input.%s`, name))
			if preconcept2 != "" {
				convIn = append(convIn, "} else { v = nil }")
			}
		} else {
			if preconcept2 != "" {
				convIn = append(convIn, preconcept2)
			}
			subGoType := got
			if subGoType[0] == '*' {
				subGoType = subGoType[1:]
			}
			convIn = append(convIn,
				fmt.Sprintf(`v = input.%s`, name))
			if preconcept2 != "" {
				convIn = append(convIn, "}")
			}
		}
	}
	convIn = append(convIn,
		fmt.Sprintf("params[\"%s\"] = v\n", paramName))
	return convIn, convOut
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

func (arg Argument) getConvRec(convIn, convOut []string,
	types map[string]string, name, paramName string, tableSize uint,
	parentArg *Argument, key string) ([]string, []string) {

	got := arg.goType(types)
	preconcept, preconcept2 := "", ""
	if strings.Count(name, ".") >= 1 && parentArg != nil {
		preconcept = "input." + name[:strings.LastIndex(name, ".")] + " != nil &&"
		preconcept2 = "if " + preconcept[:len(preconcept)-3] + " {"
	}
	nilS, deRef := `nil`, "*"
	if arg.goType(types) == "string" {
		nilS, deRef = `""`, ""
	}
	if arg.IsOutput() {
		convIn = append(convIn,
			fmt.Sprintf(`if v, err = ses.NewVariable(%d, %s, %d); err != nil {
                    err = fmt.Errorf("error creating variable for type %s: %%s", err)
                    return
                }`,
				tableSize, oracleVarTypeName(arg.Type), arg.Charlength, arg.Type))
		pTyp := got
		if pTyp[0] == '*' {
			pTyp = pTyp[1:]
		}
		var outConv string
		if tableSize == 0 {
			if got == "string" {
				outConv = fmt.Sprintf(`output.%s = y`, name)
			} else if got[0] != '*' {
				outConv = fmt.Sprintf(`output.%s = &y`, name)
			} else {
				outConv = fmt.Sprintf(`z := %s(y)
            output.%s = &z`, pTyp, name)
			}
			convOut = append(convOut,
				fmt.Sprintf(`if params["%s"] != nil {
                v = params["%s"].(*oracle.Variable)
                if err = v.GetValueInto(&x, 0); err != nil {
                    err = fmt.Errorf("error getting value of %s: %%s", err)
                    return
                } else if x != `+nilS+` {
					%s
                    %s
                }}
                `, paramName, paramName, name, getOutConvTSwitch(name, pTyp), outConv))
		} else {
			if got == "string" {
				outConv = fmt.Sprintf(`output.%s[i].%s = y`, name, capitalize(key))
			} else if got[0] != '*' {
				outConv = fmt.Sprintf(`output.%s[i].%s = &y`, name, capitalize(key))
			} else {
				outConv = fmt.Sprintf(`z := %s(y)
            output.%s[i].%s = &z`, pTyp, name, capitalize(key))
			}
			convOut = append(convOut,
				fmt.Sprintf(`if params["%s"] != nil {
                v = params["%s"].(*oracle.Variable)
                for i := 0; i < v.Len(); i++ {
                    if err = v.GetValueInto(&x, uint(i)); err != nil {
                        err = fmt.Errorf("error getting %%d. value into %s%s: %%s", i, err)
                        return
                    } else if x != `+nilS+` {
						%s
                        %s
                    }
                }
            }
            `, paramName, paramName,
					name, key,
					getOutConvTSwitch(name, pTyp), outConv))
		}
		if arg.IsInput() {
			if tableSize == 0 {
				if preconcept2 != "" {
					convIn = append(convIn, preconcept2)
				}
				convIn = append(convIn,
					fmt.Sprintf(`if input.%s != `+nilS+` {
                        if err = v.SetValue(0, `+deRef+`input.%s); err != nil {
                            err = fmt.Errorf("error setting value %%v from %s: %%s", v, err)
                            return
                        }
                        }`,
						name, name, name))
				if preconcept2 != "" {
					convIn = append(convIn, "}")
				}
			} else {
				if preconcept2 != "" {
					convIn = append(convIn, preconcept2)
				}
				convIn = append(convIn,
					fmt.Sprintf(`for i, x := range input.%s {
                        if err = v.SetValue(uint(i), x.%s); err != nil {
                            err = fmt.Errorf("error setting value %%v[%%d] from %s%s: %%s", v, i, err)
                            return
                        }
                        }`, name, capitalize(key), name, capitalize(key)))
				if preconcept2 != "" {
					convIn = append(convIn, "}")
				}
			}
		}
	} else { // just and only input
		if tableSize == 0 {
			if preconcept2 != "" {
				convIn = append(convIn, preconcept2)
			}
			convIn = append(convIn,
				fmt.Sprintf(`v = input.%s`, name))
			if preconcept2 != "" {
				convIn = append(convIn, "} else { v = nil }")
			}
		} else {
			if preconcept2 != "" {
				convIn = append(convIn, preconcept2)
			}
			subGoType := got
			if subGoType[0] == '*' {
				subGoType = subGoType[1:]
			}
			convIn = append(convIn,
				fmt.Sprintf(`{
                var a []%s
                if len(input.%s) == 0 {
                    a = make([]%s, 0)
                } else {
                    a = make([]%s, len(input.%s))
                    for i, x := range input.%s {
                        if x != nil && x.%s != `+nilS+` { a[i] = `+deRef+`(x.%s) }
                    }
                }
                if v, err = ses.NewVar(a); err != nil {
                    err = fmt.Errorf("error creating new var from %s(%%v): %%s", a, err)
                    return
                }
                }
                    `,
					subGoType, name, subGoType, subGoType, name, name,
					capitalize(key), capitalize(key), name))
			if preconcept2 != "" {
				convIn = append(convIn, "}")
			}
		}
	}
	convIn = append(convIn,
		fmt.Sprintf("params[\"%s\"] = v\n", paramName))
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
	return getVarName(funName, varName, "x")
}

func getParamName(funName, paramName string) string {
	return getVarName(funName, paramName, "x")
}
