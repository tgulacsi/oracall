package structs

import (
	"io"
	"log"
	"strings"

	"github.com/golang/glog"
)

func SaveFunctions(dst io.Writer, functions []Function) error {
	var err error
	for _, fun := range functions {
		for _, dir := range []bool{false, true} {
			if err = fun.SaveStruct(dst, dir); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f Function) SaveStruct(dst io.Writer, out bool) error {
	glog.Infof("f=%s", f)
	dirmap, dirname := uint8(DIR_IN), "input"
	if out {
		dirmap, dirname = DIR_OUT, "output"
	}
	var err error
	args := make([]Argument, 0, len(f.Args))
	for _, arg := range f.Args {
		glog.Infof("dir=%d map=%d => %d", arg.Direction, dirmap, arg.Direction&dirmap)
		if arg.Direction&dirmap > 0 {
			args = append(args, arg)
		}
	}
	glog.Infof("args[%d]: %s", dirmap, args)

	if len(args) == 0 { // no args
		return nil
	}

	if _, err = io.WriteString(dst,
		"// "+f.Name()+" "+dirname+"\nstruct "+
			capitalize(f.Package+"__"+f.name+"__"+dirname)+" {\n"); err != nil {
		return err
	}

	for _, arg := range args {
		if _, err = io.WriteString(dst, capitalize(arg.Name)+" "+arg.goType()+",\n"); err != nil {
			return err
		}
	}

	if _, err = io.WriteString(dst, "}\n"); err != nil {
		return err
	}
	return nil
}

func capitalize(text string) string {
	return strings.ToUpper(text[:1]) + strings.ToLower(text[1:])
}

func unocap(text string) string {
	i := strings.Index(text, "_")
	if i == 0 {
		return capitalize(text)
	}
	return strings.ToUpper(text[:i]) + "_" + strings.ToLower(text[i+1:])
}

// returns a go type for the argument's type
func (arg Argument) goType() string {
	if arg.Flavor == FLAVOR_SIMPLE {
		switch arg.Type {
		case "CHAR", "VARCHAR2":
			return "string"
		case "NUMBER":
			return "float64"
		case "INTEGER":
			return "int64"
		case "PLS_INTEGER", "BINARY_INTEGER":
			return "int32"
		case "DATE", "DATETIME", "TIME", "TIMESTAMP":
			return "time.Time"
		default:
			log.Fatalf("unknown simple type %s (%s)", arg.Type, arg)
		}
	}
	return arg.TypeName
}
