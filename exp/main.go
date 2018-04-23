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
	return (&ObjPrinter{}).Print(os.Stdout, ot)
}

type ObjPrinter struct {
	Prefix  string
	printed map[string]struct{}
}

func (p *ObjPrinter) Print(w io.Writer, ot goracle.ObjectType) error {
	if p.printed == nil {
		p.printed = make(map[string]struct{})
	}
	if _, seen := p.printed[ot.FullName()]; seen {
		return nil
	}
	p.printed[ot.FullName()] = struct{}{}
	// FIXME: print protobuf syntax
	if ot.Name == "" {
		return errors.Errorf("EMPTY %+v", ot)
	}
	if ot.Attributes == nil && ot.NativeTypeNum != 3009 {
		_, err := fmt.Fprintf(w, p.Prefix+"type %s %s\n", ot.Name, nativeToGo(int(ot.NativeTypeNum)))
		return err
	}
	fmt.Fprintf(w, p.Prefix+"type %s struct {\n", ot.Name)
	var later []goracle.ObjectType
	for _, attr := range ot.Attributes {
		prefix := ""
		ot := attr.ObjectType
		if attr.ObjectType.CollectionOf != nil {
			prefix = "[]"
			ot = *attr.ObjectType.CollectionOf
		}
		if attr.ObjectType.Attributes == nil {
			typ := nativeToGo(int(ot.NativeTypeNum))
			_, err := fmt.Fprintf(w, p.Prefix+"\t%s %s%s,\n", attr.Name, prefix, typ)
			if err != nil {
				return err
			}
			continue
		}
		_, err := fmt.Fprintf(w, p.Prefix+"\t%s %s%s,\n", attr.Name, prefix, ot.Name)
		if err != nil {
			return err
		}
		later = append(later, ot)
	}
	_, err := fmt.Fprintf(w, p.Prefix+"}\n")
	if err != nil {
		return err
	}
	for _, ot := range later {
		if err := p.Print(w, ot); err != nil {
			return errors.Wrap(err, ot.FullName())
		}
	}
	return nil
}

func nativeToGo(nativeTypeNum int) string {
	switch nativeTypeNum {
	case 3005:
		return "time.Time"
	default:
		return "string"
	}
}
