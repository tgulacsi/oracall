// Copyright 2013, 2022 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

// text/template is used for go code, not html.
// nosemgrep: go.lang.security.audit.xss.import-text-template.import-text-template
import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/UNO-SOFT/zlog/v2/slog"

	"github.com/godror/godror"
)

// MaxTableSize is the default size of the array elements
var MaxTableSize = 128

const batchSize = 1024

// SavePlsqlBlock saves the plsql block definition into writer
func (fun Function) PlsqlBlock(checkName string) (plsql, callFun string) {
	decls, pre, call, post, convIn, convOut, err := fun.prepareCall()
	if err != nil {
		logger.Error("error preparing", "function", fun, "error", err)
		panic(fmt.Errorf("%s: %w", fun.Name(), err))
	}
	fn := fun.name
	if fun.alias != "" {
		fn = fun.alias
	}
	fn = strings.Replace(fn, ".", "__", -1)

	plsBuf := Buffers.Get()
	defer Buffers.Put(plsBuf)
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
	if len(fun.handle) == 0 {
		plsBuf.WriteString("\n")
	} else {
		plsBuf.WriteString("  BEGIN\n  ")
	}
	fmt.Fprintf(plsBuf, "  %s;\n", call)
	if len(fun.handle) != 0 {
		fmt.Fprintf(plsBuf, "  EXCEPTION WHEN %s THEN NULL;\n  END;\n",
			strings.Join(fun.handle, " OR "))
	}
	plsBuf.WriteByte('\n')
	for _, line := range post {
		fmt.Fprintf(plsBuf, "  %s\n", line)
	}
	io.WriteString(plsBuf, "\nEND;\n")

	var check string
	if checkName != "" {
		check = fmt.Sprintf(`
	if err = %s(input); err != nil {
        return
    }
	`, checkName)
	}

	callBuf := Buffers.Get()
	defer Buffers.Put(callBuf)
	callBuf.Reset()

	hasCursorOut := fun.HasCursorOut()
	if hasCursorOut {
		fmt.Fprintf(callBuf, `func (s *oracallServer) %s(input *pb.%s, stream pb.%s_%sServer) (err error) {
			ctx := stream.Context()
			%s
			output := new(pb.%s)
			iterators := make([]iterator, 0, 1)
		`,
			CamelCase(fn), CamelCase(fun.getStructName(false, false)), CamelCase(fun.Package), CamelCase(fn),
			check,
			CamelCase(fun.getStructName(true, false)),
		)
	} else {
		fmt.Fprintf(callBuf, `func (s *oracallServer) %s(ctx context.Context, input *pb.%s) (output *pb.%s, err error) {
		%s
		output = new(pb.%s)
		iterators := make([]iterator, 0, 1) // just temporary
		_ = iterators
    `,
			CamelCase(fn), CamelCase(fun.getStructName(false, false)), CamelCase(fun.getStructName(true, false)),
			check,
			CamelCase(fun.getStructName(true, false)),
		)
	}
	fmt.Fprintf(callBuf, `
	logger := s.Logger
	if lgr := oracall.FromContext(ctx); lgr != nil {
		logger = lgr
	}
	if err = ctx.Err(); err != nil { return }
	`)
	for _, line := range convIn {
		io.WriteString(callBuf, line+"\n")
	}

	var pls string
	{
		var i int
		paramsMap := make(map[string][]int, bytes.Count(plsBuf.Bytes(), []byte{':'}))
		first := make(map[string]int, len(paramsMap))
		pls, _ = godror.MapToSlice(
			plsBuf.String(),
			func(key string) interface{} {
				paramsMap[key] = append(paramsMap[key], i)
				if _, ok := first[key]; !ok {
					first[key] = i
				}
				i++
				return key
			})
	}

	i := strings.Index(call, fun.RealName())
	if i < 0 {
		logger.Info("not found", "name", fun.RealName(), "in", call)
	}
	j := i + strings.Index(call[i:], ")") + 1
	fmt.Fprintf(callBuf, `
	const funName = "%s"
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var tx *sql.Tx
	if tx, err = s.db.BeginTx(ctx, nil); err != nil {
		return 
	}
	defer tx.Rollback()
	ctx = godror.ContextWithTraceTag(ctx, godror.TraceTag{Module: %q, Action: %q})
if s.DBLog != nil {
	var err error
	if ctx, err = s.DBLog(ctx, tx, funName, input); err != nil {
		logger.Error("dbLog", "fun", funName, "error", err)
	}
}
const callText = `+"`%s`"+`
if DebugLevel > 0 {
	logger.Debug("calling", "qry", callText, "stmt", `+"`%s`"+`)
}
	qry := %s
`,
		fun.Name(),
		fun.Package, fun.name,
		call[i:j], rIdentifier.ReplaceAllString(pls, "'%#v'"),
		fun.getPlsqlConstName(),
	)
	aS := "1024"
	if fun.maxTableSize > 0 {
		if fun.maxTableSize < 1<<16 {
			aS = strconv.Itoa(fun.maxTableSize)
		} else {
			aS = "65536"
		}
	}

	callBuf.WriteString(`
	stmt, stmtErr := tx.PrepareContext(ctx, qry)
	if stmtErr != nil {
		err = fmt.Errorf("%s: %w", qry, stmtErr)
		return
	}
	defer stmt.Close()
	stmtP := fmt.Sprintf("%p", stmt)
	dl, _ := ctx.Deadline()
	logger.Debug( "calling", "fun", funName, "input", input, "stmt", stmtP, "deadline", dl.UTC().Format(time.RFC3339))
	_, err = stmt.ExecContext(ctx, append(params, godror.PlSQLArrays, godror.ArraySize(` + aS + `))...)
	logger.Info( "finished", "fun", funName, "stmt", stmtP, "error", err)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if c, ok := err.(interface{ Code() int }); ok && c.Code() == 4068 {
			// "existing state of packages has been discarded"
			_, err = stmt.ExecContext(ctx, append(params, godror.PlSQLArrays, godror.ArraySize(` + aS + `))...)
		}
		if err != nil {
			qe := oracall.NewQueryError(qry, fmt.Errorf("%v: %w", params, err))
			err = qe
			if s.DBLog != nil {
				var logErr error
				if _, logErr = s.DBLog(ctx, tx, funName, err); logErr != nil {
					logger.Error("dbLog", "fun", funName, "error", logErr)
				}
			}
			if qe.Code() == 6502 {  // Numeric or Value Error
				err = fmt.Errorf("%+v: %w", qe, oracall.ErrInvalidArgument)
			}
			return
		}
	}
    `)

	callBuf.WriteString("\nif DebugLevel > 0 { logger.Debug(`result params`, params, `output`, output) }\n")
	for _, line := range convOut {
		io.WriteString(callBuf, line+"\n")
	}
	if !hasCursorOut {
		fmt.Fprintf(callBuf, "\nerr = tx.Commit()\nreturn\n")
	} else {
		fmt.Fprintf(callBuf, `
		if len(iterators) == 0 {
			if err = stream.Send(output); err == nil {
				err = tx.Commit()
			}
			return
		}
		iterators2 := make([]iterator, 0, len(iterators))
		for {
			for _, it := range iterators {
				if err = ctx.Err(); err != nil { return }
				err = it.Iterate()
				if sendErr := stream.Send(output); sendErr != nil && err == nil {
					err = sendErr
				}
				it.Reset()
				if err == nil {
					iterators2 = append(iterators2, it)
					continue
				}
				if !errors.Is(err, io.EOF) {
					logger.Error("iterate", "error", err)
					return
				}
			}
			if len(iterators) != len(iterators2) {
				if len(iterators2) == 0 {
					err = tx.Commit()
					return
				}
				iterators = append(iterators[:0], iterators2...)
			}
			iterators2 = iterators2[:0]
		}
		`)
	}
	callBuf.WriteString("\n}\n")
	callFun = callBuf.String()
	plsql = plsBuf.String()

	plsql, callFun = demap(plsql, callFun)
	return
}

