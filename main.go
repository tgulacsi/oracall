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

package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/tgulacsi/oracall/structs"
	"gopkg.in/inconshreveable/log15.v2"
	"gopkg.in/rana/ora.v3" // for Oracle-specific drivers
)

var Log = log15.New()

var flagConnect = flag.String("connect", "", "connect to DB for retrieving function arguments")

func main() {
	Log.SetHandler(log15.StderrHandler)
	structs.Log.SetHandler(log15.StderrHandler)

	flagSkipFormat := flag.Bool("F", false, "skip formatting")
	flagVerbose := flag.Bool("v", false, "verbose logging")
	flagDump := flag.String("dump", "", "dump to this csv")

	flag.Parse()
	if !*flagVerbose {
		hndl := log15.LvlFilterHandler(log15.LvlInfo, log15.StderrHandler)
		Log.SetHandler(hndl)
		structs.Log.SetHandler(hndl)
	}

	var functions []structs.Function
	var err error

	if *flagConnect == "" {
		functions, err = structs.ParseCsvFile(flag.Arg(0))
	} else {
		pattern := "%"
		if flag.NArg() >= 1 {
			pattern = flag.Arg(0)
		}
		ora.Register(nil)
		cx, err := sql.Open("ora", *flagConnect)
		if err != nil {
			log.Fatalf("error connecting to %q: %s", *flagConnect, err)
		}
		defer cx.Close()
		if err = cx.Ping(); err != nil {
			log.Fatalf("error pinging %q: %v", *flagConnect, err)
		}
		qry := `
    SELECT object_id, subprogram_id, package_name, object_name,
           data_level, position, argument_name, in_out,
           data_type, data_precision, data_scale, character_set_name,
           pls_type, char_length, type_owner, type_name, type_subname, type_link
      FROM user_arguments
	  WHERE package_name||'.'||object_name LIKE UPPER(:1)
      ORDER BY object_id, subprogram_id, SEQUENCE`
		rows, err := cx.Query(qry, pattern)
		if err != nil {
			log.Fatalf("error querying %q: %s", qry, err)
		}
		defer rows.Close()

		var cw *csv.Writer
		if *flagDump != "" {
			fh, err := os.Create(*flagDump)
			if err != nil {
				log.Fatalf("error creating %q for csv dump: %v", *flagDump, err)
			}
			defer func() {
				cw.Flush()
				if err := cw.Error(); err != nil {
					log.Printf("ERROR flushing csv %q: %v", fh.Name(), err)
				}
				if err := fh.Close(); err != nil {
					log.Printf("ERROR closing dump file %q: %v", fh.Name(), err)
				}
			}()
			cw = csv.NewWriter(fh)
			if err = cw.Write(strings.Split(
				strings.Map(
					func(r rune) rune {
						if 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' || r == '_' || r == ',' {
							return r
						}
						if 'a' <= r && r <= 'z' {
							return unicode.ToUpper(r)
						}
						return -1
					},
					qry[strings.Index(qry, "SELECT ")+7:strings.Index(qry, "FROM ")],
				),
				",",
			)); err != nil {
				log.Fatalf("error writing header to csv: %v", err)
			}
		}

		userArgs := make(chan structs.UserArgument, 16)
		var readErr error
		go func() {
			defer close(userArgs)
			var pn, on, an, cs, plsT, tOwner, tName, tSub, tLink sql.NullString
			var oid, subid, level, pos, prec, scale, length sql.NullInt64
			ua := structs.UserArgument{}
			for rows.Next() {
				err = rows.Scan(&oid, &subid, &pn, &on,
					&level, &pos, &an, &ua.InOut,
					&ua.DataType, &prec, &scale, &cs,
					&plsT, &length, &tOwner, &tName, &tSub, &tLink)
				if err != nil {
					readErr = err
					log.Fatalf("error reading row %v: %v", rows, err)
				}
				if cw != nil {
					N := i64ToString
					if err = cw.Write([]string{
						N(oid), N(subid), pn.String, on.String,
						N(level), N(pos), an.String, ua.InOut,
						ua.DataType, N(prec), N(scale), cs.String,
						plsT.String, N(length),
						tOwner.String, tName.String, tSub.String, tLink.String,
					}); err != nil {
						log.Printf("error writing csv: %v", err)
					}
				}
				ua.PackageName, ua.ObjectName, ua.ArgumentName = "", "", ""
				ua.ObjectID, ua.SubprogramID, ua.DataLevel = 0, 0, 0
				ua.Position, ua.DataPrecision, ua.DataScale, ua.CharLength = 0, 0, 0, 0
				if pn.Valid {
					ua.PackageName = pn.String
				}
				if on.Valid {
					ua.ObjectName = on.String
				}
				if an.Valid {
					ua.ArgumentName = an.String
				}
				if cs.Valid {
					ua.CharacterSetName = cs.String
				}
				if plsT.Valid {
					ua.PlsType = plsT.String
				}
				if tOwner.Valid {
					ua.TypeOwner = tOwner.String
				}
				if tName.Valid {
					ua.TypeName = tName.String
				}
				if tSub.Valid {
					ua.TypeSubname = tSub.String
				}
				if tLink.Valid {
					ua.TypeLink = tLink.String
				}
				if oid.Valid {
					ua.ObjectID = uint(oid.Int64)
				}
				if subid.Valid {
					ua.SubprogramID = uint(subid.Int64)
				}
				if level.Valid {
					ua.DataLevel = uint8(level.Int64)
				}
				if pos.Valid {
					ua.Position = uint8(pos.Int64)
				}
				if prec.Valid {
					ua.DataPrecision = uint8(prec.Int64)
				}
				if scale.Valid {
					ua.DataScale = uint8(scale.Int64)
				}
				if length.Valid {
					ua.CharLength = uint(length.Int64)
				}
				userArgs <- ua
			}
			if err = rows.Err(); err != nil {
				readErr = err
				log.Fatalf("error walking rows: %s", err)
			}
		}()
		functions, err = structs.ParseArguments(userArgs)
	}
	if err != nil {
		log.Fatalf("error reading %q: %s", flag.Arg(0), err)
	}

	defer os.Stdout.Sync()
	if err = structs.SaveFunctions(os.Stdout, functions, "main", *flagSkipFormat); err != nil {
		log.Printf("error saving functions: %s", err)
		os.Exit(1)
	}
}

func i64ToString(n sql.NullInt64) string {
	if n.Valid {
		return strconv.FormatInt(n.Int64, 10)
	}
	return ""
}
