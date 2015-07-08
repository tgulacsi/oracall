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

	"github.com/tgulacsi/oracall/structs"
	"gopkg.in/inconshreveable/log15.v2"
	"gopkg.in/rana/ora.v2" // for Oracle-specific drivers
)

var Log = log15.New()

var flagConnect = flag.String("connect", "", "connect to DB for retrieving function arguments")

func main() {
	Log.SetHandler(log15.StderrHandler)
	structs.Log.SetHandler(log15.StderrHandler)

	flagSkipFormat := flag.Bool("F", false, "skip formatting")
	flagVerbose := flag.Bool("v", false, "verbose logging")

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