func demap(plsql, callFun string) (string, string) {
	var i int
	paramsMap := make(map[string][]int, 16)
	first := make(map[string]int, len(paramsMap))
	plsql, paramsArr := godror.MapToSlice(
		plsql,
		func(key string) interface{} {
			paramsMap[key] = append(paramsMap[key], i)
			if _, ok := first[key]; !ok {
				first[key] = i
			}
			i++
			return key
		})

	type repl struct {
		ParamsArrLen int
	}
	opts := repl{
		ParamsArrLen: len(paramsArr),
	}
	callBuf := Buffers.Get()
	defer Buffers.Put(callBuf)
	var lastIdx int
	tpl, err := template.New("callFun").
		Funcs(
			map[string]interface{}{
				"paramsIdx": func(key string) int {
					if strings.HasSuffix(key, MarkHidden) {
						key = key[:len(key)-len(MarkHidden)] + "#"
					}
					arr := paramsMap[key]
					if len(arr) == 0 {
						logger.Info("paramsIdx", "key", key, "val", arr, "map", paramsMap)
					}
					i = arr[0]
					if len(arr) > 1 {
						paramsMap[key] = arr[1:]
					}
					lastIdx = i
					return i
				},
			}).
		Parse(callFun)
	if err != nil {
		fmt.Fprintln(os.Stderr, callFun)
		panic(err)
	}
	if err = tpl.Execute(callBuf, opts); err != nil {
		panic(err)
	}
	b, fmtErr := format.Source(callBuf.Bytes())
	if fmtErr != nil {
		panic(fmtErr)
	}
	callBuf.Reset()
	prev := make(map[string]string)
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		if line = bytes.TrimSpace(line); bytes.HasPrefix(line, []byte("params[")) && bytes.Contains(line, []byte("] = ")) {
			idx := string(line[:bytes.IndexByte(line, ']')+1])
			line = line[len(idx)+2:]
			if i = bytes.Index(line, []byte("//")); i >= 0 {
				line = line[:i]
			}
			prev[idx] = string(bytes.TrimSpace(line))
		}
	}
	callBuf.Write(b)

	plusIdxs := make([]idxRemap, 0, len(paramsMap))
	for k, vv := range paramsMap {
		for _, v := range vv {
			if i = first[k]; i != v {
				plusIdxs = append(plusIdxs, idxRemap{Name: k, New: v, Old: i})
			}
		}
	}
	if len(plusIdxs) == 0 {
		return plsql, callBuf.String()
	}

	sort.Sort(byNewRemap(plusIdxs))
	plus := Buffers.Get()
	defer Buffers.Put(plus)

	b = callBuf.Bytes()
	i = bytes.LastIndex(b, []byte(fmt.Sprintf("params[%d] =", lastIdx)))
	j := bytes.IndexByte(b[i:], '\n')
	j += i + 1
	rest := string(b[j:])
	callBuf.Truncate(j)

	for _, v := range plusIdxs {
		idx := fmt.Sprintf("params[%d]", v.Old)
		old := prev[idx]
		if old == "" {
			fmt.Fprintf(callBuf, "params[%d] = params[%d]  // %s\n", v.New, v.Old, v.Name)
			logger.Error("plusIdx", "v", v, "error", fmt.Errorf("cannot find %q in %+v", idx, prev))
		} else {
			if !strings.HasPrefix(old, "sql.Out{") {
				if old[0] != '&' {
					old = "&" + old
				}
				old = "sql.Out{Dest: " + old + "}"
			} else {
				old = strings.Replace(old, "In: true", "", 1)
			}
			fmt.Fprintf(callBuf, "params[%d] = %s  // %s\n", v.New, old, v.Name)
		}
	}
	callBuf.WriteString(rest)
	return plsql, callBuf.String()
}

