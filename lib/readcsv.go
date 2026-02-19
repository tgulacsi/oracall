// Copyright 2019, 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"iter"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// UserArgument represents the required info from the user_arguments view
type UserArgument struct {
	PackageName string `sql:"PACKAGE_NAME"`
	ObjectName  string `sql:"OBJECT_NAME"`
	LastDDL     time.Time

	ArgumentName string `sql:"ARGUMENT_NAME"`
	InOut        string `sql:"IN_OUT"`

	DataType string `sql:"DATA_TYPE"`

	CharacterSetName string `sql:"CHARACTER_SET_NAME"`
	IndexBy          string `sql:"INDEX_BY"`

	PlsType     string `sql:"PLS_TYPE"`
	TypeLink    string `sql:"TYPE_LINK"`
	TypeOwner   string `sql:"TYPE_OWNER"`
	TypeName    string `sql:"TYPE_NAME"`
	TypeSubname string `sql:"TYPE_SUBNAME"`

	ObjectID     uint `sql:"OBJECT_ID"`
	SubprogramID uint `sql:"SUBPROGRAM_ID"`

	CharLength uint `sql:"CHAR_LENGTH"`
	Position   uint `sql:"POSITION"`

	DataPrecision uint8 `sql:"DATA_PRECISION"`
	DataScale     uint8 `sql:"DATA_SCALE"`
	DataLevel     uint8 `sql:"DATA_LEVEL"`
}

