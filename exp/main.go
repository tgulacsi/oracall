package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/goracle.v2"
)

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}

func Main() error {
	flag.Parse()
	db, err := sql.Open("goracle", flag.Arg(0))
	if err != nil {
		return errors.Wrap(err, flag.Arg(0))
	}
	defer db.Close()

	conn, err := goracle.DriverConn(db)
	if err != nil {
		return err
	}
	defer conn.Close()
	ot, err := conn.GetObjectType(`MMB."GDPRRequest"`)
	//log.Printf("ot=%+v error=%v", ot, err)
	if err != nil {
		return err
	}
	(&ObjPrinter{}).Print(os.Stdout, ot)
	return nil
}

type ObjPrinter struct {
	Prefix  string
	printed map[string]struct{}
}

func (p *ObjPrinter) Print(w io.Writer, ot goracle.ObjectType) {
	if p.printed == nil {
		p.printed = make(map[string]struct{})
	}
	// FIXME: print protobuf syntax
	fmt.Fprintf(w, p.Prefix+"type %s struct {\n", ot.Info.Name())
	later := make(map[string]goracle.ObjectType)
	for _, attr := range ot.Attributes {
		// FIXME: handle Collection types
		if attr.ObjectType.Attributes == nil {
			typ := "string"
			switch attr.NativeTypeNum {
			case 3004:
			case 3005:
				typ = "time.Time"
			}
			fmt.Fprintf(w, p.Prefix+"\t%s %s,\n", attr.Name, typ)
			continue
		}
		fmt.Fprintf(w, p.Prefix+"\t%s %s,\n", attr.Name, attr.ObjectType.Info.Name())
		later[attr.ObjectType.Info.Name()] = attr.ObjectType
	}
	fmt.Fprintf(w, p.Prefix+"}\n")
	for _, ot := range later {
		p.Print(w, ot)
	}
}
