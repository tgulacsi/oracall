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

// UserArgument represents the required info from the user_arguments view
type UserArgument struct {
	ObjectID     uint `sql:"OBJECT_ID"`
	SubprogramID uint `sql:"SUBPROGRAM_ID"`

	PackageName string `sql:"PACKAGE_NAME"`
	ObjectName  string `sql:"OBJECT_NAME"`

	DataLevel    uint8  `sql:"DATA_LEVEL"`
	Position     uint8  `sql:"POSITION"`
	ArgumentName string `sql:"ARGUMENT_NAME"`
	InOut        string `sql:"IN_OUT"`

	DataType      string `sql:"DATA_TYPE"`
	DataPrecision uint8  `sql:"DATA_PRECISION"`
	DataScale     uint8  `sql:"DATA_SCALE"`

	CharacterSetName string `sql:"CHARACTER_SET_NAME"`
	CharLength       uint   `sql:"CHAR_LENGTH"`

	PlsType     string `sql:"PLS_TYPE"`
	TypeLink    string `sql:"TYPE_LINK"`
	TypeOwner   string `sql:"TYPE_OWNER"`
	TypeName    string `sql:"TYPE_NAME"`
	TypeSubname string `sql:"TYPE_SUBNAME"`
}

// ParseCsv reads the given csv file as user_arguments
// The csv should be an export of
/*
   SELECT object_id, subprogram_id, package_name, object_name,
          data_level, position, argument_name, in_out,
          data_type, data_precision, data_scale, character_set_name,
          pls_type, char_length, type_owner, type_name, type_subname, type_link
     FROM user_arguments
     ORDER BY object_id, subprogram_id, SEQUENCE;
*/
func ParseCsvFile(filename string) (functions []Function, err error) {
	fh, err := OpenCsv(filename)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return ParseCsv(fh)
}

// ParseCsv parses the csv
func ParseCsv(r io.Reader) (functions []Function, err error) {
	userArgs := make(chan UserArgument, 16)
	errch := make(chan error, 2)
	defer close(errch)
	go func() {
		for lastErr := range errch {
			if lastErr != nil && err == nil {
				err = lastErr
			}
		}
	}()
	go func() { errch <- ReadCsv(r, userArgs) }()
	functions, err = ParseArguments(userArgs)
	if err != nil {
		return nil, err
	}
	return
}

// OpenCsv opens the filename
func OpenCsv(filename string) (*os.File, error) {
	if filename == "" || filename == "-" {
		return os.Stdin, nil
	}
	fh, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q: %s", filename, err)
	}
	return fh, nil
}

// MustOpenCsv opens the file, or panics on error
func MustOpenCsv(filename string) *os.File {
	fh, err := OpenCsv(filename)
	if err != nil {
		glog.Fatal(err)
	}
	return fh
}

// ReadCsv reads the csv from the Reader, and sends the arguments to the given channel
func ReadCsv(r io.Reader, userArgs chan<- UserArgument) error {
	defer close(userArgs)

	var err error

	br := bufio.NewReader(r)
	csvr := csv.NewReader(br)
	b, err := br.Peek(100)
	if err != nil {
		return fmt.Errorf("error peeking into file: %s", err)
	}
	if bytes.IndexByte(b, ';') >= 0 {
		csvr.Comma = ';'
	}
	csvr.LazyQuotes, csvr.TrailingComma, csvr.TrimLeadingSpace = true, true, true
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
	if rec, err = csvr.Read(); err != nil {
		return fmt.Errorf("cannot read head: %s", err)
	}
	for i, h := range rec {
		h = strings.ToUpper(h)
		if j, ok := csvFields[h]; ok && j < 0 {
			csvFields[h] = i
		}
	}
	glog.Infof("field order: %v", csvFields)

	for {
		if rec, err = csvr.Read(); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		userArgs <- UserArgument{
			ObjectID:     mustBeUint(rec[csvFields["OBJECT_ID"]]),
			SubprogramID: mustBeUint(rec[csvFields["SUBPROGRAM_ID"]]),

			PackageName: rec[csvFields["PACKAGE_NAME"]],
			ObjectName:  rec[csvFields["OBJECT_NAME"]],

			DataLevel:    mustBeUint8(rec[csvFields["DATA_LEVEL"]]),
			Position:     mustBeUint8(rec[csvFields["POSITION"]]),
			ArgumentName: rec[csvFields["ARGUMENT_NAME"]],
			InOut:        rec[csvFields["IN_OUT"]],

			DataType:      rec[csvFields["DATA_TYPE"]],
			DataPrecision: mustBeUint8(rec[csvFields["DATA_PRECISION"]]),
			DataScale:     mustBeUint8(rec[csvFields["DATA_SCALE"]]),

			CharacterSetName: rec[csvFields["CHARACTER_SET_NAME"]],
			CharLength:       mustBeUint(rec[csvFields["CHAR_LENGTH"]]),

			PlsType:     rec[csvFields["PLS_TYPE"]],
			TypeLink:    rec[csvFields["TYPE_LINK"]],
			TypeOwner:   rec[csvFields["TYPE_OWNER"]],
			TypeName:    rec[csvFields["TYPE_NAME"]],
			TypeSubname: rec[csvFields["TYPE_SUBNAME"]],
		}
	}
	return nil
}

