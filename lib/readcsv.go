/*
Copyright 2019 Tamás Gulácsi

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
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

// UserArgument represents the required info from the user_arguments view
type UserArgument struct {
	PackageName string `sql:"PACKAGE_NAME"`
	ObjectName  string `sql:"OBJECT_NAME"`

	ArgumentName string `sql:"ARGUMENT_NAME"`
	InOut        string `sql:"IN_OUT"`

	DataType string `sql:"DATA_TYPE"`

	CharacterSetName string `sql:"CHARACTER_SET_NAME"`

	PlsType     string `sql:"PLS_TYPE"`
	TypeLink    string `sql:"TYPE_LINK"`
	TypeOwner   string `sql:"TYPE_OWNER"`
	TypeName    string `sql:"TYPE_NAME"`
	TypeSubname string `sql:"TYPE_SUBNAME"`

	ObjectID     uint `sql:"OBJECT_ID"`
	SubprogramID uint `sql:"SUBPROGRAM_ID"`

	CharLength uint `sql:"CHAR_LENGTH"`

	DataPrecision uint8 `sql:"DATA_PRECISION"`
	DataScale     uint8 `sql:"DATA_SCALE"`
	DataLevel     uint8 `sql:"DATA_LEVEL"`
	Position      uint8 `sql:"POSITION"`
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
func ParseCsvFile(filename string, filter func(string) bool) (functions []Function, err error) {
	fh, err := OpenCsv(filename)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return ParseCsv(fh, filter)
}

// ParseCsv parses the csv
func ParseCsv(r io.Reader, filter func(string) bool) (functions []Function, err error) {
	userArgs := make(chan UserArgument, 16)
	var grp errgroup.Group
	grp.Go(func() error { return ReadCsv(userArgs, r) })
	filteredArgs := make(chan []UserArgument, 16)
	grp.Go(func() error { FilterAndGroup(filteredArgs, userArgs, filter); return nil })
	functions, err = ParseArguments(filteredArgs, filter)
	if err == nil {
		err = grp.Wait()
	}
	return functions, err
}

func FilterAndGroup(filteredArgs chan<- []UserArgument, userArgs <-chan UserArgument, filter func(string) bool) {
	defer close(filteredArgs)
	type program struct {
		ObjectID, SubprogramID  uint
		PackageName, ObjectName string
	}
	var lastProg, zeroProg program
	args := make([]UserArgument, 0, 4)
	for ua := range userArgs {
		if filter != nil && !filter(ua.PackageName+"."+ua.ObjectName) {
			continue
		}
		actProg := program{
			ObjectID: ua.ObjectID, SubprogramID: ua.SubprogramID,
			PackageName: ua.PackageName, ObjectName: ua.ObjectName}
		if lastProg != zeroProg && lastProg != actProg {
			if len(args) != 0 {
				filteredArgs <- args
				args = make([]UserArgument, 0, cap(args))
			}
		}
		args = append(args, ua)
		lastProg = actProg
	}
	if len(args) != 0 {
		filteredArgs <- args
	}
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
		Log("msg", "MustOpenCsv", "file", filename, "error", err)
		panic(errors.Wrap(err, filename))
	}
	return fh
}

// ReadCsv reads the csv from the Reader, and sends the arguments to the given channel.
func ReadCsv(userArgs chan<- UserArgument, r io.Reader) error {
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
	csvr.LazyQuotes, csvr.TrimLeadingSpace = true, true
	csvr.ReuseRecord = true
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
	csvr.FieldsPerRecord = len(rec)
	for i, h := range rec {
		h = strings.ToUpper(h)
		if j, ok := csvFields[h]; ok && j < 0 {
			csvFields[h] = i
		}
	}
	Log("msg", "field order", "fields", csvFields)

	for {
		rec, err = csvr.Read()
		//Log("rec", rec, "err", err)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		arg := UserArgument{
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

		userArgs <- arg
	}
	return err
}

func ParseArguments(userArgs <-chan []UserArgument, filter func(string) bool) (functions []Function, err error) {
	// Split args by functions
	var dumpBuf strings.Builder
	dumpEnc := xml.NewEncoder(&dumpBuf)
	dumpEnc.Indent("", "  ")
	dumpXML := func(v interface{}) string {
		dumpBuf.Reset()
		if err := dumpEnc.Encode(v); err != nil {
			panic(err)
		}
		return dumpBuf.String()
	}
	_ = dumpXML
	var row int
	for uas := range userArgs {
		if ua := uas[0]; ua.ObjectName[len(ua.ObjectName)-1] == '#' || //hidden
			filter != nil && !filter(ua.ObjectName) {
			continue
		}

		var fun Function
		lastArgs := make(map[int8]*Argument, 8)
		lastArgs[-1] = &Argument{Flavor: FLAVOR_RECORD}
		var level int8
		for i, ua := range uas {
			row++
			if i == 0 {
				fun = Function{Package: ua.PackageName, name: ua.ObjectName}
			}

			level = int8(ua.DataLevel)
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
			if level == 0 && fun.Returns == nil && arg.Name == "" {
				arg.Name = "ret"
				fun.Returns = &arg
				continue
			}
			parent := lastArgs[level-1]
			if arg.Flavor != FLAVOR_SIMPLE {
				lastArgs[level] = &arg
			}
			if parent == nil {
				Log("level", level, "lastArgs", lastArgs, "fun", fun)
			}
			if parent.Flavor == FLAVOR_TABLE {
				parent.TableOf = &arg
			} else {
				parent.RecordOf = append(parent.RecordOf, NamedArgument{Name: arg.Name, Argument: &arg})
			}
		}
		fun.Args = make([]Argument, len(lastArgs[-1].RecordOf))
		for i, na := range lastArgs[-1].RecordOf {
			fun.Args[i] = *na.Argument
		}
		functions = append(functions, fun)
	}
	Log("msg", fmt.Sprintf("found %d functions.", len(functions)))
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

func ReplaceFunctions(functions []Function, tbr map[string]string) []Function {
	if len(tbr) == 0 {
		return functions
	}
	funcs := make(map[string]*Function, len(functions))
	for i := range functions {
		f := functions[i]
		funcs[strings.ToLower(f.Name())] = &f
	}
	for k, v := range tbr {
		if v == "" {
			continue
		}
		funcs[strings.ToLower(k)].Replacement = funcs[strings.ToLower(v)]
	}
	// delete replacements
	for k, v := range tbr {
		if v == "" {
			delete(funcs, k)
		}
	}
	functions = functions[:0]
	for _, f := range funcs {
		if f.Replacement != nil {
			Log("replacee", f.Name(), "replacement", f.Replacement.Name())
		}
		functions = append(functions, *f)
	}
	return functions
}
