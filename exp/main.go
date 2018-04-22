package main

import (
	"database/sql"
	"flag"
	"log"

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
	log.Printf("ot=%+v error=%v", ot, err)
	if err != nil {
		return err
	}
	log.Printf("attrs=%+v", ot.Attributes)
	for i, attr := range ot.Attributes {
		log.Printf("  %d: %+v", i, attr)
		log.Printf("sub: %+v", attr.ObjectType.Attributes)
	}
	return nil
}
