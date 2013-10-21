package structs

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

// ReadCsv reads the given csv file as user_arguments
// The csv should be an export of
/*
   SELECT object_id, subprogram_id, package_name, object_name,
          data_level, position, argument_name, in_out,
          data_type, data_precision, data_scale, character_set_name,
          pls_type, char_length, type_owner, type_name, type_subname, type_link
     FROM user_arguments
     ORDER BY object_id, subprogram_id, SEQUENCE;
*/
func ReadCsv(filename string) (functions []Function, err error) {
	var fh *os.File
	if filename == "" || filename == "-" {
		fh = os.Stdin
	} else {
		fh, err = os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("cannot open %q: %s", filename, err)
		}
	}
	defer fh.Close()
	br := bufio.NewReader(fh)
	r := csv.NewReader(br)
	b, err := br.Peek(100)
	if err != nil {
		return nil, fmt.Errorf("error peeking into file: %s", err)
	}
	if bytes.IndexByte(b, ';') >= 0 {
		r.Comma = ';'
	}
	r.LazyQuotes, r.TrailingComma, r.TrimLeadingSpace = true, true, true
	var (
		rec       []string
		csvFields = make(map[string]int, 20)
	)
	for _, h := range []string{"OBJECT_ID", "SUBPROGRAM_ID", "PACKAGE_NAME",
		"OBJECT_NAME", "DATA_LEVEL", "POSITION", "ARGUMENT_NAME", "IN_OUT",
		"DATA_TYPE", "DATA_PRECISION", "DATA_SCALE", "CHARACTER_SET_NAME",
		"PLS_TYPE", "CHAR_LENGTH",
		"TYPE_LINK", "TYPE_OWNER", "TYPE_NAME", "TYPE_SUBNAME"} {
		csvFields[h] = -1
	}
	// get head
	if rec, err = r.Read(); err != nil {
		return nil, fmt.Errorf("cannot read head: %s", err)
	}
	var (
		level        uint8
		j            int
		ok           bool
		funName      string
		fun, nameFun Function
		args         = make([]Argument, 0, 16)
		lastArgs     = make([]*Argument, 0, 3)
		row          int
	)
	for i, h := range rec {
		h = strings.ToUpper(h)
		j, ok = csvFields[h]
		//glog.Infof("%d. %q", i, h)
		if ok && j < 0 {
			csvFields[h] = i
		}
	}
	functions = make([]Function, 0, 8)
	glog.Infof("field order: %v", csvFields)
	for {
		if rec, err = r.Read(); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		row++
		if fun.name != "" {
			nameFun.Package, nameFun.name = rec[csvFields["PACKAGE_NAME"]], rec[csvFields["OBJECT_NAME"]]
			funName = nameFun.Name()
		}
		if fun.name == "" || fun.Name() != funName { //new (differs from prev record
			glog.V(1).Infof("New function %s", funName)
			if fun.name != "" {
				x := fun // copy
				x.Args = append(make([]Argument, 0, len(args)), args...)
				//glog.V(1).Infof("old fun: %s", x)
				functions = append(functions, x)
			}
			fun = Function{Package: rec[csvFields["PACKAGE_NAME"]],
				name: rec[csvFields["OBJECT_NAME"]]}
			args = args[:0]
			lastArgs = lastArgs[:0]
		}
		level = mustBeUint8(rec[csvFields["DATA_LEVEL"]])
		arg := NewArgument(rec[csvFields["ARGUMENT_NAME"]],
			rec[csvFields["DATA_TYPE"]],
			rec[csvFields["PLS_TYPE"]],
			rec[csvFields["TYPE_OWNER"]]+"."+
				rec[csvFields["TYPE_NAME"]]+"."+
				rec[csvFields["TYPE_SUBNAME"]]+"@"+
				rec[csvFields["TYPE_LINK"]],
			rec[csvFields["IN_OUT"]],
			0,
			rec[csvFields["CHARACTER_SET_NAME"]],
			mustBeUint8(rec[csvFields["DATA_PRECISION"]]),
			mustBeUint8(rec[csvFields["DATA_SCALE"]]),
			mustBeUint(rec[csvFields["CHAR_LENGTH"]]),
		)
		// Possibilities:
		// 1. SIMPLE
		// 2. RECORD at level 0
		// 3. TABLE OF simple
		// 4. TABLE OF as level 0, RECORD as level 1 (without name), simple at level 2
		if level == 0 {
			args = append(args, arg)
		} else {
			glog.V(2).Infof("row %d: level=%d fun.Args: %s", row, level, fun.Args)
			lastArgs = lastArgs[:level]
			lastArg := lastArgs[level-1]
			if lastArg.Flavor == FLAVOR_RECORD {
				if lastArg.RecordOf == nil {
					lastArg.RecordOf = make(map[string]Argument, 1)
				}
				lastArg.RecordOf[arg.Name] = arg
			} else {
				lastArg.TableOf = append(lastArg.TableOf, arg)
			}
			for i := 0; i < int(level)-1; i++ {
				if lastArgs[i+1].Flavor == FLAVOR_RECORD {
					glog.V(1).Infof("setting %v.RecordOf[%q] to %v",
						lastArgs[i], lastArgs[i+1].Name, lastArgs[i+1])
					if lastArgs[i].RecordOf == nil {
						lastArgs[i].RecordOf = make(map[string]Argument, 1)
					}
					lastArgs[i].RecordOf[lastArgs[i+1].Name] = *lastArgs[i+1]
				} else {
					glog.V(1).Infof("setting %v.TableOf[%d] to %v",
						lastArgs[i], len(lastArgs[i].TableOf)-1, lastArgs[i+1])
					lastArgs[i].TableOf[len(lastArgs[i].TableOf)-1] = *lastArgs[i+1]
				}
			}
		}
		if arg.Flavor != FLAVOR_SIMPLE {
			lastArgs = append(lastArgs[:level], &arg)
		}
		if lastArgs != nil && len(lastArgs) > 0 {
			glog.V(2).Infof("lastArg: %q tof=%#v", lastArgs, lastArgs[len(lastArgs)-1].TableOf)
		}
		//glog.V(2).Infof("last arg: %q tof=%#v", args[len(args)-1], args[len(args)-1].TableOf)
		//glog.Infof("arg=%#v", arg)
	}
	if fun.name != "" {
		fun.Args = args
		functions = append(functions, fun)
	}
	glog.V(1).Infof("functions=%s", functions)
	return
}

func mustBeUint(text string) uint {
	if text == "" {
		return 0
	}
	u, e := strconv.Atoi(text)
	if e != nil {
		panic(e)
	}
	if u < 0 || u > 1<<32 {
		panic(fmt.Sprintf("%d out of range (not uint8)", u))
	}
	return uint(u)
}

func mustBeUint8(text string) uint8 {
	if text == "" {
		return 0
	}
	u, e := strconv.Atoi(text)
	if e != nil {
		panic(e)
	}
	if u < 0 || u > 255 {
		panic(fmt.Sprintf("%d out of range (not uint8)", u))
	}
	return uint8(u)
}
