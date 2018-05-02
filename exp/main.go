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
	fmt.Fprintf(os.Stdout, "syntax = \"proto3\";\npackage gdpr;\n")
	if err = (&ObjPrinter{}).Print(os.Stdout, ot); err != nil {
		return err
	}

	obj, err := ot.NewObject()
	if err != nil {
		return err
	}
	log.Printf("obj=%+v", obj)
	for i, attr := range obj.Attributes {
		var data goracle.Data
		if err := obj.GetAttribute(&data, i); err != nil {
			return err
		}
		log.Printf("  %d. %+v: %+v", i, attr, data)
		sub := data.GetObject()
		if len(sub.Attributes) == 0 {
			continue
		}
		var sd goracle.Data
		if err = sub.GetAttribute(&sd, 0); err != nil {
			return err
		}
		if sd.NativeTypeNum != 3004 {
			continue
		}
		sd.SetBytes([]byte("aaa"))
		if err = sub.SetAttribute(0, &sd); err != nil {
			return err
		}
		if obj.SetAttribute(0, &sd); err != nil {
			return err
		}

		log.Printf(" !!! %d. %+v", i, attr)
	}

	var str string
	if _, err = db.Exec(`BEGIN :1 := XMLType(:2).getClobVal; END;`,
		sql.Out{Dest: &str}, obj,
	); err != nil {
		return err
	}
	log.Println("str:", str)
	return nil
}

type ObjPrinter struct {
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
		_, err := fmt.Fprintf(w, "message %s { %s item = 1; }\n", ot.Name, nativeToGo(int(ot.NativeTypeNum)))
		return err
	}
	fmt.Fprintf(w, "message %s {\n", ot.Name)
	var later []goracle.ObjectType
	for i, attr := range ot.Attributes {
		prefix := ""
		ot := attr.ObjectType
		if attr.ObjectType.CollectionOf != nil {
			prefix = "repeated "
			ot = *attr.ObjectType.CollectionOf
		}
		if attr.ObjectType.Attributes == nil {
			typ := nativeToGo(int(ot.NativeTypeNum))
			_, err := fmt.Fprintf(w, "\t%s%s %s = %d;\n", prefix, typ, attr.Name, i+1)
			if err != nil {
				return err
			}
			continue
		}
		_, err := fmt.Fprintf(w, "\t%s%s %s = %d;\n", prefix, ot.Name, attr.Name, i+1)
		if err != nil {
			return err
		}
		later = append(later, ot)
	}
	_, err := fmt.Fprintf(w, "}\n")
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
		return "string"
	default:
		return "string"
	}
}