func (fun Function) prepareCall() (decls, pre []string, call string, post []string, convIn, convOut []string, err error) {
	callArgs := make(map[string]string, 16)
	if repl := fun.Replacement; repl != nil {
		decls = append(decls, "v_in CLOB := :1;")
		convIn = append(convIn,
			"inCLOB := oracall.Buffers.Get(); defer oracall.Buffers.Put(inCLOB)",
			"var outCLOB string",
		)
		if fun.ReplacementIsJSON {
			convIn = append(convIn, "if err = json.NewEncoder(inCLOB).Encode(input); err != nil { return }")
		} else {
			convIn = append(convIn, "if err = xml.NewEncoder(inCLOB).Encode(input); err != nil { return }")
		}
		convIn = append(convIn,
			"params := []interface{}{inCLOB.String(), sql.Out{Dest:&outCLOB}}",
		)
		if fun.ReplacementIsJSON {
			convOut = append(convOut,
				`if err = json.NewDecoder(strings.NewReader(outCLOB)).Decode(&output); err != nil { err = fmt.Errorf("%s: %w", outCLOB,err); return; }`,
			)
		} else {
			convOut = append(convOut,
				`if err = xml.NewDecoder(strings.NewReader(outCLOB)).Decode(&output); err != nil { err = fmt.Errorf("%s: %w", outCLOB, err); return; }`,
			)
		}

		if repl.Returns != nil {
			call = fmt.Sprintf(":2 := %s(%s=>v_in)", repl.Name(), repl.Args[0].Name)
		} else {
			var argIn, argOut *Argument
			for i, a := range repl.Args {
				if a.Direction.IsOutput() {
					argOut = &repl.Args[i]
				} else if a.Direction.IsInput() {
					argIn = &repl.Args[i]
				}
			}
			call = fmt.Sprintf("%s(%s=>v_in, %s=>:2)", repl.RealName(), argIn.Name, argOut.Name)
		}
		return decls, pre, call, post, convIn, convOut, nil
	}

	tableTypes := make(map[string]string, 4)
	getTableType := func(absType string) string {
		if strings.HasPrefix(absType, "CHAR") {
			absType = "VARCHAR2" + absType[4:]
		}
		typ, ok := tableTypes[absType]
		if ok {
			return typ
		}
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
		return typ
	}
	//fStructIn, fStructOut := fun.getStructName(false), fun.getStructName(true)
	args := make([]Argument, 0, len(fun.Args)+1)
	for _, arg := range fun.Args {
		arg.Name = replHidden(arg.Name)
		args = append(args, arg)
	}
	if fun.Returns != nil {
		args = append(args, *fun.Returns)
	}
	for _, arg := range args {
		if arg.Flavor == FLAVOR_TABLE {
			break
		}
	}

	var (
		vn, tmp, typ string
		ok           bool
	)
	decls = append(decls, "i1 PLS_INTEGER;", "i2 PLS_INTEGER;")
	convIn = append(convIn,
		"params := make([]interface{}, {{.ParamsArrLen}}, {{.ParamsArrLen}}+2)",
	)

	addParam := func(paramName string) string {
		if paramName == "" {
			panic("empty param name")
		}
		return fmt.Sprintf(`params[{{paramsIdx %q}}]`, paramName)
	}
	maxTableSize := fun.maxTableSize
	if maxTableSize <= 0 {
		maxTableSize = MaxTableSize
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
			if arg.TypeName == "" {
				arg.TypeName = mkRecTypName(arg.Name)
				decls = append(decls, "TYPE "+arg.TypeName+" IS RECORD (")
				for i, sub := range arg.RecordOf {
					var comma string
					if i != 0 {
						comma = ","
					}
					decls = append(decls, "  "+comma+sub.Name+" "+sub.AbsType)
				}
				decls = append(decls, ");")
			}
			decls = append(decls, vn+" "+arg.TypeName+ " := " + arg.TypeName + "()" + "; --E="+arg.Name)
			callArgs[arg.Name] = vn
			aname := (CamelCase(arg.Name))
			//aname := capitalize(replHidden(arg.Name))
			if arg.IsOutput() {
				var got string
				if got, err = arg.goType(false); err != nil {
					return
				}
				if arg.IsInput() {
					convIn = append(convIn, fmt.Sprintf(`
					output.%s = new(%s)  // sr1
					if input.%s != nil { *output.%s = *input.%s }
					`, aname, withPb(CamelCase(got[1:])),
						aname, aname, aname))
				} else {
					var got string
					if got, err = arg.goType(false); err != nil {
						return
					}
					// yes, convIn - getConvRec uses this!
					convIn = append(convIn, fmt.Sprintf(`
                    if output.%s == nil {
                        output.%s = new(%s)  // sr2
                    }`, aname,
						aname, withPb(CamelCase(got[1:]))))
				}
			}
			for _, a := range arg.RecordOf {
				a := a
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
					0, arg, k, maxTableSize)
			}
		case FLAVOR_TABLE:
			if arg.Type == "REF CURSOR" {
				if arg.IsInput() {
					logger.Info("cannot use IN cursor variables", "arg", arg)
					panic(fmt.Sprintf("cannot use IN cursor variables (%v)", arg))
				}
				name := (CamelCase(arg.Name))
				//name := capitalize(replHidden(arg.Name))
				convIn, convOut = arg.getConvSimpleTable(convIn, convOut,
					name, addParam(arg.Name), maxTableSize)
			} else {
				switch arg.TableOf.Flavor {
				case FLAVOR_SIMPLE: // like simple, but for the arg.TableOf
					typ = getTableType(arg.TableOf.AbsType)
					if strings.IndexByte(typ, '/') >= 0 {
						err = fmt.Errorf("nonsense table type of %s", arg)
						return
					}

					setvar := ""
					if arg.IsInput() {
						setvar = " := :" + arg.Name
					}
					decls = append(decls, arg.Name+" "+typ+setvar+"; --A="+arg.Name)

					vn = getInnerVarName(fun.Name(), arg.Name)
					callArgs[arg.Name] = vn
					decls = append(decls, vn+" "+arg.TypeName+ " := " + arg.TypeName + "()" + "; --B="+arg.Name)
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
						name, addParam(arg.Name), maxTableSize)

				case FLAVOR_RECORD:
					vn = getInnerVarName(fun.Name(), arg.Name+"."+arg.TableOf.Name)
					callArgs[arg.Name] = vn
					decls = append(decls, vn+" "+arg.TypeName+ " := " + arg.TypeName + "()" + "; --C="+arg.Name)

					aname := (CamelCase(arg.Name))
					//aname := capitalize(replHidden(arg.Name))
					if arg.IsOutput() {
						var tgot string
						if tgot, err = arg.TableOf.goType(true); err != nil {
							return
						}
						st := withPb(CamelCase(tgot))
						convOut = append(convOut, fmt.Sprintf(`
					if m := %d - cap(output.%s); m > 0 { // %s
						output.%s = append(output.%s[:cap(output.%s)], make([]%s, m)...) // fr1
                    }
					output.%s = output.%s[:%d]
					`,
							maxTableSize, aname, tgot,
							aname, aname, aname, st,
							aname, aname, maxTableSize))
					}
					if !arg.IsInput() {
						pre = append(pre, vn+".DELETE;")
					}

					// declarations go first
					for _, a := range arg.TableOf.RecordOf {
						a := a
						k, v := a.Name, a.Argument
						typ = getTableType(v.AbsType)
						if strings.IndexByte(typ, '/') >= 0 {
							err = fmt.Errorf("nonsense table type of %s", arg)
							return
						}
						decls = append(decls, getParamName(fun.Name(), vn+"."+k)+" "+typ+ " := " + typ + "()" + "; --D="+arg.Name)

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
						a := a
						k, v := a.Name, a.Argument

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
							uint(maxTableSize),
							k, *arg.TableOf)

						if arg.IsInput() {
							if arg.IsNestedTable() {
								pre = append(pre,
									"  "+vn+".extend;")
							}
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
							a := a
							k := a.Name
							tmp = getParamName(fun.Name(), vn+"."+k)
							post = append(post, ":"+tmp+" := "+tmp+";")
						}
					}
				default:
					logger.Info("Only table of simple or record types are allowed (no table of table!)", "function", fun.Name(), "arg", arg.Name)
					panic(fmt.Errorf("only table of simple or record types are allowed (no table of table!) - %s(%v)", fun.Name(), arg.Name))
				}
			}
		default:
			logger.Info("unkown flavor", "flavor", arg.Flavor)
			panic(fmt.Errorf("unknown flavor %s(%v)", fun.Name(), arg.Name))
		}
	}

	callb := Buffers.Get()
	defer Buffers.Put(callb)
	if fun.Returns != nil {
		callb.WriteString(":ret := ")
	}
	callb.WriteString(fun.RealName() + "(")
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
	name, paramName string,
) ([]string, []string) {
	if !arg.IsOutput() {
		in, _ := arg.ToOra(paramName, "input."+name, arg.Direction)
		convIn = append(convIn, in+"  // gcs4i")
	} else {
		got, err := arg.goType(false)
		if err != nil {
			panic(err)
		}
		if got[0] == '*' {
			//convIn = append(convIn, fmt.Sprintf("output.%s = new(%s) // %s  // gcs1", name, got[1:], got))
			if arg.IsInput() {
				convIn = append(convIn, fmt.Sprintf(`if input.%s != nil { *output.%s = *input.%s }  // gcs2`, name, name, name))
			}
		} else if arg.IsInput() {
			convIn = append(convIn, fmt.Sprintf(`output.%s = input.%s  // gcs3`, name, name))
		}
		if got == "time.Time" {
			convOut = append(convOut, fmt.Sprintf("if output.%s != nil && !output.%s.IsValid() { output.%s = nil }", name, name, name))
		}
		src := "output." + name
		in, varName := arg.ToOra(paramName, "&"+src, arg.Direction)
		convIn = append(convIn, in)
		//fmt.Sprintf("%s = sql.Out{Dest:%s,In:%t}  // gcs3", paramName, "&"+src, arg.IsInput()))
		if varName != "" {
			convOut = append(convOut,
				fmt.Sprintf("%s  // gcs4", arg.FromOra(src, paramName, varName)))
		}
	}
	return convIn, convOut
}