// ParseCsv reads the given csv file as user_arguments
// The csv should be an export of
/*
   SELECT object_id, subprogram_id, package_name, sequence, object_name,
          data_level, argument_name, in_out,
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
	functions = ParseArguments(filteredArgs, filter)
	return functions, grp.Wait()
}

func filterArgs(userArgs iter.Seq[UserArgument], filter func(string) bool) iter.Seq[UserArgument] {
	return func(yield func(UserArgument) bool) {
		for ua := range userArgs {
			if filter != nil && !filter(ua.PackageName+"."+ua.ObjectName) {
				continue
			}
			if !yield(ua) {
				return
			}
		}
	}
}

func groupArgs(userArgs iter.Seq[UserArgument]) iter.Seq[[]UserArgument] {
	return func(yield func([]UserArgument) bool) {
		type program struct {
			PackageName, ObjectName string
			ObjectID, SubprogramID  uint
		}
		var lastProg, zeroProg program
		args := make([]UserArgument, 0, 4)
		for ua := range userArgs {
			actProg := program{
				ObjectID: ua.ObjectID, SubprogramID: ua.SubprogramID,
				PackageName: ua.PackageName, ObjectName: ua.ObjectName}
			if lastProg != zeroProg && lastProg != actProg {
				if len(args) != 0 {
					if !yield(args) {
						return
					}
					args = make([]UserArgument, 0, cap(args))
				}
			}
			args = append(args, ua)
			lastProg = actProg
		}
		if len(args) != 0 {
			if !yield(args) {
				return
			}
		}
	}
}

func FilterAndGroupIter(userArgs iter.Seq[UserArgument], filter func(string) bool) iter.Seq[[]UserArgument] {
	return groupArgs(filterArgs(userArgs, filter))
}

func FilterAndGroup(filteredArgs chan<- []UserArgument, userArgs <-chan UserArgument, filter func(string) bool) {
	defer close(filteredArgs)
	for uas := range groupArgs(filterArgs(func(yield func(UserArgument) bool) {
		for ua := range userArgs {
			if !yield(ua) {
				return
			}
		}
	}, filter)) {
		filteredArgs <- uas
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
		logger.Error("MustOpenCsv", "file", filename, "error", err)
		panic(fmt.Errorf("%s: %w", filename, err))
	}
	return fh
}

func NewUACsvWriter(w io.Writer) (*UACsvWriter, error) {
	cw := csv.NewWriter(w)
	var ua UserArgument
	rt := reflect.TypeOf(ua)
	colNames := make([]string, 0, rt.NumField())
	converters := make([]func(reflect.Value) string, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		ft := rt.Field(i)
		if !ft.IsExported() {
			continue
		}
		name := ft.Tag.Get("sql")
		if name == "" {
			continue
		}
		colNames = append(colNames, name)
		// fmt.Printf("%d: %q %v\n", i, name, ft.Type.Kind())
		switch ft.Type.Kind() {
		case reflect.Uint, reflect.Uint8:
			converters[i] = func(v reflect.Value) string { return strconv.FormatUint(v.Uint(), 10) }
		default:
			converters[i] = func(v reflect.Value) string { return v.String() }
		}
	}
	if err := cw.Write(colNames); err != nil {
		return nil, err
	}
	return &UACsvWriter{cw: cw, converters: converters}, nil
}

type UACsvWriter struct {
	cw         *csv.Writer
	fields     []string
	converters []func(reflect.Value) string
}

func (cw *UACsvWriter) WriteUA(ua UserArgument) error {
	cw.fields = cw.fields[:0]
	rv := reflect.ValueOf(ua)
	for i, conv := range cw.converters {
		if conv != nil {
			cw.fields = append(cw.fields, conv(rv.Field(i)))
		}
	}
	return cw.cw.Write(cw.fields)
}
func (cw *UACsvWriter) Flush()       { cw.cw.Flush() }
func (cw *UACsvWriter) Error() error { return cw.cw.Error() }

func NewUACsvReader(r io.Reader) iter.Seq2[UserArgument, error] {
	br := bufio.NewReader(r)
	csvr := csv.NewReader(br)
	b, err := br.Peek(100)
	if err != nil {
		return func(yield func(UserArgument, error) bool) {
			yield(UserArgument{}, fmt.Errorf("error peeking into file: %s", err))
		}
	}
	if bytes.IndexByte(b, ';') >= 0 {
		csvr.Comma = ';'
	}
	csvr.LazyQuotes, csvr.TrimLeadingSpace = true, true
	csvr.ReuseRecord = true
	type csvStructIdx struct {
		Conv        func(reflect.Value, string)
		Csv, Struct int
	}
	var (
		rec       []string
		csvFields = make(map[string]csvStructIdx, 20)
	)
	rt := reflect.TypeOf(UserArgument{})
	for i := 0; i < rt.NumField(); i++ {
		ft := rt.Field(i)
		if !ft.IsExported() {
			continue
		}
		name := ft.Tag.Get("sql")
		if name == "" {
			continue
		}
		conv := func(st reflect.Value, s string) {
			st.Field(i).SetString(s)
		}
		switch ft.Type.Kind() {
		case reflect.Uint8:
			conv = func(st reflect.Value, s string) {
				st.Field(i).SetUint(uint64(mustBeUint8(s)))
			}
		case reflect.Uint:
			conv = func(st reflect.Value, s string) {
				st.Field(i).SetUint(uint64(mustBeUint(s)))
			}
		}
		csvFields[name] = csvStructIdx{Csv: -1, Struct: i, Conv: conv}
	}
	// get head
	if rec, err = csvr.Read(); err != nil {
		return func(yield func(UserArgument, error) bool) {
			yield(UserArgument{}, fmt.Errorf("cannot read head: %s", err))
		}
	}
	csvr.FieldsPerRecord = len(rec)
	for i, h := range rec {
		h = strings.ToUpper(h)
		if idx, ok := csvFields[h]; ok && idx.Csv < 0 {
			idx.Csv = i
			csvFields[h] = idx
		}
	}
	logger.Info("field order", "fields", csvFields)

	return func(yield func(UserArgument, error) bool) {
		for pos := uint(0); ; pos++ {
			rec, err = csvr.Read()
			if err != nil {
				yield(UserArgument{}, err)
				return
			}
			ua := UserArgument{Position: pos}
			rv := reflect.ValueOf(&ua).Elem()
			for _, idx := range csvFields {
				if idx.Conv == nil || idx.Csv < 0 {
					continue
				}
				idx.Conv(rv, rec[idx.Csv])
			}
			if !yield(ua, nil) {
				return
			}
		}
	}
}

// ReadCsv reads the csv from the Reader, and sends the arguments to the given channel.
func ReadCsv(userArgs chan<- UserArgument, r io.Reader) error {
	defer close(userArgs)

	for ua, err := range NewUACsvReader(r) {
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		userArgs <- ua
	}
	return nil
}

func ParseArgumentsIter(userArgs iter.Seq[[]UserArgument], filter func(string) bool) []Function {
	// Split args by functions
	var names []string
	var functions []Function
	var row int
	for uas := range userArgs {
		if ua := uas[0]; ua.ObjectName[len(ua.ObjectName)-1] == '#' || //hidden
			filter != nil && !filter(ua.ObjectName) {
			continue
		}

		var fun Function
		lastArgs := make(map[int8]*Argument, 8)
		lastArgs[-1] = &Argument{Type: &Type{Flavor: FLAVOR_RECORD}}
		var level int8
		for i, ua := range uas {
			row++
			if i == 0 {
				fun = Function{Package: ua.PackageName, name: ua.ObjectName, LastDDL: ua.LastDDL}
			}

			level = int8(ua.DataLevel)
			typeName := ua.TypeOwner + "." + ua.TypeName + "." + ua.TypeSubname + "@" + ua.TypeLink
			if ua.TypeSubname == "" && ua.PlsType+"@" == typeName {
				typeName = ua.TypeOwner + "." + ua.TypeName + "%ROWTYPE"
			}
			arg := NewArgument(ua.ArgumentName,
				ua.DataType,
				ua.PlsType,
				typeName,
				ua.InOut,
				0,
				ua.CharacterSetName,
				ua.IndexBy,
				ua.DataPrecision,
				ua.DataScale,
				ua.CharLength,
			)
			logger.Debug("ParseArgument", "level", level, "fun", fun.name, "arg", arg.Name, "type", ua.DataType, "last", lastArgs, "flavor", arg.Flavor, "typeName", typeName, "ua", ua, "arg", arg, "typeSub", ua.TypeSubname, "pls", ua.PlsType)
			// Possibilities:
			// 1. SIMPLE
			// 2. RECORD at level 0
			// 3. TABLE OF simple
			// 4. TABLE OF as level 0, RECORD as level 1 (without name), simple at level 2
			if arg.Flavor != FLAVOR_SIMPLE {
				lastArgs[level] = &arg
			}
			if level == 0 && fun.Returns == nil && arg.Name == "" {
				arg.Name = "ret"
				fun.Returns = arg.Type
				continue
			}
			parent := lastArgs[level-1]
			if parent == nil {
				logger.Info("parent is nil", "level", level, "lastArgs", lastArgs, "fun", fun)
				panic(fmt.Sprintf("parent is nil, at level=%d, lastArgs=%v, fun=%v", level, lastArgs, fun))
			}
			if parent.Flavor == FLAVOR_TABLE {
				parent.TableOf = arg.Type
			} else {
				parent.RecordOf = append(parent.RecordOf, Argument{Name: arg.Name, Type: arg.Type})
			}
		}
		fun.Args = make([]Argument, len(lastArgs[-1].RecordOf))
		copy(fun.Args, lastArgs[-1].RecordOf)
		functions = append(functions, fun)
		names = append(names, fun.Name())
	}
	logger.Info("found", "functions", names)
	return functions
}

func ParseArguments(userArgs <-chan []UserArgument, filter func(string) bool) []Function {
	return ParseArgumentsIter(
		func(yield func(uas []UserArgument) bool) {
			for uas := range userArgs {
				if !yield(uas) {
					return
				}
			}
		}, filter)
}

func mustBeUint(text string) uint {
	if text == "" {
		return 0
	}
	u, e := strconv.ParseUint(text, 10, uintWidthBits)
	if e != nil {
		panic(e)
	}
	return uint(u)
}

func mustBeUint8(text string) uint8 {
	if text == "" {
		return 0
	}
	u, err := strconv.ParseUint(text, 10, 8)
	if err != nil {
		panic(err)
	}
	return uint8(u)
}

type Annotation struct {
	Package, Type, Name, Other string
	Size                       int
}

func (a Annotation) FullName() string {
	if a.Package == "" || a.Name == "" {
		return a.Name
	}
	return a.Package + "." + a.Name
}
func (a Annotation) FullOther() string {
	if a.Package == "" || a.Other == "" {
		return a.Other
	}
	return a.Package + "." + a.Other
}
func (a Annotation) String() string {
	if a.Type == "" || a.Name == "" {
		return ""
	}
	switch a.Type {
	case "private":
		return a.Type + " " + a.FullName()
	case "max-table-size":
		return fmt.Sprintf("%s.MaxTableSize=%d", a.FullName(), a.Size)
	}
	return a.Type + " " + a.FullName() + "=>" + a.FullOther()
}

func ApplyAnnotations(functions []Function, annotations []Annotation) []Function {
	if len(annotations) == 0 {
		return functions
	}
	L := strings.ToLower
	funcs := make(map[string]*Function, len(functions))
	for i := range functions {
		f := functions[i]
		funcs[L(f.RealName())] = &f
	}
	for _, a := range annotations {
		if a.Name == "" || a.Type == "" {
			continue
		}
		if a.Other == "" && !(a.Type == "private" || a.Type == "handle" || a.Type == "max-table-size") {
			continue
		}
		if a.Size <= 0 && a.Type == "max-table-size" {
			continue
		}
		switch a.Type {
		case "private":
			nm := L(a.FullName())
			logger.Info("directive", "private", nm)
			delete(funcs, nm)
		case "rename":
			nm := L(a.FullName())
			if f := funcs[nm]; f != nil {
				delete(funcs, nm)
				funcs[L(a.FullOther())] = f
				logger.Info("directive", "rename", nm, "to", a.Other)
				f.alias = a.Other
			}
		case "replace", "replace_json":
			k, v := L(a.FullName()), L(a.FullOther())
			if f := funcs[k]; f != nil {
				logger.Info("directive", "replace", k, "with", v)
				f.Replacement = funcs[v]
				f.ReplacementIsJSON = a.Type == "replace_json"
				delete(funcs, v)
				logger.Info("directive", "delete", v, "add", f.Name())
				funcs[L(f.Name())] = f
			}

		// add handler to ALL functions in the same package
		case "handle":
			exc := strings.ToUpper(a.Name)
			for _, f := range funcs {
				if strings.EqualFold(f.Package, a.Package) {
					f.handle = append(f.handle, exc)
				}
			}

		case "max-table-size":
			nm := L(a.FullName())
			logger.Info("directive", "max-table-size", nm, "size", a.Size)
			if f := funcs[nm]; f != nil && a.Size >= f.maxTableSize {
				f.maxTableSize = a.Size
			}

		case "tag":
			nm := L(a.FullName())
			if f := funcs[nm]; f != nil {
				f.Tag = append(f.Tag, a.Other)
				logger.Info("directive", "f", nm, "tag", f.Tag)
			} else {
				logger.Warn("no function for tag", "name", nm, "tag", a.Other, "have", funcs)
			}
		}
	}
	functions = functions[:0]
	for _, f := range funcs {
		functions = append(functions, *f)
	}
	for _, f := range functions {
		if len(f.Tag) != 0 {
			logger.Info("ApplyAnnotations", "f", f.name, "tag", f.Tag)
		}
	}
	return functions
}
