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
		if flag.NArg() < 1 {
			log.Fatalf("please specify the csv to read function argument data from!")
		}
		functions, err = structs.ParseCsv(flag.Arg(0))
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
      WHERE package_name||'.'||object_name LIKE ?
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
			ua := structs.UserArgument{}
			for rows.Next() {
				err = rows.Scan(&ua.ObjectID, &ua.SubprogramID, &ua.PackageName, &ua.ObjectName,
					&ua.DataLevel, &ua.Position, &ua.ArgumentName, &ua.InOut,
					&ua.DataType, &ua.DataPrecision, &ua.DataScale, &ua.CharacterSetName,
					&ua.PlsType, &ua.CharLength, &ua.TypeOwner, &ua.TypeName, &ua.TypeSubname, &ua.TypeLink)
				if err != nil {
					readErr = err
					log.Fatalf("error reading row %q: %s", rows, err)
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