func (arg Argument) getConvSimpleTable(
	convIn, convOut []string,
	name, paramName string,
	tableSize int,
) ([]string, []string) {
	if arg.IsOutput() {
		got, err := arg.goType(true)
		if err != nil {
			panic(err)
		}
		if arg.Type == "REF CURSOR" {
			return arg.getConvRefCursor(convIn, convOut, name, paramName, tableSize)
		}
		if strings.HasPrefix(got, "*[]") { // FIXME(tgulacsi): just a hack, ProtoBuf never generates a pointer to a slice
			got = got[1:]
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
				if got == "[]godror.Number" {
					convIn = append(convIn,
						fmt.Sprintf("output.%s = make([]string, 0, %d) // gcst3", name, tableSize))
				} else {
					convIn = append(convIn,
						fmt.Sprintf("output.%s = make(%s, 0, %d) // gcst3", name, got, tableSize))
				}
			}
		}
		in, varName := arg.ToOra(
			strings.Replace(strings.Replace(paramName, `[{{paramsIdx "`, "__", 1), `"}}]`, "", 1),
			"output."+name,
			arg.Direction)
		convIn = append(convIn,
			//fmt.Sprintf(`if cap(input.%s) == 0 { input.%s = append(input.%s, make(%s, 1)...)[:0] }`, name, name, name, arg.goType(true)[1:]),
			fmt.Sprintf(`// in=%q varName=%q`, in, varName))
		if got == "[]godror.Number" { // don't copy, hack
			convIn = append(convIn,
				fmt.Sprintf(`if cap(output.%s) == 0 { output.%s = make([]string, 0, %d) }`, name, name, tableSize),
				fmt.Sprintf(`%s = sql.Out{Dest: custom.NumbersFromStrings(&output.%s), In:%t}  // gcst1`, paramName, name, arg.IsInput()))
		} else {
			convIn = append(convIn, fmt.Sprintf(`%s = sql.Out{Dest: &output.%s, In:%t} // gcst1`, paramName, name, arg.IsInput()))
		}
	} else {
		in, varName := arg.ToOra(
			strings.Replace(strings.Replace(paramName, `[{{paramsIdx "`, "__", 1), `"}}]`, "", 1),
			"output."+name,
			arg.Direction)
		convIn = append(convIn,
			fmt.Sprintf(`// in=%q varName=%q`, in, varName))
		if got, _ := arg.goType(true); got == "[]godror.Number" {
			convIn = append(convIn,
				fmt.Sprintf(`if len(input.%s) == 0 { %s = []godror.Number{} } else {
			%s = *custom.NumbersFromStrings(&input.%s) // gcst2
		}`,
					name, paramName,
					paramName, name))
		} else {
			convIn = append(convIn, fmt.Sprintf("%s = input.%s // gcst2", paramName, name))
		}
	}
	return convIn, convOut
}