func ParseArguments(userArgs <-chan UserArgument) (functions []Function, err error) {
	var (
		level    uint8
		fun      Function
		args     = make([]Argument, 0, 16)
		lastArgs = make([]*Argument, 0, 3)
		row      int
	)
	functions = make([]Function, 0, 8)
	seen := make(map[string]uint8, 64)
	for ua := range userArgs {
		row++
		funName := ua.ObjectName
		if funName[len(funName)-1] == '#' { //hidden
			continue
		}
		nameFun := Function{Package: ua.PackageName, name: ua.ObjectName}
		funName = nameFun.Name()
		if seen[funName] > 1 {
			glog.Warningf("function %s already seen! skipping...", funName)
			continue
		}
		if fun.Name() == "" || funName != fun.Name() { //new (differs from prev record
			seen[funName]++
			glog.V(1).Infof("old=%s new=%s seen=%d", fun.Name(), funName, seen[funName])
			glog.V(1).Infof("New function %s", funName)
			if fun.name != "" {
				x := fun // copy
				x.Args = append(make([]Argument, 0, len(args)), args...)
				//glog.V(1).Infof("old fun: %s", x)
				functions = append(functions, x)
			}
			if seen[funName] > 1 {
				continue
			}
			fun = Function{Package: ua.PackageName, name: ua.ObjectName}
			args = args[:0]
			lastArgs = lastArgs[:0]
		}
		level = ua.DataLevel
		arg := NewArgument(ua.ArgumentName,
			ua.DataType,
			ua.PlsType,
			ua.TypeOwner+"."+ua.TypeName+"."+ua.TypeSubname+"@"+ua.TypeLink,
			ua.InOut,
			0,
			ua.CharacterSetName,
			ua.DataPrecision,
			ua.DataScale,
			ua.CharLength,
		)
		// Possibilities:
		// 1. SIMPLE
		// 2. RECORD at level 0
		// 3. TABLE OF simple
		// 4. TABLE OF as level 0, RECORD as level 1 (without name), simple at level 2
		if level == 0 {
			if len(args) == 0 && arg.Name == "" {
				arg.Name = "ret"
				fun.Returns = &arg
			} else {
				args = append(args, arg)
			}
		} else {
			glog.V(2).Infof("row %d: level=%d fun.Args: %s", row, level, fun.Args)
			lastArgs = lastArgs[:level]
			lastArg := lastArgs[level-1]
			if lastArg.Flavor == FLAVOR_TABLE {
				lastArg.TableOf = &arg
			} else {
				if lastArg.RecordOf == nil {
					lastArg.RecordOf = make(map[string]Argument, 1)
				}
				lastArg.RecordOf[arg.Name] = arg
			}
			/*
				for i := int(level) - 2; i >= 0; i-- {
						if lastArgs[i].Flavor == FLAVOR_TABLE {
							glog.V(1).Infof("setting %v.TableOf to %v",
								lastArgs[i], lastArgs[i+1])
							lastArgs[i].TableOf = lastArgs[i+1]
						} else {
							glog.V(1).Infof("setting %v.RecordOf[%q] to %v",
								lastArgs[i], lastArgs[i+1].Name, lastArgs[i+1])
							if lastArgs[i].RecordOf == nil {
								lastArgs[i].RecordOf = make(map[string]Argument, 1)
							}
							lastArgs[i].RecordOf[lastArgs[i+1].Name] = *lastArgs[i+1]
						}
				}
			*/
			// copy back to root
			if len(args) > 0 {
				args[len(args)-1] = *lastArgs[0]
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
