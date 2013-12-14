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
	"flag"
	"log"
	"os"

	_ "github.com/tgulacsi/goracle/godrv" // for Oracle-specific drivers
	"github.com/tgulacsi/oracall/structs"
)

var flagSkipFormat = flag.Bool("F", false, "skip formatting")
var flagConnect = flag.String("connect", "", "connect to DB for retrieving function arguments")

func main() {
	flag.Parse()
	var functions []structs.Function
	var err error

	if *flagConnect == "" {
		functions, err = structs.ParseCsvFile(flag.Arg(0))
	} else {
		pattern := "%"
		if flag.NArg() >= 1 {
			pattern = flag.Arg(0)
		}
		cx, err := sql.Open("goracle", *flagConnect)
		if err != nil {
			log.Fatalf("error connecting to %q: %s", *flagConnect, err)
		}
		defer cx.Close()
		qry := `
    SELECT object_id, subprogram_id, package_name, object_name,
           data_level, position, argument_name, in_out,
           data_type, data_precision, data_scale, character_set_name,
           pls_type, char_length, type_owner, type_name, type_subname, type_link
      FROM user_arguments
      WHERE package_name||'.'||object_name LIKE UPPER(?)
      ORDER BY object_id, subprogram_id, SEQUENCE`
		rows, err := cx.Query(qry, pattern)
		if err != nil {
			log.Fatalf("error querying %q: %s", qry, err)
		}
		defer rows.Close()

		userArgs := make(chan structs.UserArgument, 16)
		var readErr error
		go func() {
			defer close(userArgs)
			var oid, subid, level, pos, prec, scale, length sql.NullInt64
			ua := structs.UserArgument{}
			for rows.Next() {
				err = rows.Scan(&oid, &subid, &ua.PackageName, &ua.ObjectName,
					&level, &pos, &ua.ArgumentName, &ua.InOut,
					&ua.DataType, &prec, &scale, &ua.CharacterSetName,
					&ua.PlsType, &length, &ua.TypeOwner, &ua.TypeName, &ua.TypeSubname, &ua.TypeLink)
				if err != nil {
					readErr = err
					log.Fatalf("error reading row %q: %s", rows, err)
				}
				ua.ObjectID, ua.SubprogramID, ua.DataLevel = 0, 0, 0
				ua.Position, ua.DataPrecision, ua.DataScale, ua.CharLength = 0, 0, 0, 0
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
