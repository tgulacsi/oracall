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
func ReadCsv(filename string) error {
	var (
		err error
		fh  *os.File
	)
	if filename == "" || filename == "-" {
		fh = os.Stdin
	} else {
		fh, err = os.Open(filename)
		if err != nil {
			return fmt.Errorf("cannot open %q: %s", filename, err)
		}
	}
	defer fh.Close()
	br := bufio.NewReader(fh)
	r := csv.NewReader(br)
	b, err := br.Peek(100)
	if err != nil {
		return fmt.Errorf("error peeking into file: %s", err)
	}
	if bytes.IndexByte(b, ';') >= 0 {
		r.Comma = ';'
	}
	r.LazyQuotes, r.TrailingComma, r.TrimLeadingSpace = true, true, true
	var (
		arg       Argument
		rec       []string
		csvFields = make(map[string]int, 20)
		numFields = make([]int, 0, 8)
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
		return fmt.Errorf("cannot read head: %s", err)
	}
	var (
		j       int
		ok      bool
		funName string
		fun     *Function
	)
	for i, h := range rec {
		h = strings.ToUpper(h)
		j, ok = csvFields[h]
		//glog.Infof("%d. %q", i, h)
		if ok && j < 0 {
			csvFields[h] = i
		}
	}
	for _, h := range []string{"OBJECT_ID", "SUBPROGRAM_ID", "DATA_LEVEL",
		"POSITION", "DATA_PRECISION", "DATA_SCALE", "CHAR_LENGTH"} {
		numFields = append(numFields, csvFields[h])
	}
	glog.Infof("field order: %v", csvFields)
	for {
		if rec, err = r.Read(); err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		funName = "" + "." + rec[csvFields["PACKAGE_NAME"]] + "." +
			rec[csvFields["OBJECT_NAME"]]
		if fun, ok = Functions[funName]; !ok {
			glog.Infof("New function %s", funName)
			fun = &Function{Owner: "", Package: rec[csvFields["PACKAGE_NAME"]], Name: rec[csvFields["OBJECT_NAME"]]}
			Functions[funName] = fun
		}
		if fun.Args == nil {
			fun.Args = make([]Argument, 0, 1)
		}
		arg = Argument{Level: mustBeUint8(rec[csvFields["DATA_LEVEL"]]),
			Position: mustBeUint8(rec[csvFields["POSITION"]]),
			Name:     rec[csvFields["ARGUMENT_NAME"]],
			Type:     rec[csvFields["DATA_TYPE"]],
			PlsType:  rec[csvFields["PLS_TYPE"]],
			TypeName: rec[csvFields["TYPE_OWNER"]] + "." +
				rec[csvFields["TYPE_NAME"]] + "." +
				rec[csvFields["TYPE_SUBNAME"]] + "@" +
				rec[csvFields["TYPE_LINK"]],
		}
		switch rec[csvFields["IN_OUT"]] {
		case "IN/OUT":
			arg.InOut = 2
		case "OUT":
			arg.InOut = 1
		}
		if arg.TypeName == "..@" {
			arg.TypeName = ""
		}
		fun.Args = append(fun.Args, arg)
		//glog.Infof("arg=%#v", arg)
	}
	glog.Infof("Functions: %#v", Functions)
	return err
}

func mustBeUint8(text string) uint8 {
	u, e := strconv.Atoi(text)
	if e != nil {
		panic(e)
	}
	if u < 0 || u > 255 {
		panic(fmt.Sprintf("%d out of range (not uint8)", u))
	}
	return uint8(u)
}