func (arg Argument) getConvRefCursor(
	convIn, convOut []string,
	name, paramName string,
	tableSize int,
) ([]string, []string) {
	got, err := arg.goType(true)
	if err != nil {
		panic(err)
	}
	GoT := withPb(CamelCase(got))
	convIn = append(convIn, fmt.Sprintf(`output.%s = make([]%s, 0, %d)  // gcrf1
		%s = sql.Out{Dest:new(driver.Rows)} // gcrf1 %q`,
		name, GoT, tableSize,
		paramName, got))

	convOut = append(convOut, fmt.Sprintf(`
	{
		rset := *(%s.(sql.Out).Dest.(*driver.Rows))
		if rset != nil { 
			defer rset.Close()
			iterators = append(iterators, iterator{
				Reset: func() { output.%s = output.%s[:0] },
				Iterate: func() error {
			a := output.%s[:0]
			I := make([]driver.Value, %d)
			var err error
			for i := 0; i < %d; i++ {
				if err = rset.Next(I); err != nil {
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
		name, name,
		name,
		len(arg.TableOf.RecordOf),
		batchSize,
		arg.getFromRset("I"),
		name,
	))
	return convIn, convOut
}

func (arg Argument) getFromRset(rsetRow string) string {
	buf := Buffers.Get()
	defer Buffers.Put(buf)

	got, err := arg.goType(true)
	if err != nil {
		panic(err)
	}
	GoT := CamelCase(got)
	if GoT[0] == '*' {
		GoT = "&" + GoT[1:]
	}
	fmt.Fprintf(buf, "%s{\n", withPb(GoT))
	for i, a := range arg.TableOf.RecordOf {
		a := a
		got, err = a.Argument.goType(true)
		if err != nil {
			panic(err)
		}
		if strings.Contains(got, ".") {
			fmt.Fprintf(buf, "\t%s: %s, // %s\n", CamelCase(a.Name),
				a.GetOra(fmt.Sprintf("%s[%d]", rsetRow, i), ""),
				got)
		} else {
			fmt.Fprintf(buf, "\t%s: custom.As%s(%s[%d]), // %s\n", CamelCase(a.Name), CamelCase(got), rsetRow, i,
				got)
		}
	}
	fmt.Fprintf(buf, "}")
	return buf.String()
}

/*
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
*/
func (arg Argument) getConvRec(
	convIn, convOut []string,
	name, paramName string,
	tableSize uint,
	parentArg Argument,
	key string,
	maxTableSize int,
) ([]string, []string) {

	if arg.IsOutput() {
		too, varName := arg.ToOra(paramName, "&output."+name, arg.Direction)
		if arg.TableOf != nil {
			st, err := arg.TableOf.goType(true)
			if err != nil {
				panic(err)
			}
			convIn = append(convIn, fmt.Sprintf(`
					if %d > cap(output.%s) {
						output.%s = append(make([]%s, 0, %d), output.%s...) // gcr2-fr1
                    }
					`,
				maxTableSize, name,
				name, st, maxTableSize, name),
			)
		}

		convIn = append(convIn, too+" // gcr2 var="+varName)
		if varName != "" {
			convIn = append(convIn, fmt.Sprintf("%s = sql.Out{Dest:&%s, In:%t} // gcr2out", paramName, varName, arg.IsInput()))
			convOut = append(convOut, arg.FromOra("output."+name, varName, varName)+" // gcr2out")
		}
	} else if arg.IsInput() {
		parts := strings.Split(name, ".")
		too, _ := arg.ToOra(paramName, "input."+name, arg.Direction)
		convIn = append(convIn,
			fmt.Sprintf(`if input.%s != nil {
				%s
			} // gcr1`,
				parts[0], too))
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
	absName := "x__" + name[0] + "__" + name[1]
	typ, err := arg.goType(true)
	if err != nil {
		panic(err)
	}
	oraTyp := typ
	switch oraTyp {
	case "custom.Date", "custom.DateTime":
		oraTyp = "time.Time"
	case "float64":
		oraTyp = "float64"
	case "int32":
		oraTyp = "int32"
	}
	if arg.IsInput() {
		lengthS := "len(input." + name[0] + ")"
		too, _ := arg.ToOra(absName+"[i]", "v."+name[1], arg.Direction)
		setParams := absName
		if arg.IsOutput() {
			setParams = fmt.Sprintf("sql.Out{Dest:&%s,In:true} //gctr1", absName)
		}
		convIn = append(convIn, fmt.Sprintf(`
			%s := make([]%s, %s, %d)  // gctr1
			for i,v := range input.%s {
				%s
			} // gctr1
			%s = %s`,
			absName, oraTyp, lengthS, tableSize,
			name[0],
			too,
			paramName, setParams))
		_ = too
	}
	if arg.IsOutput() {
		if !arg.IsInput() {
			convIn = append(convIn,
				fmt.Sprintf(`%s := make([]%s, 0, %d)  // gctr2
				%s = sql.Out{Dest:&%s} // gctr2`,
					absName, oraTyp, tableSize,
					paramName, absName))
		}
		got, err := parent.goType(true)
		if err != nil {
			panic(err)
		}
		convert := arg.FromOra(fmt.Sprintf("output.%s[i].%s", name[0], name[1]), "v", "v")
		if !Gogo && oraTyp == "time.Time" {
			convert = fmt.Sprintf("output.%s[i].%s = timestamppb.New(v)", name[0], name[1])
		}

		convOut = append(convOut,
			fmt.Sprintf(`if m := len(%s)-cap(output.%s); m > 0 { // gctr3
			output.%s = append(output.%s, make([]%s, m)...)
		}
		output.%s = output.%s[:len(%s)]
		for i, v := range %s {
			if output.%s[i] == nil {
				output.%s[i] = new(%s)
			}
			%s // gctr3
		}`,
				absName, name[0],
				name[0], name[0], withPb(CamelCase(got)),
				name[0], name[0], absName,
				absName,
				name[0],
				name[0], withPb(CamelCase(got[1:])),
				convert,
			))
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

func withPb(s string) string {
	if s == "" {
		return s
	}
	if s[0] == '*' || s[0] == '&' {
		return s[:1] + "pb." + s[1:]
	}
	return "pb." + s
}

type idxRemap struct {
	Name     string
	New, Old int
}

var _ = sort.Interface(byNewRemap(nil))

type byNewRemap []idxRemap

func (s byNewRemap) Len() int           { return len(s) }
func (s byNewRemap) Less(i, j int) bool { return s[i].Old < s[j].Old }
func (s byNewRemap) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type ctxLogger struct{}

func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxLogger{}, logger)
}
func FromContext(ctx context.Context) *slog.Logger {
	logger, _ := ctx.Value(ctxLogger{}).(*slog.Logger)
	return logger
}

// vim: se noet fileencoding=utf-8:
