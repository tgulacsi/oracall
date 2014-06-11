/*
Copyright 2014 Tamás Gulácsi

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

package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"flag"
	"io"
	"log"
	"os"
	"strings"

	_ "github.com/tgulacsi/goracle/godrv"
)

var flagConnect = flag.String("connect", "", "Oracle database connection string")

func main() {
	flag.Parse()
	if *flagConnect == "" {
		log.Fatalf("connect string is needed")
	}
	conn, err := sql.Open("goracle", *flagConnect)
	if err != nil {
		log.Fatalf("error creating connection to %s: %s", *flagConnect, err)
	}
	defer conn.Close()

	input := os.Stdin
	if flag.NArg() >= 1 {
		if input, err = os.Open(flag.Arg(0)); err != nil {
			log.Fatalf("error opening %s: %v", flag.Arg(0), err)
		}
		defer input.Close()
	}
	inp := bufio.NewReader(input)
	buf := bytes.NewBuffer(make([]byte, 0, 4096))
	for {
		line, isPrefix, err := inp.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("error reading %s: %v", input, err)
		}
		if isPrefix {
			log.Fatalf("line too long (%q)", line)
		}
		sline := string(bytes.TrimSpace(line))
		if sline == "/" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(sline), "CREATE ") {
			if buf.Len() > 0 {
				if err = compile(conn, buf.String()); err != nil {
					log.Printf("compilation error of %q: %v", buf, err)
				}
				buf.Reset()
			}
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if buf.Len() > 0 {
		if err = compile(conn, buf.String()); err != nil {
			log.Printf("compilation error of %q: %v", buf, err)
		}
	}

	if errCheck(conn) > 0 {
		os.Exit(2)
	}
}

type execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func compile(db execer, qry string) error {
	if _, err := db.Exec(qry); err != nil {
		return err
	}
	return nil
}

type querier interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

func errCheck(db querier) int {
	errcount := 0
	rows, err := db.Query("SELECT name, type, line||':'||position, text FROM user_errors ORDER BY sequence")
	if err != nil {
		log.Fatalf("error querying errors: %v", err)
	}
	defer rows.Close()
	var name, typ, pos, text string
	for rows.Next() {
		if err = rows.Scan(&name, &typ, &pos, &text); err != nil {
			log.Printf("error scanning: %v", err)
		}
		log.Printf("ERROR in %s %s at %s: %s", typ, name, pos, text)
		errcount++
	}

	return errcount
}
