/*
Copyright 2017 Tamás Gulácsi

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
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"flag"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/sync/errgroup"

	"go4.org/syncutil"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	"github.com/tgulacsi/go/loghlp/kitloghlp"
	custom "github.com/tgulacsi/oracall/custom"
	custom_ora "github.com/tgulacsi/oracall/custom_ora"
	oracall "github.com/tgulacsi/oracall/lib"

	// for Oracle-specific drivers
	goracle "gopkg.in/goracle.v2"
)

//go:generate go generate ./lib
// Should install protobuf-compiler to use it, like
// curl -L https://github.com/google/protobuf/releases/download/v3.0.0-beta-2/protoc-3.0.0-beta-2-linux-x86_64.zip -o /tmp/protoc-3.0.0-beta-2-linux-x86_64.zip && unzip -p /tmp/protoc-3.0.0-beta-2-linux-x86_64.zip protoc >$HOME/bin/protoc

var logger = kitloghlp.New(os.Stderr)

var flagConnect = flag.String("connect", "", "connect to DB for retrieving function arguments")

func main() {
	oracall.Log = log.With(logger, "lib", "oracall").Log
	os.Exit(Main(os.Args))
}

func Main(args []string) int {
	os.Args = args
	GopSrc := filepath.Join(os.Getenv("GOPATH"), "src")

	flag.BoolVar(&oracall.SkipMissingTableOf, "skip-missing-table-of", true, "skip functions with missing TableOf info")
	flagDump := flag.String("dump", "", "dump to this csv")
	flagBaseDir := flag.String("base-dir", GopSrc, "base dir for the -pb-out, -db-out flags")
	flagPbOut := flag.String("pb-out", "", "package import path for the Protocol Buffers files, optionally with the package name, like \"my/pb-pkg:main\"")
	flagDbOut := flag.String("db-out", "-:main", "package name of the generated functions, optionally with the package name, like \"my/db-pkg:main\"")
	flagGenerator := flag.String("protoc-gen", "gofast", "use protoc-gen-<generator>")
	flag.BoolVar(&oracall.NumberAsString, "number-as-string", false, "add ,string to json tags")
	flag.BoolVar(&custom.ZeroIsAlmostZero, "zero-is-almost-zero", false, "zero should be just almost zero, to distinguish 0 and non-set field")
	flagVerbose := flag.Bool("v", false, "verbose logging")

	flag.Parse()
	custom_ora.ZeroIsAlmostZero = custom.ZeroIsAlmostZero
	if *flagPbOut == "" {
		if *flagDbOut == "" {
			logger.Log("-pb-out or -db-out is required!")
			os.Exit(1)
		}
		*flagPbOut = *flagDbOut
	} else if *flagDbOut == "" {
		*flagDbOut = *flagPbOut
	}
	pbPath, pbPkg := parsePkgFlag(*flagPbOut)
	dbPath, dbPkg := parsePkgFlag(*flagDbOut)

	Log := logger.Log
	pattern := flag.Arg(0)
	if pattern == "" {
		pattern = "%"
	}
	oracall.Gogo = *flagGenerator != "go"

	var functions []oracall.Function
	var err error

	if *flagConnect == "" {
		var filter func(string) bool
		if pattern != "%" {
			rPattern := regexp.MustCompile("(?i)" + strings.Replace(strings.Replace(pattern, ".", "[.]", -1), "%", ".*", -1))
			filter = func(s string) bool {
				return rPattern.MatchString(s)
			}
		}
		functions, err = oracall.ParseCsvFile("", filter)
	} else {
		cx, err := sql.Open("goracle", *flagConnect)
		if err != nil {
			Log("msg", "connecting to", "dsn", *flagConnect, "error", err)
			return 1
		}
		defer cx.Close()
		if *flagVerbose {
			goracle.Log = log.With(logger, "lib", "goracle").Log
		}
		if err = cx.Ping(); err != nil {
			Log("msg", "pinging", "dsn", *flagConnect, "error", err)
			return 1
		}
		tbl := "user_arguments"
		if strings.HasPrefix(pattern, "DBMS_") || strings.HasPrefix(pattern, "UTL_") {
			tbl = "all_arguments"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var grp syncutil.Group

		qry := `
    SELECT A.*
	  FROM
	(SELECT DISTINCT object_id, subprogram_id, sequence,
	       package_name, object_name,
           data_level, position, argument_name, in_out,
           data_type, data_precision, data_scale, character_set_name,
           pls_type, char_length, type_owner, type_name, type_subname, type_link
      FROM ` + tbl + `
	  WHERE package_name||'.'||object_name LIKE UPPER(:1)
     ) A
      ORDER BY 1, 2, 3`
		rows, err := cx.QueryContext(ctx, qry, pattern)
		if err != nil {
			Log("qry", qry, "error", err)
			return 2
		}
		defer rows.Close()

		var cw *csv.Writer
		if *flagDump != "" {
			fh, err := os.Create(*flagDump)
			if err != nil {
				Log("msg", "create", "dump", *flagDump, "error", err)
				return 3
			}
			defer func() {
				cw.Flush()
				if err := cw.Error(); err != nil {
					Log("msg", "flush", "csv", fh.Name(), "error", err)
				}
				if err := fh.Close(); err != nil {
					Log("msg", "close", "dump", fh.Name(), "error", err)
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
				Log("msg", "write header to csv", "error", err)
				return 3
			}
		}

		var prevPackage string
		var docsMu sync.Mutex
		docs := make(map[string]string)
		userArgs := make(chan oracall.UserArgument, 16)
		grp.Go(func() error {
			defer close(userArgs)
			var pn, on, an, cs, plsT, tOwner, tName, tSub, tLink sql.NullString
			var oid, seq, subid, level, pos, prec, scale, length sql.NullInt64
			ua := oracall.UserArgument{}
			for rows.Next() {
				err = rows.Scan(&oid, &subid, &seq, &pn, &on,
					&level, &pos, &an, &ua.InOut,
					&ua.DataType, &prec, &scale, &cs,
					&plsT, &length, &tOwner, &tName, &tSub, &tLink)
				if err != nil {
					return errors.Wrapf(err, "reading row=%v", rows)
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
						return errors.Wrapf(err, "writing csv")
					}
				}
				ua.PackageName, ua.ObjectName, ua.ArgumentName = "", "", ""
				ua.ObjectID, ua.SubprogramID, ua.DataLevel = 0, 0, 0
				ua.Position, ua.DataPrecision, ua.DataScale, ua.CharLength = 0, 0, 0, 0
				if pn.Valid {
					ua.PackageName = pn.String
					if ua.PackageName != prevPackage {
						prevPackage = ua.PackageName
						grp.Go(func() error {
							buf := bufPool.Get().(*bytes.Buffer)
							defer bufPool.Put(buf)
							buf.Reset()

							Log := log.With(logger, "package", ua.PackageName).Log
							if err := getSource(ctx, buf, cx, ua.PackageName); err != nil {
								Log("msg", "getSource", "error", err)
								return errors.WithMessage(err, ua.PackageName)
							}
							ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
							funDocs, err := parseDocs(ctx, buf.String())
							cancel()
							Log("msg", "parseDocs", "docs", len(funDocs), "error", err)
							docsMu.Lock()
							pn := oracall.UnoCap(ua.PackageName) + "."
							for nm, doc := range funDocs {
								docs[pn+strings.ToLower(nm)] = doc
							}
							docsMu.Unlock()
							if err == context.DeadlineExceeded {
								err = nil
							}
							return err
						})
					}
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
				return errors.Wrapf(err, "walking rows")
			}
			return nil
		})
		functions, err = oracall.ParseArguments(userArgs)
		if grpErr := grp.Err(); grpErr != nil {
			if err == nil {
				err = grpErr
			}
			Log("msg", "ParseArguments", "error", grpErr)
		}
		docNames := make([]string, 0, len(docs))
		for k := range docs {
			docNames = append(docNames, k)
		}
		sort.Strings(docNames)
		var any bool
		for i, f := range functions {
			if f.Documentation == "" {
				if f.Documentation = docs[f.Name()]; f.Documentation == "" {
					Log("msg", "No documentation", "function", f.Name())
					any = true
				} else {
					functions[i] = f
				}
			}
		}
		if any {
			Log("has", docNames)
		}
	}
	if err != nil {
		Log("msg", "read", "file", flag.Arg(0), "error", err)
		return 3
	}

	defer os.Stdout.Sync()
	out := os.Stdout
	if dbPath != "" && dbPath != "-" {
		fn := "oracall.go"
		if dbPkg != "main" {
			fn = dbPkg + ".go"
		}
		fn = filepath.Join(*flagBaseDir, dbPath, fn)
		Log("msg", "Writing generated functions", "file", fn)
		os.MkdirAll(filepath.Dir(fn), 0775)
		if out, err = os.Create(fn); err != nil {
			Log("msg", "create", "file", fn, "error", err)
			return 1
		}
		defer func() {
			if err := out.Close(); err != nil {
				Log("msg", "close", "file", out.Name(), "error", err)
			}
		}()
	}

	var grp errgroup.Group
	grp.Go(func() error {
		pbPath := pbPath
		if pbPath == dbPath {
			pbPath = ""
		}
		if err := oracall.SaveFunctions(
			out, functions,
			dbPkg, pbPath, false,
		); err != nil {
			return errors.Wrap(err, "save functions")
		}
		return nil
	})

	grp.Go(func() error {
		fn := "oracall.proto"
		if pbPkg != "main" {
			fn = pbPkg + ".proto"
		}
		fn = filepath.Join(*flagBaseDir, pbPath, fn)
		os.MkdirAll(filepath.Dir(fn), 0775)
		Log("msg", "Writing Protocol Buffers", "file", fn)
		fh, err := os.Create(fn)
		if err != nil {
			return errors.Wrap(err, "create proto")
		}
		err = oracall.SaveProtobuf(fh, functions, pbPkg)
		if closeErr := fh.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			return errors.Wrap(err, "SaveProtobuf")
		}

		goOut := *flagGenerator + "_out"
		cmd := exec.Command(
			"protoc",
			"--proto_path="+GopSrc+":.",
			"--"+goOut+"=plugins=grpc:"+*flagBaseDir,
			fn,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "%q", cmd.Args)
		}
		return nil
	})

	if err := grp.Wait(); err != nil {
		Log("error", err)
		return 1
	}
	return 0
}

var bufPool = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 1024)) }}

func getSource(ctx context.Context, w io.Writer, cx *sql.DB, packageName string) error {
	qry := "SELECT text FROM user_source WHERE name = UPPER(:1) AND type = 'PACKAGE' ORDER BY line"
	rows, err := cx.QueryContext(ctx, qry, packageName)
	if err != nil {
		return errors.Wrap(err, qry)
	}
	defer rows.Close()
	for rows.Next() {
		var line sql.NullString
		if err := rows.Scan(&line); err != nil {
			return errors.Wrap(err, qry)
		}
		if _, err := io.WriteString(w, line.String); err != nil {
			return err
		}
	}
	return errors.Wrap(rows.Err(), qry)
}

func i64ToString(n sql.NullInt64) string {
	if n.Valid {
		return strconv.FormatInt(n.Int64, 10)
	}
	return ""
}

func parsePkgFlag(s string) (string, string) {
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		return s[:i], s[i+1:]
	}
	pkg := path.Base(s)
	if pkg == "" {
		pkg = "main"
	}
	return s, pkg
}

// vim: set fileencoding=utf-8 noet:
