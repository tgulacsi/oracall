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
	"log"
	"strings"

	"github.com/tgulacsi/goracle/oracle"
)

// MaxTableSize is the maximum size of the array arguments
const MaxTableSize = 1000

//
// OracleArgument
//
func oracleVarType(typ string) *oracle.VariableType {
	switch typ {
	case "BINARY_INTEGER", "PLS_INTEGER":
		return oracle.Int32VarType
	case "STRING", "VARCHAR2":
		return oracle.StringVarType
	case "INTEGER":
		return oracle.Int64VarType
	case "ROWID":
		return oracle.RowidVarType
	case "REF CURSOR":
		return oracle.CursorVarType
	//case "TIMESTAMP":
	//return oracle.TimestampVarType
	case "NUMBER":
		return oracle.FloatVarType
	case "DATE":
		return oracle.DateTimeVarType
	case "BLOB":
		return oracle.BlobVarType
	case "CHAR":
		return oracle.FixedCharVarType
	case "BINARY":
		return oracle.BinaryVarType
	case "CLOB":
		return oracle.ClobVarType
	case "BOOLEAN", "PL/SQL BOOLEAN":
		return oracle.BooleanVarType
	default:
		log.Fatalf("oracleVarType: unknown variable type %q", typ)
	}
	return nil
}

func oracleVarTypeName(typ string) string {
	switch typ {
	case "BINARY_INTEGER", "PLS_INTEGER":
		return "oracle.Int32VarType"
	case "STRING", "VARCHAR2":
		return "oracle.StringVarType"
	case "INTEGER":
		return "oracle.Int64VarType"
	case "ROWID":
		return "oracle.RowidVarType"
	case "REF CURSOR":
		return "oracle.CursorVarType"
	//case "TIMESTAMP":
	//return oracle.TimestampVarType
	case "NUMBER":
		return "oracle.FloatVarType"
	case "DATE":
		return "oracle.DateTimeVarType"
	case "BLOB":
		return "oracle.BlobVarType"
	case "CHAR":
		return "oracle.FixedCharVarType"
	case "BINARY":
		return "oracle.BinaryVarType"
	case "CLOB":
		return "oracle.ClobVarType"
	case "BOOLEAN", "PL/SQL BOOLEAN":
		return "oracle.BooleanVarType"
	default:
		log.Fatalf("oracleVarTypeName: unknown variable type %q", typ)
	}
	return ""
}

// SavePlsqlBlock saves the plsql block definition into writer
func (fun Function) PlsqlBlock() (plsql, callFun string) {
	decls, pre, call, post, convIn, convOut, err := fun.prepareCall()
	if err != nil {
		log.Fatalf("error preparing %s: %s", fun, err)
	}
	fn := strings.Replace(fun.Name(), ".", "__", -1)
	callBuf := bytes.NewBuffer(make([]byte, 0, 16384))
	fmt.Fprintf(callBuf, `func Call_%s(cur *oracle.Cursor, input %s) (output %s, err error) {
    if err = input.Check(); err != nil {
        return
    }
    params := make(map[string]interface{}, %d)
    `, fn, fun.getStructName(false), fun.getStructName(true), len(fun.Args)+1)
	for _, line := range convIn {
		io.WriteString(callBuf, line+"\n")
	}
	i := strings.Index(call, fun.Name())
	j := i + strings.Index(call[i:], ")") + 1
	log.Printf("i=%d j=%d call=\n%s", i, j, call)
	fmt.Fprintf(callBuf, "\nif true || DebugLevel > 0 { log.Printf(`calling %s\n\twith %%#v`, params) }"+`
    if err = cur.Execute(%s, nil, params); err != nil { return }
    `, call[i:j], fun.getPlsqlConstName())
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
		log.Printf("nil types of %s", fun)
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
	convIn = append(convIn, "var v *oracle.Variable\nvar x interface{}\n _, _ = v, x")

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
			callArgs[arg.Name] = ":" + arg.Name
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

				//log.Printf("arg.Name=%q arg.TableOf.Name=%q arg.TableOf.RecordOf=%#v",
				//arg.Name, arg.TableOf.Name, arg.TableOf.RecordOf)
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
				log.Fatalf("%s/%s: only table of simple or record types are allowed (no table of table!)", fun.Name(), arg.Name)
			}
		default:
			log.Fatalf("unkown flavor %q", arg.Flavor)
		}
	}
	callb := bytes.NewBuffer(nil)
	if fun.Returns != nil {
		callb.WriteString(":ret := ")
	}
	log.Printf("callArgs=%s", callArgs)
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

func (arg Argument) getConvSimple(convIn, convOut []string, types map[string]string,
	name, paramName string, tableSize uint) ([]string, []string) {

	got := arg.goType(types)
	preconcept, preconcept2 := "", ""
	if strings.Count(name, ".") >= 1 {
		preconcept = "input." + name[:strings.LastIndex(name, ".")] + " != nil &&"
		preconcept2 = "if " + preconcept[:len(preconcept)-3] + " {"
	}
	if arg.IsOutput() {
		convIn = append(convIn,
			fmt.Sprintf(`if v, err = cur.NewVariable(%d, %s, %d); err != nil {
                    err = fmt.Errorf("error creating variable for type %s: %%s", err)
                    return
                }`,
				tableSize, oracleVarTypeName(arg.Type), arg.Charlength, arg.Type))
		pTyp := got[1:]
		switch pTyp {
		case "float32":
			pTyp = "float64"
		case "int", "int32":
			pTyp = "int64"
		}
		var outConv string
		if tableSize == 0 {
			if pTyp == got[1:] {
				outConv = fmt.Sprintf(`output.%s = &y`, name)
			} else {
				outConv = fmt.Sprintf(`z := %s(y)
            output.%s = &z`, got[1:], name)
			}
			convOut = append(convOut,
				fmt.Sprintf(`if params["%s"] != nil {
                v = params["%s"].(*oracle.Variable)
                if err = v.GetValueInto(&x, 0); err != nil {
                    err = fmt.Errorf("error getting value of %s: %%s", err)
                    return
                } else if x != nil {
                    y, ok := x.(%s)
                    if !ok {
                        err = fmt.Errorf("out parameter %s is bad type: awaited %s, got %%T", x)
                        return
                    }
                    %s
                }}
                `, paramName, paramName, name, pTyp, name, pTyp, outConv))
		} else {
			if pTyp == got[1:] {
				outConv = fmt.Sprintf(`output.%s[i] = &y`, name)
			} else {
				outConv = fmt.Sprintf(`z := %s(y)
            output.%s[i] = &z`, got[1:], name)
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
                    } else if x != nil {
                        y, ok := x.(%s)
                        if !ok {
                            err = fmt.Errorf("out parameter %s has bad type: awaited %s, got %%T", x)
                            return
                        }
                        %s
                    }
                }
            }
            `, paramName, paramName,
					mk,
					name,
					pTyp, name, pTyp, outConv))
		}
		if arg.IsInput() {
			if tableSize == 0 {
				if preconcept2 != "" {
					convIn = append(convIn, preconcept2)
				}
				convIn = append(convIn,
					fmt.Sprintf(`if input.%s != nil {
                            if err = v.SetValue(0, *input.%s); err != nil {
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
				fmt.Sprintf(`if input.%s != nil {
                        v, err = cur.NewVar(*input.%s)
                    } else {
                        v, err = cur.NewVariable(%d, %s, %d)
                    }
                    if err != nil {
                        err = fmt.Errorf("error creating new var from %s: %%s", err)
                        return
                    }`,
					name, name, tableSize, oracleVarTypeName(arg.Type), 0, name))
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
                        if x != nil { a[i] = *x }
                    }
                }
                if v, err = cur.NewVar(a); err != nil {
                    err = fmt.Errorf("error creating new var from %s(%%v): %%s", a, err)
                    return
                }
                }
                    `,
					subGoType, name, subGoType, subGoType, name, name, name))
			if preconcept2 != "" {
				convIn = append(convIn, "}")
			}
		}
	}
	convIn = append(convIn,
		fmt.Sprintf("params[\"%s\"] = v\n", paramName))
	return convIn, convOut
}

func (arg Argument) getConvRec(convIn, convOut []string,
	types map[string]string, name, paramName string, tableSize uint,
	parentArg *Argument, key string) ([]string, []string) {

	got := arg.goType(types)
	preconcept, preconcept2 := "", ""
	if strings.Count(name, ".") >= 1 {
		preconcept = "input." + name[:strings.LastIndex(name, ".")] + " != nil &&"
		preconcept2 = "if " + preconcept[:len(preconcept)-3] + " {"
	}
	if arg.IsOutput() {
		convIn = append(convIn,
			fmt.Sprintf(`if v, err = cur.NewVariable(%d, %s, %d); err != nil {
                    err = fmt.Errorf("error creating variable for type %s: %%s", err)
                    return
                }`,
				tableSize, oracleVarTypeName(arg.Type), arg.Charlength, arg.Type))
		pTyp := got[1:]
		switch pTyp {
		case "float32":
			pTyp = "float64"
		case "int", "int32":
			pTyp = "int64"
		}
		var outConv string
		if tableSize == 0 {
			if pTyp == got[1:] {
				outConv = fmt.Sprintf(`output.%s = &y`, name)
			} else {
				outConv = fmt.Sprintf(`z := %s(y)
            output.%s = &z`, got[1:], name)
			}
			convOut = append(convOut,
				fmt.Sprintf(`if params["%s"] != nil {
                v = params["%s"].(*oracle.Variable)
                if err = v.GetValueInto(&x, 0); err != nil {
                    err = fmt.Errorf("error getting value of %s: %%s", err)
                    return
                } else if x != nil {
                    y, ok := x.(%s)
                    if !ok {
                        err = fmt.Errorf("out parameter %s is bad type: awaited %s, got %%T", x)
                        return
                    }
                    %s
                }}
                `, paramName, paramName, name, pTyp, name, pTyp, outConv))
		} else {
			if pTyp == got[1:] {
				outConv = fmt.Sprintf(`output.%s[i].%s = &y`, name, capitalize(key))
			} else {
				outConv = fmt.Sprintf(`z := %s(y)
            output.%s[i].%s = &z`, got[1:], name, capitalize(key))
			}
			convOut = append(convOut,
				fmt.Sprintf(`if params["%s"] != nil {
                v = params["%s"].(*oracle.Variable)
                for i := 0; i < v.Len(); i++ {
                    if err = v.GetValueInto(&x, uint(i)); err != nil {
                        err = fmt.Errorf("error getting %%d. value into %s%s: %%s", i, err)
                        return
                    } else if x != nil {
                        y, ok := x.(%s)
                        if !ok {
                            err = fmt.Errorf("out parameter %s has bad type: awaited %s, got %%T", x)
                            return
                        }
                        %s
                    }
                }
            }
            `, paramName, paramName,
					name, key,
					pTyp, name, pTyp, outConv))
		}
		if arg.IsInput() {
			if tableSize == 0 {
				if preconcept2 != "" {
					convIn = append(convIn, preconcept2)
				}
				convIn = append(convIn,
					fmt.Sprintf(`if input.%s != nil {
                        if err = v.SetValue(0, *input.%s); err != nil {
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
                        if err = v.SetValue(uint(i), x%s); err != nil {
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
				fmt.Sprintf(`if input.%s != nil {
                        v, err = cur.NewVar(*input.%s)
                    } else {
                        v, err = cur.NewVariable(%d, %s, %d)
                    }
                    if err != nil {
                        err = fmt.Errorf("error creating new var from %s: %%s", err)
                        return
                    }`,
					name, name, tableSize, oracleVarTypeName(arg.Type), 0, name))
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
                        if x != nil && x.%s != nil { a[i] = *(x.%s) }
                    }
                }
                if v, err = cur.NewVar(a); err != nil {
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
