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
const nullable = true
const useNil = true

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
		log.Fatalf("unknown variable type %q", typ)
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
		log.Fatalf("unknown variable type %q", typ)
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
	fmt.Fprintf(callBuf, `
    if err = cur.Execute(%s, nil, params); err != nil { return }
    `, fun.getPlsqlConstName())
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
	callArgs := make(map[string]string, 16)
	//fStructIn, fStructOut := fun.getStructName(false), fun.getStructName(true)
	var (
		vn, tmp string
		ok      bool
	)
	decls = append(decls, "i1 PLS_INTEGER;", "i2 PLS_INTEGER;")
	convIn = append(convIn, "var v *oracle.Variable\nvar x interface{}\n _, _ = v, x")
	for _, arg := range fun.Args {
		if arg.Flavor != FLAVOR_SIMPLE {
			if arg.IsInput() {
				pre = append(pre, "", "--"+arg.String())
			}
			if arg.IsOutput() {
				post = append(post, "", "--"+arg.String())
			}
		}
		switch arg.Flavor {
		case FLAVOR_SIMPLE:
			name := capitalize(goName(arg.Name))
			convIn, convOut = arg.getConv(convIn, convOut, fun.types, name, arg.Name, 0)

		case FLAVOR_RECORD:
			vn = getInnerVarName(fun.Name(), arg.Name)
			callArgs[arg.Name] = vn
			decls = append(decls, vn+" "+arg.TypeName+";")
			aname := capitalize(goName(arg.Name))
			for k, v := range arg.RecordOf {
				tmp = getParamName(fun.Name(), vn+"."+k)
				name := aname + "." + capitalize(goName(k))
				if arg.IsOutput() {
					post = append(post, ":"+tmp+"."+k+" := "+vn+"."+k+";")
				}
				convIn, convOut = v.getConv(convIn, convOut, fun.types, name, tmp, 0)

				if arg.IsInput() {
					pre = append(pre, vn+"."+k+" := :"+tmp+";")
				}
			}
		case FLAVOR_TABLE:
			switch arg.TableOf.Flavor {
			case FLAVOR_SIMPLE:
				name := capitalize(goName(arg.Name))
				convIn, convOut = arg.TableOf.getConv(convIn, convOut, fun.types, name, arg.Name, MaxTableSize)

			case FLAVOR_RECORD:
				vn = getInnerVarName(fun.Name(), arg.Name+"."+arg.TableOf.Name)
				callArgs[arg.Name] = vn
				decls = append(decls, vn+" "+arg.TableOf.TypeName+";")

				if arg.IsOutput() {
					// DELETE out tables
					for k := range arg.TableOf.RecordOf {
						post = append(post,
							":"+getParamName(fun.Name(), vn+"."+k)+".DELETE;")
					}
				}

				var idxvar string
				for k := range arg.TableOf.RecordOf {
					if idxvar == "" {
						idxvar = getParamName(fun.Name(), vn+"."+k)
						if arg.IsInput() {
							pre = append(pre,
								"i1 := :"+idxvar+".FIRST;",
								"WHILE i1 IS NOT NULL LOOP")
						}
						if arg.IsOutput() {
							post = append(post,
								"i1 := "+vn+".FIRST; i2 := 1;",
								"WHILE i1 IS NOT NULL LOOP")
						}
					}
					tmp = getParamName(fun.Name(), vn+"."+k)
					if arg.IsInput() {
						pre = append(pre,
							"  "+vn+"(i1)."+k+" := :"+tmp+"(i1);")
					}
					if arg.IsOutput() {
						post = append(post,
							"  :"+tmp+"(i2) := "+vn+"(i1)."+k+";")
					}
				}
				if arg.IsInput() {
					pre = append(pre,
						"  i1 := :"+idxvar+".NEXT(i1);",
						"END LOOP;")
				}
				if arg.IsOutput() {
					post = append(post,
						"  i1 := "+vn+".NEXT(i1); i2 := i2 + 1;",
						"END LOOP;")
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

func (arg Argument) getConv(convIn, convOut []string, types map[string]string, name, paramName string, tableSize uint) ([]string, []string) {
	got := arg.goType(types)
	nullable := strings.HasPrefix(got, "Null") || strings.HasPrefix(got, "sql.Null")
	preconcept, preconcept2 := "", ""
	if strings.Count(name, ".") >= 1 {
		if useNil {
			preconcept = "input." + name[:strings.LastIndex(name, ".")] + " != nil &&"
			preconcept2 = "if " + preconcept[:len(preconcept)-3] + " {"
		} else {
			i := strings.LastIndex(name, ".")
			preconcept = "input." + name[:i] + "Valid &&"
			preconcept2 = "if " + preconcept[:len(preconcept)-3] + " {"
			name = name[:i] + ".Struct" + name[i:]
		}
	}
	valueName := ""
	if nullable {
		valueName = got[strings.Index(got, "Null")+4:]
	}
	if arg.IsOutput() {
		convIn = append(convIn,
			fmt.Sprintf("if v, err = cur.NewVariable(%d, %s, %d); err != nil { return }",
				tableSize, oracleVarTypeName(arg.Type), arg.Charlength))
		if !useNil && nullable {
			convOut = append(convOut,
				fmt.Sprintf(`if params["%s"] != nil {
                v = params["%s"].(*oracle.Variable)
                if err = v.GetValueInto(&x, 0); err != nil {
                    return
                } else if x != nil {
                    output.%s.%s = x.(%s)
                    output.%s.Valid = true
                }}
            `, paramName, paramName, name, valueName, strings.ToLower(valueName), name))
		} else if useNil { //got == "time.Time" {
			convOut = append(convOut,
				fmt.Sprintf(`if params["%s"] != nil {
                v = params["%s"].(*oracle.Variable)
                if err = v.GetValueInto(&x, 0); err != nil {
                    return
                } else if x != nil {
                    output.%s = x.(%s)
                }}
            `, paramName, paramName, name, got))
		} else {
			convOut = append(convOut,
				fmt.Sprintf(`if params["%s"] != nil {
                v = params["%s"].(*oracle.Variable)
                if err = v.GetValueInto(&output.%s, 0); err != nil { return }
                }
            `, paramName, paramName, name))
		}
		if arg.IsInput() {
			if tableSize == 0 {
				if !useNil && nullable {
					convIn = append(convIn,
						fmt.Sprintf(`if %s input.%s.Valid {
                            if err = v.SetValue(0, input.%s.%s); err != nil { return }
                        }`,
							preconcept, name, name, valueName))
				} else {
					if preconcept2 != "" {
						convIn = append(convIn, preconcept2)
					}
					convIn = append(convIn,
						fmt.Sprintf("if err = v.SetValue(0, input.%s); err != nil { return }",
							name))
					if preconcept2 != "" {
						convIn = append(convIn, "}")
					}
				}
			} else {
				if preconcept2 != "" {
					convIn = append(convIn, preconcept2)
				}
				if !useNil && nullable {
					convIn = append(convIn,
						fmt.Sprintf(`for i, x := range input.%s {
                                if x.Valid {
                                if err = v.SetValue(uint(i), x.%s); err != nil { return }
                                }
                            }`, name, valueName))
				} else {
					convIn = append(convIn,
						fmt.Sprintf(`for i, x := range input.%s {
                                if err = v.SetValue(uint(i), x); err != nil { return }
                            }`, name))
				}
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
			if !useNil && nullable {
				convIn = append(convIn,
					fmt.Sprintf(`if input.%s.Valid {
                    if v, err = cur.NewVar(input.%s.%s); err != nil { return }
                    }`,
						name, name, valueName))
			} else {
				convIn = append(convIn,
					fmt.Sprintf("if v, err = cur.NewVar(input.%s); err != nil {return }",
						name))
			}
			if preconcept2 != "" {
				convIn = append(convIn, "} else { v = nil }")
			}
		} else {
			if preconcept2 != "" {
				convIn = append(convIn, preconcept2)
			}
			if !useNil && nullable {
				subGoType := strings.ToLower(got[strings.Index(got, "Null")+4:])
				convIn = append(convIn,
					fmt.Sprintf(`a := make([]%s, len(input.%s))
                    for i, x := range input.%s {
                        if x.Valid { a[i] = x.%s }
                    }
                    if v, err = cur.NewVar(a); err != nil {return }
                    `,
						subGoType, name, name, valueName))
			} else {
				convIn = append(convIn,
					fmt.Sprintf("if v, err = cur.NewVar(input.%s); err != nil {return }",
						name))
			}
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
		x = fmt.Sprintf("%s%03d", prefix, len(m)+1)
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

/*

class OraType(object):
    def __init__(self, typname, obj=None):
        self.typ = (c_typeconv_d[typname] if isinstance(typname, basestring)
                else typname)
        self.obj = obj

    char_length = property(
        lambda s: getattr(s.obj, 'char_length', -1) if s.obj else -1)

    def __eq__(self, other):
        return self.typ == other

    def __str__(self):
        result = str(self.typ)
        length = self.char_length
        if length > 0:
            result += '(%d)' % length
        return result

    @classmethod
    def revert(cls, obj):
        return obj.typ if isinstance(obj, cls) else obj


type OracleArgument struct {
    Function,VarName string
    alg uint8
    Argument
}

func NewOracleArgument(funName , varName string, argument Argument) {
    arg := OracleArgument{Function:funName, VarName: varName,
    Argument:argument}
    otyp := argument.Type
        data_type = self.data_type
        if not (data_type in c_typeconv_d or data_type.startswith('PL/SQL')):
            LOG.warning('unknown datatype at (%s): %s', self.var_name,
                    data_type)
        assert (data_type in c_typeconv_d
                        or data_type in ('PL/SQL TABLE', 'PL/SQL RECORD',)),\
            'UNKNOWN TYPE: |%s| (%s.%s %s)' % (data_type,
                    function.package.name,
                    function.name, name)
        self.ora_type = c_typeconv_d.get(data_type, data_type)
        self.data_level = data_level
        self.has_default = has_default
        tup = (tuple(map(lambda z: int(z.strip()),
            otyp[otyp.index('('):].strip('() ').split(',')))
            if '(' in otyp else (-1, -1))
        del otyp
        self.data_precision = data_precision or tup[0]
        self.data_length = data_length or tup[0]
        self.data_scale = data_scale or tup[1]
        argdbg = '%s.%s(%s %s %s [%d])' % (function.package.name,
            function.name, name, in_out, data_type, position)
        if char_length is None:
            LOG.warn('char_length is None for %s', argdbg)
            char_length = -1
        assert (data_type not in ('VARCHAR2', 'CHAR')
                or isinstance(char_length, int)), argdbg
        self._set_lengths(data_type, data_length, data_precision, data_scale,
            char_length)

        assert type_name is None or isinstance(type_name, basestring)
        self.type_name = type_name
        self._children = children or []
        if char_length > -1:
            self.char_length = char_length
        elif data_type in ('VARCHAR2', 'CHAR') and data_length > -1:
            self.char_length = data_length
        else:
            self.char_length = -1
        self._ensure_alg()

    def _set_lengths(self, data_type, data_length, data_precision, data_scale,
            char_length):
        if data_type in ('VARCHAR2', 'CHAR'):
            if char_length == -1:
                char_length = {'VARCHAR2': 1000, 'CHAR': 10}[data_type]
            elif char_length > 4000:
                char_length = 4000
            self.abstype = '%s(%d)' % (data_type, char_length)
        elif data_type == 'NUMBER':
            if data_scale > -1:
                self.abstype = 'NUMBER(%d, %d)' % (data_precision, data_scale)
            elif data_precision > -1:
                self.abstype = 'NUMBER(%d)' % data_precision
            else:
                self.abstype = 'NUMBER'
        else:
            self.abstype = data_type

    def _ensure_name(self):
        if not hasattr(self, 'name') and hasattr(self, '__name__'):
            self.name = self.__name__

    def _ensure_alg(self):
        self._ensure_name()
        self.var_prefix = self.var_name(self.name
                    + '_' + {1: 'in', 2: 'inout', 3: 'out'}[self.in_out],
                    shortest=True)
        self._var = None
        if self.data_type.startswith('PL/SQL '):
            if not self._children:
                return False
            if self.data_type.endswith(' RECORD'):
                self._alg = self.TYP_RECORD
                self._typ = dict((child.name,
                        OraType(c_typeconv_d[child.data_type], child))
                    for child in self._children)
                self._var = dict((child.name, self.inner_var_name(child))
                        for child in self._children)
            elif self.data_type.endswith(' TABLE'):
                if self._children[0].children:
                    self._alg = self.TYP_TOR
                    try:
                        self._typ = [dict((child.name,
                                OraType(c_typeconv_d[child.data_type], child))
                            for child in self._children[0].children)]
                    except:
                        LOG.exception('OraType error for %r',
                                list(self._children[0].children))
                        raise
                    self._var = [dict((child.name, self.inner_var_name(child))
                        for child in self._children[0].children)]
                else:
                    self._alg = self.TYP_TOS
                    self._typ = [
                            OraType(c_typeconv_d[self._children[0].data_type],
                                self._children[0])]
                    self._var = [None]
        else:
            self._alg = self.TYP_SIMPLE
            self._typ = OraType(c_typeconv_d[self.data_type], self)
        #
        return self._alg

    def inner_var_name(self, child):
        return self.var_name(SEP.join((self.var_prefix, child.name)))

    def add_child(self, child):
        assert isinstance(child, OracleArgument)
        super(OracleArgument, self).add_child(child)

    def call_arg(self, package_name=None):
        '''prepares calling sql block'''
        decl_types = set()
        decl_vars = None
        var = None
        pre = None
        post = None
        end = None
        if self.genre in (self.TYP_SIMPLE, self.TYP_TOS):
            if self.name == 'ret':
                var = ':ret := '
            else:
                var = '%s=>:%s' % (self.name, self.name)
        elif self.genre in (self.TYP_TOR, self.TYP_RECORD):
            var_name = self.var_name(self.var_prefix + SEP)
            tn = self.type_name or self.data_type
            var_type = (tn
                    if ('.' in tn or '%' in tn or not package_name)
                    else package_name + '.' + tn)
            del tn
            var = '%s=>%s' % (self.name, var_name)
            decl_vars = ['  ' + var_name + ' ' + var_type + ';']
            pre = []
            post = []
            end = []
            inner_types = {}
            if self.genre == self.TYP_RECORD:
                self._ca_hlp_record(var_name, pre, post)
            else:
                self._ca_hlp_nonrec(pre, post, end, var_name,
                        inner_types, decl_types, decl_vars)
            #
            Z = lambda a: '  ' + '\n  '.join(a) + '\n' if a else ''
            pre = Z(pre)
            post = (Z(post) + Z(end))
        else:
            LOG.warning('WARNING: genre = %s at %s' % (self.genre, self.name))
        #
        return (decl_types, decl_vars, pre, var, post)

    def _ca_hlp_record(self, var_name, pre, post):
        for child in self._children:
            inner_var = self._var[child.name]
            if self.input:
                pre.append('  %s.%s := :%s;' % (var_name, child.name,
                    inner_var))
            if self.output:
                post.append('  :%s := %s.%s;' % (inner_var, var_name,
                    child.name))

    def _ca_hlp_nonrec(self, pre, post, end, var_name, inner_types,
            decl_types, decl_vars):
        idx_name = self.var_name(var_name + '_idx')
        first_var_name = self.var_name(
            SEP.join((var_name, self._children[0].children[0].name)))
        pre.append('%s.DELETE;' % var_name)
        if self.justoutput:
            pre.append(' '.join('%s.DELETE;' % inner_var
                for inner_var in self._var[0].itervalues()))
        decl_vars.append('%s PLS_INTEGER := NULL;' % idx_name)
        alist = [
            # "DB_LOG.esemeny('%s.FIRST='||%s, 'w');" % (idx_name, idx_name),
            # 'WHILE %s IS NOT NULL AND %s BETWEEN 1 AND %d LOOP' % (
            #     idx_name, idx_name, TABLE_SIZE - 1)]
            'WHILE %s IS NOT NULL LOOP' % idx_name]  # let it bail
        post_pos = 0
        if self.input:
            pre.append('%s := %s.FIRST;' % (idx_name, first_var_name))
            pre.extend(alist)
        if self.output:
            post_pos = len(post)
            post.append('%s := %s.FIRST;' % (idx_name, var_name))
            post.extend(alist)

        self._ca_hlp_chld_loop(pre, post, end, var_name, idx_name,
                post_pos, inner_types, decl_types,
                decl_vars)

        alist = ['END LOOP;']
        if self.input:
            # pre.append('  EXIT WHEN %s >= %d;' % (idx_name, TABLE_SIZE - 1))
            pre.append('  %s := %s.NEXT(%s);' % (idx_name,
                first_var_name, idx_name))
            # pre.append("  DB_log.esemeny('%s='||%s);" % (idx_name, idx_name))
            pre.extend(alist)
        if self.output:
            # post.append('  EXIT WHEN %s >= %d;' % (idx_name, TABLE_SIZE - 1))
            post.append('  %s := %s.NEXT(%s);' % (idx_name, var_name,
                idx_name))
            # post.append("  DB_log.esemeny('%s='||%s);" % (idx_name,
            #     idx_name))
            post.extend(alist)

    def _ca_hlp_chld_loop(self, pre, post, end, var_name, idx_name, post_pos,
            inner_types, decl_types, decl_vars):
        for child in self._children[0].children:
            inner_var = self._var[0][child.name]
            cdt = (child.abstype if child.abstype != 'BINARY_INTEGER'
                    else 'NUMBER(10)')
            for x in list('(), '):
                cdt = cdt.replace(x, '_')
            # cdt = cdt.replace('__', '_').strip('_')
            cdt = cdt.strip('_')
            if cdt not in inner_types:
                inner_types[cdt] = (cdt + '_tab_typ')
                _numtyp = (child.abstype
                        if child.abstype != 'BINARY_INTEGER'
                        else 'NUMBER(10)')
                decl_types.add('TYPE %s IS TABLE OF %s'
                        ' INDEX BY BINARY_INTEGER;'
                        % (inner_types[cdt], _numtyp))
            #
            if self.input:
                decl_vars.append('%s %s := :%s;' % (inner_var,
                    inner_types[cdt], inner_var))
                pre.append('  %s(%s).%s := %s(%s);' % (var_name,
                    idx_name, child.name, inner_var, idx_name))
            if self.output:
                if self.justoutput:
                    decl_vars.append('%s %s;' % (inner_var,
                        inner_types[cdt]))
                post.append('  %s(%s) := %s(%s).%s;' % (inner_var,
                    idx_name, var_name, idx_name, child.name))
                post.insert(post_pos, '%s.DELETE;' % inner_var)
                # end.append("DB_log.esemeny(':%s<-'||%s.COUNT);"
                    # % (inner_var, inner_var))
                end.append(':%s := %s;' % (inner_var, inner_var))
        #

    @classmethod
    def call_args(cls, args, package_name=None):
        decl_types = set()
        decl_vars = []
        pre = []
        call_args = []
        post = []
        for dt, dv, pr, va, po in (arg.call_arg(package_name=package_name)
                for arg in args):
            if dt:
                decl_types |= dt
            if dv:
                decl_vars.extend(dv)
            if pr:
                pre.append(pr)
            if va:
                call_args.append(va)
            if po:
                post.append(po)
        #
        return (decl_types, decl_vars, pre, call_args, post)

    def _param_typ(self, cur, typ, val):
        oratyp = typ
        typ = OraType.revert(oratyp)
        if self.output:
            if typ == cx_Oracle.CURSOR:
                res = cur.connection.cursor()
            elif isinstance(val, (tuple, list)) or hasattr(val, 'next'):
                res = cur.arrayvar(typ, val)
            else:
                res = cur.var(typ)
                if val is not None:
                    if not isinstance(val, typ):
                        if typ in (int, float):
                            val = typ(val)
                        elif typ == cx_Oracle.STRING:
                            val = str(self.db_encode(val))
                    try:
                        res.setvalue(0, val)
                    except TypeError:
                        LOG.warning('bad type setting %r <%s> on %s [%s]',
                            val, typ, cur, self)
                        if callable(typ) and not isinstance(val, typ):
                            try:
                                val2 = typ(val)
                                res.setvalue(0, val2)
                            except TypeError:
                                LOG.exception('bad type setting (even after'
                                        ' conversion) %r (%s) on %s [%s]',
                                    val, typ, cur, self)
                                raise
        else:
            res = val
        #
        return res

    def getvalue(self, delete=True):
        u'''visszaadja a paraméterben lévő értéket'''
        S = lambda z: (
                z.rstrip() if isinstance(z, basestring) and z.endswith(' ')
                else z)
        if self.genre in (self.TYP_RECORD, self.TYP_TOR):
            #visszaalakítás
            #LOG.debug('prefix: %s, params: %s', self.var_prefix, self._param)
            #pref_len = len(self.var_prefix) + 1
            res = self._getvalue_hlp_torles(self._getvalue_hlp_vissza(S))
        else:
            res = S(self._param.getvalue() if hasattr(self._param, 'getvalue')
                            else self._param)
        #
        if delete:
            self._param = None
        return res

    def _getvalue_hlp_vissza(self, stripval):
        if self.genre == self.TYP_RECORD:
            res = dict((k.rsplit(SEP, 1)[-1], stripval(v.getvalue()))
                for k, v in self._param.iteritems())
        else:
            #print 'params:', self._param
            res = []
            for k, v in self._param.iteritems():
                val = v.getvalue()
                if not res:
                    for i in xrange(len(val)):
                        res.append({})
                for i in xrange(0, len(val)):
                    res[i][k.rsplit(SEP, 1)[-1]] = stripval(val[i])
        return res

    def _getvalue_hlp_torles(self, alist):
        # töröljük a tömb üres elemeit (a végéről)
        if self.genre == self.TYP_TOR:
            n = len(alist)
            for i in xrange(len(alist) - 1, -1, -1):
                elt = alist[i]
                if not (elt is None or (isinstance(elt, dict)
                    and (not elt
                        or all(not x for x in elt.itervalues())))):
                    # már nem üres
                    n = i + 1  # utolsó üres
                    break
            #
            if n < len(alist) - 1:
                del alist[n:]
        return alist

    def setvalue(self, value, cur=None):
        '''eltárolja a megadott értéket átadásra és létrehozza
        a paramétert'''
        #print 'SETVALUE', self.name, repr(value)
        typ = None
        if self.input and value is not None:
            typ = self._typ
            self._value = self._prepare_value(typ, value)
        else:
            self._value = None

        oratyp = typ
        typ = OraType.revert(oratyp)
        #print '->', repr(self._value)
        # paraméter előállítása
        assert cur is None or isinstance(cur, cx_Oracle.Cursor)
        if self.genre == self.TYP_SIMPLE:
            assert hasattr(self, '_typ'), '%s has no _typ!' % self
            res = self._param_typ(cur, self._typ, self._value)
        elif self.genre == self.TYP_RECORD:
            #prefix = self.var_prefix + '_'
            res = dict(
                    (self._var[k],
                        self._param_typ(cur, typ, self._value.get(k)
                            if self._value else None))
                    for k, typ in self._typ.iteritems())
        elif self.genre == self.TYP_TOS:
            LOG.debug('VAL@%s: %s [%s]', self.var_name, self._value,
                    bool(self._value))
            #print 'VAL', self._value, bool(self._value)
            if isinstance(self._value, dict):
                self._value = self._value.values()
            res = cur.arrayvar(OraType.revert(self._typ[0]),
                    list(self._value) if self._value else TABLE_SIZE)
        elif self.genre == self.TYP_TOR:
            res = self._setvalue_hlp_tor(cur)
        #
        self._param = res
        return self._param

    def _setvalue_hlp_tor(self, cur):
        #prefix = self.var_prefix + '_'
        res = None
        keys = self._typ[0].keys()
        vals = dict((k,
            ([self._value[i].get(k) for i in xrange(0, len(self._value))]
                if self._value else TABLE_SIZE))
            for k in self._typ[0].iterkeys())
        del keys
        if self.input and self.output:
            for k, v in vals.iteritems():
                if isinstance(v, list):
                    if len(v) < TABLE_SIZE:
                        v.extend([None] * (TABLE_SIZE - len(v)))
                    else:
                        LOG.warn('TRIM? len(%s)=%d!', k, len(v))
                        # vals[k] = v[:TABLE_SIZE]
        try:
            res = {}
            for k, typ in self._typ[0].iteritems():
                res[self._var[0][k]] = cur.arrayvar(
                    OraType.revert(typ), vals[k])
        except:
            #import traceback
            #traceback.print_exc()
            txt = []
            txt.append('vals: %s' % vals)
            for k, typ in self._typ[0].iteritems():
                txt.append('  k:%s cur.arrayvar(%s, %s)'
                        % (repr(k), repr(typ), repr(vals[k])))
            LOG.exception('\n'.join(txt))
            raise
        return res

    def _prepare_value(self, typ, value):
        '''(rekurzívan) megfésüli a megadott értéket az adott típus
        szerint'''
        if value is not None:
            if isinstance(value, basestring) and value.lower() in ('', 'n/a'):
                value = None
            elif self.db_encode and isinstance(value, unicode):
                value = self.db_encode(value)
        #
        if value is not None:
            oratyp = typ
            typ = OraType.revert(typ)
            argname = self.name
            if oratyp != typ and oratyp.obj:
                argname += '.' + oratyp.obj.name

            if isinstance(typ, (dict, list, tuple)):
                value = self._pv_hlp_collection(value, typ, argname)
            else:
                #max_len = self.char_length
                max_len = (oratyp.char_length if isinstance(oratyp, OraType)
                        else self.char_length)
                value = self._pv_hlp_simple(value, typ, argname, max_len)
        #
        return value

    def _pv_hlp_collection(self, value, typ, argname):
        if isinstance(typ, dict):
            if not isinstance(value, dict):
                raise TypeError('%s value %s should be a dict (struct)'
                        % (argname, repr(value)))
            for k, v in value.iteritems():
                if k in typ:
                    value[k] = self._prepare_value(typ[k], v)
        else:
            if not isinstance(value, (list, tuple)):
                raise TypeError('%s value %s should be a list (array)'
                        % (argname, repr(value)))
            # prepare and filter out empty dicts
            value = tuple(val
                    for val in (self._prepare_value(typ[0], value[i])
                        for i in xrange(len(value)))
                    if (val is not None
                        and (not isinstance(val, dict)
                            or not all(v is None
                                or (hasattr(v, 'getvalue')
                                and v.getvalue() is None)
                                for v in val.itervalues()))))
                            # NOT every element is None
        return value

    def _pv_hlp_simple(self, value, typ, argname, max_len):
        if (typ != cx_Oracle.STRING and value is None or '' == value):
            value = None
        elif typ in (datetime, cx_Oracle.DATETIME,
                cx_Oracle.TIMESTAMP):
            value = self._pv_hlp_date(value, argname)
        elif (typ == cx_Oracle.STRING and isinstance(value, dict)
                and len(value) <= 1):
            value = value.values()[0] if value else None
        elif (max_len > 0 and isinstance(value, basestring)
                and len(value) > max_len):
            raise BadInput('%s is too long (%s)'
                    % (argname, repr(value)))
        elif (typ == cx_Oracle.NUMBER
                and isinstance(value, basestring)):
            try:
                value = value.replace(' ', '')
                value = float(value) if '.' in value else int(value)
            except Exception:
                raise BadInput('%s is not a number (%s)'
                        % (argname, repr(value)))
        return value

    def _pv_hlp_date(self, value, argname):
            dt = value
            #print 'DATE <-', repr(value),
            for length, fmt in (
                    (17, '%Y%m%dT%H:%M:%S'),
                    (19, '%Y-%m-%d %H:%M:%S'),
                    (10, '%Y-%m-%d'), (7, '%Y-%m'), (4, '%Y')):
                try:
                    dt = datetime.strptime(str(value)[:length], fmt)
                    break
                except ValueError:
                    pass
            value = dt
            if not isinstance(value, datetime):
                raise BadInput('%s: %s is not a date'
                        % (argname, repr(value)))
            #print '->', repr(value)
            return value

    @classmethod
    def process_arguments(cls, function, arguments, default_name='ret'):
        u'''feldolgozza az argumentumok listáját típusokká'''

        def inst_factory(function, **kwds):
            name = kwds['name']
            del kwds['name']
            return cls(function, name, **kwds)

        return Argument.process_arguments(function, arguments,
                default_name=default_name, instance_factory=inst_factory)

##
## OracleFunction
##


class OracleFunction(Function):
    u'''egy eljárás'''
    DUMP_ARGS_ARG = 'p_dump_args#'

    def __init__(self, package, func_name, args, retval, documentation=None):
        try:
            Function.__init__(self, package, func_name, args, retval,
                documentation=documentation)
        except:
            LOG.exception('Function(%s, %s, args=%r, retval=%r, %r)',
                    package.name, func_name, args, retval, documentation)
            raise
        assert self.name
        try:
            self.args = OracleArgument.process_arguments(self, args,
                default_name='rec')
        except:
            LOG.exception('OracleFunction(%s, %s, %r, %r, %r)', package.name,
                    func_name, args, retval, documentation)
            raise
        if retval:
            retval['name'] = 'ret'
            try:
                self.retval = OracleArgument(self, **retval)
            except:
                LOG.exception('OracleArgument(%s, %r)', self.name, retval)
                raise
        else:
            self.retval = None
        #self._cache = {}
        self._dumped_args_arg = (self.DUMP_ARGS_ARG
                if any(arg.name == self.DUMP_ARGS_ARG for arg in self.args)
                else None)

    def _ensure_name(self):
        if not hasattr(self, 'name') and hasattr(self, '__name__'):
            self.name = self.__name__

    @staticmethod
    def dump_args(args):
        return json_dumps(args)

    def call_compose(self, arg_names):
        '''composes the query (and caches it)'''
        self._ensure_name()
        arg_names = tuple(sorted(arg_names))
        if len(self._cache) > 10:
            self._cache.clear()

        if not self._cache.get(arg_names):
            sql = []
            args = {}
            funcdef = {'args': self.args, 'retval': self.retval}
            #print funcdef, arg_names
            func_args_all = [self.retval] + self.args
            #print '_call_compose', self.name, arg_names
            eff_args = [arg for arg in func_args_all
                    if arg and (arg.output or arg.name in set(arg_names))]
            tup = (decl_types, decl_vars, pre, args, post) \
                    = OracleArgument.call_args(eff_args,
                            package_name=self._pkg.name)
            vals = dict((arg.name, arg) for arg in eff_args)
            #print 'OracleFunctions(%s).call_compose' % self.name, tup, vals
            del tup
            if decl_types or decl_vars:
                sql.extend(['DECLARE\n', '\n  '.join(decl_types), '\n',
                    '\n  '.join(decl_vars), '\n'])

            sql.append('BEGIN\n')
            if pre:
                sql.extend(['\n', '\n  '.join(pre), '\n'])
            if funcdef['retval']:  # function
                sql.append('  :ret := ')
                vals['ret'] = funcdef['retval']
            sql.append('%s.%s(' % (self._pkg.name, self.name))
            sql.extend([', '.join(x for x in args if not ':=' in x), ');\n'])
            if post:
                sql.extend(['\n', '\n  '.join(post), '\n'])
            # sql.append("\n  DB_log.esemeny('VEGE');")
            sql.append('\nEND;')
            self._cache[arg_names] = (''.join(sql), vals)
            #print self._cache[arg_names][0]
            ##print cache
            if hasattr(self._cache, 'sync'):
                self._cache.sync()
        #
        return self._cache[arg_names]

    def call_prepare(self, conn, args):
        '''prepares the query (params)'''
        self._ensure_name()
        inp_args = set(x.name for x in self.args)
        inp_req_args = set(x.name for x in self.args
                if x.input and not x.has_default)
        if hasattr(args, 'iterkeys'):
            akeys = set(args.iterkeys())
            args = dict(itertools.chain(
                ((k.lower(), v) for k, v in args.iteritems() if k in inp_args),
                ((k, None) for k in inp_req_args - akeys)))
        else:
            argdefs = self.args
            if not isinstance(args, (list, tuple)):
                args = [args]
            rng = range(1 if self.retval else 0, len(args))
            akeys = set(argdefs[i].name for i in rng)
            args = dict(itertools.chain(
                ((argdefs[i].name, args[i]) for i in rng),
                ((k, None) for k in inp_req_args - akeys)))

        if self._dumped_args_arg:
            args_c = args.copy()
            for k in (self._dumped_args_arg, 'p_bazon', 'p_sessionid'):
                if k in args_c:
                    del args_c[k]
            args[self._dumped_args_arg] = self.dump_args(args_c)
            del args_c

        sql, vals = self.call_compose(args.iterkeys())
        params = {}
        #keys = []
        cur = conn.cursor()
        for key, arg in vals.iteritems():
            param = arg.setvalue(args.get(key), cur=cur)
            if isinstance(param, dict):
                params.update(param)
            else:
                params[key] = param
        keys = vals.itervalues()

        #print 'PARAM_KEYS_LEN', max(len(x) for x in params)
        vars = [m.group(1) for m in re.finditer(':([a-z0-9_#]+)', sql)]
        vars_s = set(vars)
        params_s = set(params)
        if vars_s != params_s:
            msg = 'DIFF p-v: %s, v-p: %s\n%s' % (sorted(params_s - vars_s),
                sorted(vars_s - params_s), sql)
            LOG.error(msg)
            assert vars_s == params_s, 'params != vars %s' % msg

        return (sql, params, keys)

    def setAction(self, cur, action):
        cur.connection.setInfo(
                action='%s.%s: %s' % (self._pkg.name, self.name, action))

    r_var_x = re.compile(':([a-z][a-z0-9#_]*)', re.I)

    def test_sql(self, sql, params):
        def to_str(obj):
            if isinstance(obj, (tuple, list)):
                return type(obj)(to_str(x) for x in obj)
            elif isinstance(obj, datetime):
                return "TO_DATE('%s', 'YYYY-MM-DD HH24:MI:SS')" % (
                    obj.strftime('%Y-%m-%d %H:%M:%S'),)
            elif isinstance(obj, (int, float)):
                return str(obj)
            elif obj is None:
                return "''"
            else:
                return "'%s'" % obj

        def repl(m):
            k = m.group(1)
            if k in params:
                v = params[k]
                return str(to_str(v.getvalue() if hasattr(v, 'getvalue')
                    else v))
            else:
                return m.group(0)
        return self.r_var_x.sub(repl, sql)

    def call_execute(self, cur, args):
        '''executes the given function with the given parameters'''
        #print 'call_execute1', self, cur, repr(args)
        self.setAction(cur, 'started')
        if (isinstance(args, (tuple, list)) and 1 == len(args)
                and isinstance(args[0], dict)):
            args = args[0]
        try:
            sql, params, keys = self.call_prepare(cur.connection, args)
            #LOG.debug('%s.call_execute(%s): (%s, %s, %s)'
            #          % (self.name, args, sql, params, keys))
        except:
            LOG.exception('call_prepare failed, args: %s', pformat(repr(args)))
            raise
        result = {}
        try:
            if DEBUG > 1:
                LOG.debug('%s.execute(%s, %s)', self, sql,
                        filter_passwd(params.copy()))
                if DEBUG > 2:
                    LOG.debug('%s.execute(%s)', self,
                            self.test_sql(sql, params))
            try:
                ret = cur.execute(sql, params)
            except cx_Oracle.DatabaseError, dbe:
                if dbe.message.startswith('ORA-04068:'):
                    ret = cur.execute(sql, params)
                else:
                    raise
            self.setAction(cur, 'retrieving results')
            if self.is_function:  # FUNC
                result['ret'] = ret
            outargs = list(arg for arg in keys if arg.output)
            result.update(dict((arg.name, (arg.getvalue(), arg))
                for arg in outargs))
            if 'ret' in result:
                ret = result['ret']
                if (isinstance(ret, tuple) and len(ret) == 2
                        and not isinstance(ret[0], GeneratorType)
                        and not 'Cursor' in str(type(ret[0]))):
                    result[self.name] = result['ret']
        except cx_Oracle.DatabaseError, dbe:
            LOG.exception('%s.call_execute(%s, %s): %s\n%s', self.name, cur,
                    pformat(args), sql, dbe)
            raise dbe
        #
        if not isinstance(args, (list, tuple)):
            args = [args]
        LOG.info('%s(%s) result: \n%s'
                % (self.name, map(filter_passwd, args), pformat((result,))))
        self.setAction(cur, 'done')
        return result

    def execute(self, *args, **kwds):
        assert len(args) == 0 or (len(args) == 1 and isinstance(args[0], dict))
        func_args = args[0] if args else {}
        func_args.update(kwds)
        return self._pkg.connector.runWithCursor(self.call_execute, func_args)

##
## Helper functions
##
from documentor import Documentor, doc_get_paramlist


def plus_info_from_source(desc, source, pkg_name):
    assert source, source
    for (func_name, args), adict in Documentor(source).functions.iteritems():
        for fnm in (func_name, func_name.lower()):
            if fnm in desc:
                descfun = desc[fnm]
                break
        else:
            LOG.warning('WARNING: function %s not found!\nDESC: %s',
                    func_name, desc)
            continue
        args_d = dict((arg.name, arg) for arg in descfun.args)

        args_d = _pifs_hlp(pkg_name, adict, args_d)

        if descfun.retval:
            args_d['ret'] = descfun.retval

        args_d = _pifs_hlp_comment(func_name, adict, args_d, descfun)
    return desc


def _pifs_hlp(pkg_name, adict, args_d):
    for arg_name, argdict in adict['args'].iteritems():
        arg_name = arg_name.lower()
        if not arg_name in args_d:
            continue
        assert not ' ' in arg_name, \
                'BAD ARG NAME %s in pkg %s' % (arg_name, pkg_name)
        if not args_d[arg_name].type_name:
            arg_ori_typ = argdict['ori_typ']
            args_d[arg_name].type_name = (arg_ori_typ
            if (args_d[arg_name].genre == OracleArgument.TYP_SIMPLE
                    or '.' in arg_ori_typ
                    or '%' in arg_ori_typ)
            else pkg_name + '.' + arg_ori_typ)
        args_d[arg_name].has_default = argdict['has_default']
    return args_d


def _pifs_hlp_comment(func_name, adict, args_d, descfun):
    if adict.get('comment', None):
        txt = adict['comment']
        if '/' '*' in txt and '*' '/' in txt:
            p = txt.find('/' '*') + 2
            txt = txt[p:txt.find('*' '/', p)]
        adict['comment'] = descfun.__doc__ = txt.strip()
    if adict['comment']:
        for k, v in doc_get_paramlist(adict['comment'].rstrip(' '))\
                .iteritems():
            if k not in args_d:
                LOG.warning('WARNING: param %s not found for %s!',
                            k, func_name)
            else:
                args_d[k].children = OracleArgument.process_arguments(
                        args_d[k]._fun, v)
    return args_d

__all__ = ['OracleArgument', 'OracleFunction', 'plus_info_from_source']
*/
