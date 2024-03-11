// Copyright 2017, 2023 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"golang.org/x/sync/errgroup"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/google/renameio/v2"
	"github.com/peterbourgon/ff/v3/ffcli"
	custom "github.com/tgulacsi/oracall/custom"
	oracall "github.com/tgulacsi/oracall/lib"

	// for Oracle-specific drivers
	"github.com/godror/godror"
)

//go:generate go generate ./lib
// Should install protobuf-compiler to use it, like
// curl -L https://github.com/google/protobuf/releases/download/v3.0.0-beta-2/protoc-3.0.0-beta-2-linux-x86_64.zip -o /tmp/protoc-3.0.0-beta-2-linux-x86_64.zip && unzip -p /tmp/protoc-3.0.0-beta-2-linux-x86_64.zip protoc >$HOME/bin/protoc

var (
	dsn     string
	verbose zlog.VerboseVar
	logger  = zlog.NewLogger(zlog.MaybeConsoleHandler(&verbose, os.Stderr)).SLog()
)

func main() {
	godror.SetLogger(logger)
	oracall.SetLogger(logger.WithGroup("oracall"))
	if err := Main(); err != nil {
		logger.Error("ERROR", "error", err)
		os.Exit(1)
	}
}

func Main() error {
	gopSrc := filepath.Join(os.Getenv("GOPATH"), "src")

	fs := flag.NewFlagSet("call", flag.ContinueOnError)
	fs.BoolVar(&oracall.SkipMissingTableOf, "skip-missing-table-of", true, "skip functions with missing TableOf info")
	flagDump := fs.String("dump", "", "dump to this csv")
	flagPrint := fs.String("print", "", "print spec to this file")
	flagBaseDir := fs.String("base-dir", gopSrc, "base dir for the -pb-out, -db-out flags")
	flagPbOut := fs.String("pb-out", "", "package import path for the Protocol Buffers files, optionally with the package name, like \"my/pb-pkg:main\"")
	flagDbOut := fs.String("db-out", "-:main", "package name of the generated functions, optionally with the package name, like \"my/db-pkg:main\"")
	flagGenerator := fs.String("protoc-gen", "go", "use protoc-gen-<generator>")
	fs.BoolVar(&oracall.NumberAsString, "number-as-string", false, "add ,string to json tags")
	fs.BoolVar(&custom.ZeroIsAlmostZero, "zero-is-almost-zero", false, "zero should be just almost zero, to distinguish 0 and non-set field")
	fs.Var(&verbose, "v", "verbose logging")
	flagExcept := fs.String("except", "", "except these functions")
	flagReplace := fs.String("replace", "", "funcA=>funcB")
	fs.IntVar(&oracall.MaxTableSize, "max-table-size", oracall.MaxTableSize, "maximum table size for PL/SQL associative arrays")
	fs.StringVar(&dsn, "connect", "", "connect to DB for retrieving function arguments")

	var db *sql.DB

	callCmd := ffcli.Command{Name: "call", FlagSet: fs,
		Exec: func(ctx context.Context, args []string) error {
			if *flagPbOut == "" {
				if *flagDbOut == "" && *flagPrint == "" {
					return errors.New("-pb-out or -db-out is required")
				}
				*flagPbOut = *flagDbOut
			} else if *flagDbOut == "" {
				*flagDbOut = *flagPbOut
			}
			pbPath, pbPkg := parsePkgFlag(*flagPbOut)
			dbPath, dbPkg := parsePkgFlag(*flagDbOut)

			var pattern string
			if len(args) != 0 {
				pattern = args[0]
			}
			if pattern == "" {
				pattern = "%"
			}
			oracall.Gogo = strings.HasPrefix(*flagGenerator, "gogo")

			var functions []oracall.Function
			var err error

			filters := [](func(string) bool){func(string) bool { return true }}
			filter := func(s string) bool {
				for _, f := range filters {
					if !f(s) {
						return false
					}
				}
				return true
			}
			if *flagExcept != "" {
				except := strings.FieldsFunc(*flagExcept, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
				logger.Info("found", "except", except)
				filters = append(filters, func(s string) bool {
					for _, e := range except {
						if strings.EqualFold(e, s) {
							return false
						}
					}
					return true
				})
			}

			var annotations []oracall.Annotation
			logger.Debug("parse", "db", db != nil, "pattern", pattern)
			if db == nil {
				if pattern != "%" {
					rPattern := regexp.MustCompile("(?i)" + strings.Replace(strings.Replace(pattern, ".", "[.]", -1), "%", ".*", -1))
					filters = append(filters, func(s string) bool {
						return rPattern.MatchString(s)
					})
				}
				functions, err = oracall.ParseCsvFile("", filter)
			} else {
				functions, annotations, err = parseDB(ctx, db, pattern, *flagDump, filter)
			}
			if err != nil {
				return fmt.Errorf("read %s: %w", flag.Arg(0), err)
			}

			logger.Debug("print", "print", *flagPrint, "functions", len(functions))
			if *flagPrint != "" {
				fh := io.WriteCloser(os.Stdout)
				fhClose := fh.Close
				if *flagPrint != "-" {
					pf, err := renameio.NewPendingFile(*flagPrint)
					if err != nil {
						return err
					}
					defer pf.Cleanup()
					fh, fhClose = pf, pf.CloseAtomicallyReplace
				}
				defer fhClose()
				bw := bufio.NewWriter(fh)
				defer bw.Flush()
				for _, f := range functions {
					fmt.Fprintf(bw, "\n# %s\n%s\n", f.RealName(), f.Documentation)
					fmt.Fprintln(bw, "\n## Input")
					for _, a := range f.Args {
						if !a.IsInput() {
							continue
						}
						fmt.Fprintf(bw, "  - %s - %s\n", a.Name, a.TypeString("  ", "  "))
					}
					fmt.Fprintln(bw, "\n## Output")
					for _, a := range f.Args {
						if !a.IsOutput() {
							continue
						}
						fmt.Fprintf(bw, "  - %s - %s\n", a.Name, a.TypeString("  ", "  "))
					}
					bw.WriteString("\n")
				}
				if err = bw.Flush(); err != nil {
					return nil
				}
				if err = fhClose(); err != nil {
					return err
				}
			}

			defer os.Stdout.Sync()
			out := os.Stdout
			var testOut *os.File
			if dbPath != "" && dbPath != "-" {
				fn := "oracall.go"
				if dbPkg != "main" {
					fn = dbPkg + ".go"
				}
				fn = filepath.Join(*flagBaseDir, dbPath, fn)
				logger.Info("Writing generated functions", "file", fn)
				// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
				_ = os.MkdirAll(filepath.Dir(fn), 0775)
				outP, err := renameio.NewPendingFile(fn)
				if err != nil {
					return fmt.Errorf("create %s: %w", fn, err)
				}
				defer outP.Cleanup()
				out = outP.File
				testFn := fn[:len(fn)-3] + "_test.go"
				testOutP, err := renameio.NewPendingFile(testFn)
				if err != nil {
					return fmt.Errorf("create %s: %w", testFn, err)
				}
				defer testOutP.Cleanup()
				testOut = testOutP.File
				defer func() {
					if err := outP.CloseAtomicallyReplace(); err != nil {
						logger.Error("close", "file", out.Name(), "error", err)
					}
					if err := testOutP.CloseAtomicallyReplace(); err != nil {
						logger.Error("close", "file", testOut.Name(), "error", err)
					}
				}()
			}

			*flagReplace = strings.TrimSpace(*flagReplace)
			for _, elt := range strings.FieldsFunc(
				rReplace.ReplaceAllLiteralString(*flagReplace, "=>"),
				func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
				i := strings.Index(elt, "=>")
				if i < 0 {
					continue
				}
				a := oracall.Annotation{Type: "replace", Name: elt[:i], Other: elt[i+2:]}
				if i = strings.IndexByte(a.Name, '.'); i >= 0 {
					a.Package, a.Name = a.Name[:i], a.Name[i+1:]
					a.Other = strings.TrimPrefix(a.Other, a.Package)
				}
				annotations = append(annotations, a)
			}
			logger.Info("got", "annotations", annotations)
			functions = oracall.ApplyAnnotations(functions, annotations)
			sort.Slice(functions, func(i, j int) bool { return functions[i].Name() < functions[j].Name() })

			var grp errgroup.Group
			if dbPath != "" {
				grp.Go(func() error {
					pbPath := pbPath
					if pbPath == dbPath {
						pbPath = ""
					}
					if err := oracall.SaveFunctions(
						out, functions,
						dbPkg, pbPath, false,
					); err != nil {
						return fmt.Errorf("save functions: %w", err)
					}
					return nil
				})
			}
			if testOut != nil {
				grp.Go(func() error {
					pbPath := pbPath
					if pbPath == dbPath {
						pbPath = ""
					}
					if err := oracall.SaveFunctionTests(
						testOut, functions,
						dbPkg, pbPath, false,
					); err != nil {
						return fmt.Errorf("save function tests: %w", err)
					}
					return nil
				})
			}

			if pbPath != "" {
				logger.Debug("SaveProtobuf", "pbPath", pbPath)
				grp.Go(func() error {
					pbFn := "oracall.proto"
					if pbPkg != "main" {
						pbFn = pbPkg + ".proto"
					}
					pbFn = filepath.Join(*flagBaseDir, pbPath, pbFn)
					// nosemgrep: go.lang.correctness.permissions.file_permission.incorrect-default-permission
					_ = os.MkdirAll(filepath.Dir(pbFn), 0775)
					logger.Info("Writing Protocol Buffers", "file", pbFn)
					fh, err := os.Create(pbFn)
					if err != nil {
						return fmt.Errorf("create proto: %w", err)
					}
					err = oracall.SaveProtobuf(fh, functions, pbPkg, pbPath)
					if closeErr := fh.Close(); closeErr != nil && err == nil {
						err = closeErr
					}
					if err != nil {
						return fmt.Errorf("SaveProtobuf: %w", err)
					}

					if *flagGenerator == "" {
						return nil
					}

					args := append(make([]string, 0, 5),
						"--proto_path="+*flagBaseDir+":.")
					if oracall.Gogo {
						args = append(args,
							"--"+*flagGenerator+"_out=Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,plugins=grpc:"+*flagBaseDir)
					} else {
						args = append(args, "--go_out="+*flagBaseDir, "--go-grpc_out="+*flagBaseDir)
						if *flagGenerator == "go-vtproto" {
							args = append(args,
								"--"+*flagGenerator+"_out=:"+*flagBaseDir)
						}
					}
					cmd := exec.CommandContext(ctx, "protoc", append(args, pbFn)...)
					cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
					logger.Info("calling", "protoc", cmd.Args)
					if err := cmd.Run(); err != nil {
						return fmt.Errorf("%q: %w", cmd.Args, err)
					}
					cmd = exec.CommandContext(ctx,
						"sed", "-i", "-e",
						(`/timestamp "github.com\/golang\/protobuf\/ptypes\/timestamp"/ s,timestamp.*$,timestamp "github.com/godror/knownpb/timestamppb",; ` +
							`/timestamppb "google.golang.org\/protobuf\/types\/known\/timestamppb"/ s,timestamp.*$,timestamppb "github.com/godror/knownpb/timestamppb",; `),
						strings.TrimSuffix(pbFn, ".proto")+".pb.go",
					)
					cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
					if err := cmd.Run(); err != nil {
						return fmt.Errorf("%q: %w", cmd.Args, err)
					}
					return nil
				})
			}

			return grp.Wait()
		},
	}

	fs = flag.NewFlagSet("model", flag.ContinueOnError)
	flagModelOut := fs.String("o", "-", "output file")
	flagModelPkg := fs.String("pkg", "main", "package name to generate - when empty, no package or import is generated")
	genModelCmd := ffcli.Command{Name: "model", FlagSet: fs,
		Exec: func(ctx context.Context, args []string) error {
			tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			if err != nil {
				return err
			}
			w := io.WriteCloser(os.Stdout)
			closeOk := w.Close
			if *flagModelOut == "" || *flagModelOut == "-" {
				defer w.Close()
			} else {
				fh, err := renameio.NewPendingFile(*flagModelOut)
				if err != nil {
					return err
				}
				defer fh.Cleanup()
				w = fh
				closeOk = fh.CloseAtomicallyReplace
			}
			if err := generateModel(ctx, w, tx, args, *flagModelPkg); err != nil {
				return err
			}
			return closeOk()
		},
	}

	fs = flag.NewFlagSet("oracall", flag.ContinueOnError)
	fs.StringVar(&dsn, "connect", "", "connect to DB for retrieving function arguments")
	app := ffcli.Command{Name: "oracall", FlagSet: fs,
		Subcommands: []*ffcli.Command{&callCmd, &genModelCmd},
	}

	if err := app.Parse(os.Args[1:]); err != nil {
		return err
	}

	if dsn != "" {
		P, parseErr := godror.ParseConnString(dsn)
		if parseErr != nil {
			return fmt.Errorf("%s: %w", dsn, parseErr)
		}
		P.StandaloneConnection = false
		db = sql.OpenDB(godror.NewConnector(P))
		defer db.Close()
		db.SetMaxIdleConns(0)
		if err := db.Ping(); err != nil {
			return fmt.Errorf("ping %s: %w", dsn, err)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	return app.Run(ctx)
}

type dbRow struct {
	Package, Object, InOut sql.NullString
	dbType
	SubID    sql.NullInt64
	OID, Seq int
}

func (r dbRow) String() string {
	return fmt.Sprintf("%s.%s %s", r.Package.String, r.Object.String, r.dbType)
}

type dbType struct {
	Argument                        string
	Data, PLS, Owner, Name, Subname string
	Link, Charset, IndexBy          string
	Level                           int
	Prec, Scale, Length             sql.NullInt64
}

func (t dbType) String() string {
	return fmt.Sprintf("%s{%s}[%d](%s[%s]/%s.%s.%s@%s)", t.Argument, t.Data, t.Level, t.PLS, t.IndexBy, t.Owner, t.Name, t.Subname, t.Link)
}

func parseDB(ctx context.Context, cx *sql.DB, pattern, dumpFn string, filter func(string) bool) (functions []oracall.Function, annotations []oracall.Annotation, err error) {
	tbl, objTbl := "user_arguments", "user_objects"
	if strings.HasPrefix(pattern, "DBMS_") || strings.HasPrefix(pattern, "UTL_") {
		tbl, objTbl = "all_arguments", "all_objects"
	}
	argumentsQry := `` + //nolint:gas
		`SELECT A.*
      FROM
    (SELECT DISTINCT object_id object_id, subprogram_id, sequence*100 seq,
           package_name, object_name,
           data_level, argument_name, in_out,
           data_type, data_precision, data_scale, character_set_name, NULL AS index_by,
           pls_type, char_length, type_owner, type_name, type_subname, type_link
      FROM ` + tbl + `
      WHERE data_type <> 'OBJECT' AND package_name||'.'||object_name LIKE UPPER(:1)
     UNION ALL
     SELECT DISTINCT object_id object_id, subprogram_id, A.sequence*100 + B.attr_no,
            package_name, object_name,
            A.data_level, B.attr_name, A.in_out,
            B.ATTR_TYPE_NAME, B.PRECISION, B.scale, B.character_set_name, NULL AS index_by,
            NVL2(B.ATTR_TYPE_OWNER, B.attr_type_owner||'.', '')||B.attr_type_name, B.length,
			NULL, NULL, NULL, NULL
       FROM all_type_attrs B, ` + tbl + ` A
       WHERE B.owner = A.type_owner AND B.type_name = A.type_name AND
             A.data_type = 'OBJECT' AND
             A.package_name||'.'||A.object_name LIKE UPPER(:2)
     ) A
      ORDER BY 1, 2, 3`

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	objTimeQry := `SELECT last_ddl_time FROM ` + objTbl + ` WHERE object_name = :1 AND object_type <> 'PACKAGE BODY'`
	objTimeStmt, err := cx.PrepareContext(ctx, objTimeQry)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", objTimeQry, err)
	}
	defer objTimeStmt.Close()
	getObjTime := func(name string) (time.Time, error) {
		var t time.Time
		if err := objTimeStmt.QueryRowContext(ctx, name).Scan(&t); err != nil {
			return t, fmt.Errorf("%s [%q]: %w", objTimeQry, name, err)
		}
		return t, nil
	}

	dbCh := make(chan dbRow)
	grp, grpCtx := errgroup.WithContext(ctx)
	grp.Go(func() error {
		defer close(dbCh)
		var collStmt, attrStmt *sql.Stmt
		qry := `SELECT coll_type, elem_type_owner, elem_type_name, elem_type_package,
				   length, precision, scale, character_set_name, index_by,
				   (SELECT MIN(typecode) FROM all_plsql_types B
				      WHERE B.owner = A.elem_type_owner AND
					        B.type_name = A.elem_type_name AND
							B.package_name = A.elem_type_package) typecode
			  FROM all_plsql_coll_types A
			  WHERE owner = :owner AND package_name = :pkg AND type_name = :sub
			UNION
			SELECT coll_type, elem_type_owner, elem_type_name, NULL elem_type_package,
				   length, precision, scale, character_set_name, NULL index_by,
				   (SELECT MIN(typecode) FROM all_types B
				      WHERE B.owner = A.elem_type_owner AND
					        B.type_name = A.elem_type_name) typecode
			  FROM all_coll_types A
			  WHERE (owner, type_name) IN (
			    SELECT :owner, :pkg FROM DUAL
				UNION
				SELECT table_owner, table_name||NVL2(db_link, '@'||db_link, NULL)
				  FROM user_synonyms
				  WHERE synonym_name = :pkg)`
		var resolveTypeShort func(ctx context.Context, typ, owner, name, sub string) ([]dbType, error)
		var err error
		if collStmt, err = cx.PrepareContext(grpCtx, qry); err != nil {
			logger.Error("ERROR", "qry", qry, "error", err)
		} else {
			defer collStmt.Close()
			if rows, err := collStmt.QueryContext(grpCtx,
				sql.Named("owner", ""), sql.Named("pkg", ""), sql.Named("sub", ""),
			); err != nil {
				collStmt = nil
			} else {
				rows.Close()

				qry = `SELECT attr_name, attr_type_owner, attr_type_name, attr_type_package,
                      length, precision, scale, character_set_name, attr_no,
				      (SELECT MIN(typecode) FROM all_plsql_types B
				         WHERE B.owner = A.attr_type_owner AND B.type_name = A.attr_type_name AND B.package_name = A.attr_type_package) typecode
			     FROM all_plsql_type_attrs A
				 WHERE owner = :owner AND package_name = :pkg AND type_name = :sub
				UNION ALL
				SELECT column_name, data_type_owner, data_type, NULL AS attr_type_package,
                      data_length, data_precision, data_scale, character_set_name, column_id AS attr_no,
                      'PL/SQL RECORD' AS typecode
                 FROM all_tab_cols A
                 WHERE NOT EXISTS (SELECT 1 FROM all_plsql_type_attrs B
                                     WHERE B.owner = :owner AND package_name = :pkg AND type_name = :sub) AND
                       hidden_column = 'NO' AND INSTR(column_name, '$') = 0 AND 
                       owner = :owner AND table_name = :pkg
				 ORDER BY attr_no`
				if attrStmt, err = cx.PrepareContext(grpCtx, qry); err != nil {
					logger.Error("qry", qry, "error", err)
				} else {
					defer attrStmt.Close()
					if rows, err := attrStmt.QueryContext(grpCtx,
						sql.Named("owner", ""), sql.Named("pkg", ""), sql.Named("sub", ""),
					); err != nil {
						attrStmt = nil
					} else {
						rows.Close()
						resolveTypeShort = func(ctx context.Context, typ, owner, name, sub string) ([]dbType, error) {
							return resolveType(ctx, collStmt, attrStmt, typ, owner, name, sub)
						}
					}
				}
			}
		}

		qry = argumentsQry
		rows, err := cx.QueryContext(grpCtx,
			qry, pattern, pattern, godror.FetchArraySize(1024), godror.PrefetchCount(1025),
		)
		if err != nil {
			logger.Error("qry", qry, "error", err)
			return fmt.Errorf("%s: %w", qry, err)
		}
		defer rows.Close()

		var seq int
		for rows.Next() {
			var row dbRow
			if err = rows.Scan(&row.OID, &row.SubID, &row.Seq, &row.Package, &row.Object,
				&row.Level, &row.Argument, &row.InOut,
				&row.Data, &row.Prec, &row.Scale, &row.Charset, &row.IndexBy,
				&row.PLS, &row.Length, &row.Owner, &row.Name, &row.Subname, &row.Link,
			); err != nil {
				return fmt.Errorf("reading row=%v: %w", rows, err)
			}
			row.Seq = seq
			seq++
			select {
			case <-grpCtx.Done():
				return grpCtx.Err()
			case dbCh <- row:
			}
			if resolveTypeShort == nil {
				continue
			}
			if row.Data == "PL/SQL TABLE" || row.Data == "PL/SQL RECORD" || row.Data == "REF CURSOR" || row.Data == "TABLE" {
				plus, err := resolveTypeShort(grpCtx, row.Data, row.Owner, row.Name, row.Subname)
				if err != nil {
					return err
				}
				if plus, err = expandArgs(grpCtx, plus, resolveTypeShort); err != nil {
					return err
				}
				for _, p := range plus {
					row.Seq = seq
					seq++
					row.Argument, row.Data, row.Length, row.Prec, row.Scale, row.Charset, row.IndexBy = p.Argument, p.Data, p.Length, p.Prec, p.Scale, p.Charset, p.IndexBy
					row.Owner, row.Name, row.Subname, row.Link = p.Owner, p.Name, p.Subname, p.Link
					row.Level = p.Level
					select {
					case <-grpCtx.Done():
						return grpCtx.Err()
					case dbCh <- row:
					}
				}
			}

		}
		if err != nil {
			return fmt.Errorf("walking rows: %w", err)
		}
		return nil
	})

	var cwMu sync.Mutex
	var cw *csv.Writer
	if dumpFn != "" {
		var lastOk bool
		qry := argumentsQry
		qry = qry[:strings.Index(qry, "FROM "+tbl)] //nolint:gas
		qry = strings.TrimPrefix(qry[strings.LastIndex(qry, "SELECT ")+7:], "DISTINCT ")
		colNames := strings.Split(
			strings.Map(
				func(r rune) rune {
					if 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' || r == '_' {
						lastOk = true
						return r
					}
					if 'a' <= r && r <= 'z' {
						lastOk = true
						return unicode.ToUpper(r)
					}
					if r == ',' {
						return r
					}
					if lastOk {
						lastOk = false
						return ' '
					}
					return -1
				},
				qry,
			),
			",",
		)
		for i, nm := range colNames {
			nm = strings.TrimSpace(nm)
			colNames[i] = nm
			if j := strings.LastIndexByte(nm, ' '); j >= 0 {
				colNames[i] = nm[j+1:]
			}
		}
		var fh *os.File
		if fh, err = os.Create(dumpFn); err != nil {
			logger.Error("create", "dump", dumpFn, "error", err)
			return functions, annotations, fmt.Errorf("%s: %w", dumpFn, err)
		}
		defer func() {
			cwMu.Lock()
			cw.Flush()
			err := cw.Error()
			if err != nil {
				err = fmt.Errorf("csv flush: %w", err)
			}
			cwMu.Unlock()
			if err != nil {
				logger.Error("flush", "csv", fh.Name(), "error", err)
			}
			if err = fh.Close(); err != nil {
				logger.Error("close", "dump", fh.Name(), "error", err)
			}
		}()
		cwMu.Lock()
		cw = csv.NewWriter(fh)
		err = cw.Write(colNames)
		cwMu.Unlock()
		if err != nil {
			logger.Error("write header to csv", "error", err)
			return functions, annotations, fmt.Errorf("write header: %w", err)
		}
	}

	var prevPackage string
	var docsMu sync.Mutex
	var replMu sync.Mutex
	docs := make(map[string]string)
	userArgs := make(chan oracall.UserArgument, 16)
	grp.Go(func() error {
		defer close(userArgs)
		var pkgTime time.Time
		ctx := grpCtx
	Loop:
		for {
			var row dbRow
			var ok bool
			select {
			case <-ctx.Done():
				return ctx.Err()
			case row, ok = <-dbCh:
				if !ok {
					break Loop
				}
				if row.Name == "" {
					row.PLS = row.Data
				} else {
					row.PLS = row.Owner + "." + row.Name + "." + row.Subname
					if row.Link != "" {
						row.PLS += "@" + row.Link
					}
				}
			}
			var ua oracall.UserArgument
			ua.DataType = row.Data
			ua.InOut = row.InOut.String
			if cw != nil {
				N := i64ToString
				cwMu.Lock()
				err := cw.Write([]string{
					strconv.Itoa(row.OID), N(row.SubID), strconv.Itoa(row.Seq), row.Package.String, row.Object.String,
					strconv.Itoa(row.Level), row.Argument, ua.InOut,
					ua.DataType, N(row.Prec), N(row.Scale), row.Charset, row.IndexBy,
					row.PLS, N(row.Length),
					row.Owner, row.Name, row.Subname, row.Link,
				})
				cwMu.Unlock()
				if err != nil {
					return fmt.Errorf("write csv: %w", err)
				}
			}
			if !row.Package.Valid {
				continue
			}
			ua.PackageName = row.Package.String
			if ua.PackageName != prevPackage {
				if pkgTime, err = getObjTime(ua.PackageName); err != nil {
					return err
				}
				prevPackage = ua.PackageName
				grp.Go(func() error {
					buf := bufPool.Get().(*bytes.Buffer)
					defer bufPool.Put(buf)
					buf.Reset()

					logger := logger.With("package", ua.PackageName)
					if srcErr := getSource(ctx, buf, cx, ua.PackageName); srcErr != nil {
						logger.Error("getSource", "error", srcErr)
						return nil
					}
					replMu.Lock()
					for _, b := range rAnnotation.FindAll(buf.Bytes(), -1) {
						b = bytes.TrimSpace(bytes.TrimPrefix(b, []byte("--oracall:")))
						a := oracall.Annotation{Package: ua.PackageName}
						if i := bytes.IndexByte(b, ' '); i < 0 {
							continue
						} else {
							a.Type, b = string(b[:i]), b[i+1:]
						}
						if i := bytes.Index(b, []byte("=>")); i < 0 {
							if i = bytes.IndexByte(b, '='); i < 0 {
								a.Name = string(bytes.TrimSpace(b))
							} else {
								a.Name = string(bytes.TrimSpace(b[:i]))
								size, err := strconv.Atoi(string(bytes.TrimSpace(b[i+1:])))
								if err != nil {
									return err
								}
								a.Size = size
							}
						} else {
							a.Name, a.Other = string(bytes.TrimSpace(b[:i])), string(bytes.TrimSpace(b[i+2:]))
						}
						annotations = append(annotations, a)
					}
					bb := buf.Bytes()
					if len(annotations) != 0 {
						logger.Info("found", "annotations", annotations)
						bb = rAnnotation.ReplaceAll(bb, nil)
					}
					replMu.Unlock()
					subCtx, subCancel := context.WithTimeout(ctx, 1*time.Minute)
					funDocs, docsErr := parseDocs(subCtx, string(bb))
					subCancel()
					logger.Info("parseDocs", "docs", len(funDocs), "error", docsErr)
					docsMu.Lock()
					pn := oracall.UnoCap(ua.PackageName) + "."
					for nm, doc := range funDocs {
						docs[pn+strings.ToLower(nm)] = doc
					}
					docsMu.Unlock()
					if docsErr == context.DeadlineExceeded {
						docsErr = nil
					}
					return docsErr
				})
			}
			ua.LastDDL = pkgTime
			if row.Object.Valid {
				ua.ObjectName = row.Object.String
			}
			if row.Argument != "" {
				ua.ArgumentName = row.Argument
			}
			if row.Charset != "" {
				ua.CharacterSetName = row.Charset
			}
			if row.IndexBy != "" {
				ua.IndexBy = row.IndexBy
			}
			if row.PLS != "" {
				ua.PlsType = row.PLS
			}
			if row.Owner != "" {
				ua.TypeOwner = row.Owner
			}
			if row.Name != "" {
				ua.TypeName = row.Name
			}
			if row.Subname != "" {
				ua.TypeSubname = row.Subname
			}
			if row.Link != "" {
				ua.TypeLink = row.Link
			}
			ua.ObjectID = uint(row.OID)
			if row.SubID.Valid {
				ua.SubprogramID = uint(row.SubID.Int64)
			}
			ua.DataLevel = uint8(row.Level)
			ua.Position = uint(row.Seq)
			if row.Prec.Valid {
				ua.DataPrecision = uint8(row.Prec.Int64)
			}
			if row.Scale.Valid {
				ua.DataScale = uint8(row.Scale.Int64)
			}
			if row.Length.Valid {
				ua.CharLength = uint(row.Length.Int64)
			}
			userArgs <- ua
		}
		return nil
	})
	filteredArgs := make(chan []oracall.UserArgument, 16)
	grp.Go(func() error { oracall.FilterAndGroup(filteredArgs, userArgs, filter); return nil })
	functions = oracall.ParseArguments(filteredArgs, filter)
	if grpErr := grp.Wait(); grpErr != nil {
		logger.Error("ParseArguments", "error", grpErr)
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
				any = true
			} else {
				functions[i] = f
			}
		}
	}
	if any {
		logger.Info("any", "has", docNames)
	}
	return functions, annotations, nil
}

var bufPool = sync.Pool{New: func() interface{} { return bytes.NewBuffer(make([]byte, 0, 1024)) }}

func getSource(ctx context.Context, w io.Writer, cx *sql.DB, packageName string) error {
	qry := "SELECT text FROM user_source WHERE name = UPPER(:1) AND type = 'PACKAGE' ORDER BY line"
	rows, err := cx.QueryContext(ctx, qry, packageName, godror.PrefetchCount(129))
	if err != nil {
		return fmt.Errorf("%s [%q]: %w", qry, packageName, err)
	}
	defer rows.Close()
	for rows.Next() {
		var line sql.NullString
		if err := rows.Scan(&line); err != nil {
			return fmt.Errorf("%s: %w", qry, err)
		}
		if _, err := io.WriteString(w, line.String); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("%s: %w", qry, err)
	}
	return nil
}

func i64ToString(n sql.NullInt64) string {
	if n.Valid {
		return strconv.FormatInt(n.Int64, 10)
	}
	return ""
}

func parsePkgFlag(s string) (string, string) {
	if s == "" {
		return "", ""
	}
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		return s[:i], s[i+1:]
	}
	pkg := path.Base(s)
	if pkg == "" {
		pkg = "main"
	}
	return s, pkg
}

var rReplace = regexp.MustCompile(`\s*=>\s*`)
var rAnnotation = regexp.MustCompile(`--oracall:(?:(replace(_json)?|rename|tag)\s+[a-zA-Z0-9_#]+\s*=>\s*.+|(handle|private)\s+[a-zA-Z0-9_#]+|max-table-size\s+[a-zA-Z0-9_$]+\s*=\s*[0-9]+)`)

func resolveType(ctx context.Context, collStmt, attrStmt *sql.Stmt, typ, owner, pkg, sub string) ([]dbType, error) {
	plus := make([]dbType, 0, 4)
	var rows *sql.Rows
	var err error

	switch typ {
	case "PL/SQL TABLE", "PL/SQL INDEX TABLE", "TABLE":
		/*SELECT coll_type, elem_type_owner, elem_type_name, elem_type_package,
			   length, precision, scale, character_set_name, index_by
		  FROM all_plsql_coll_types
		  WHERE owner = :1 AND package_name = :2 AND type_name = :3*/
		if rows, err = collStmt.QueryContext(ctx,
			sql.Named("owner", owner), sql.Named("pkg", pkg), sql.Named("sub", sub),
		); err != nil {
			return plus, err
		}
		defer rows.Close()
		for rows.Next() {
			var t dbType
			var typeCode string
			if err = rows.Scan(&t.Data, &t.Owner, &t.Subname, &t.Name,
				&t.Length, &t.Prec, &t.Scale, &t.Charset, &t.IndexBy, &typeCode,
			); err != nil {
				return plus, fmt.Errorf("%v: %w", collStmt, err)
			}
			if typeCode != "COLLECTION" {
				t.Data = typeCode
			}
			if t.Data == "" {
				t.Data = t.Subname
			}
			if t.Data == "PL/SQL INDEX TABLE" {
				t.Data = "PL/SQL TABLE"
			}
			t.Level = 1
			plus = append(plus, t)
		}

	case "REF CURSOR":
		/*
			ARGUMENT_NAME	SEQUENCE	DATA_LEVEL	DATA_TYPE
			        	1	0	REF CURSOR
			        	2	1	PL/SQL RECORD
			SZERZ_AZON	3	2	NUMBER
			UZENET_TIP	4	2	CHAR
			HIBAKOD  	5	2	VARCHAR2
			DATUM   	6	2	DATE
			UTOLSO_TIP	7	2	CHAR
			JAVITVA  	8	2	VARCHAR2
			P_IDO_TOL	9	0	DATE
			P_IDO_IG	10	0	DATE
		*/
		plus = append(plus, dbType{Owner: owner, Name: pkg, Subname: sub, Data: "PL/SQL RECORD", Level: 1})

	case "PL/SQL RECORD":
		/*SELECT attr_name, attr_type_owner, attr_type_name, attr_type_package,
		                      length, precision, scale, character_set_name, attr_no
					     FROM all_plsql_type_attrs
						 WHERE owner = :1 AND package_name = :2 AND type_name = :3
						 ORDER BY attr_no*/
		if rows, err = attrStmt.QueryContext(ctx,
			sql.Named("owner", owner), sql.Named("pkg", pkg), sql.Named("sub", sub),
		); err != nil {
			return plus, err
		}
		defer rows.Close()
		for rows.Next() {
			var t dbType
			var attrNo sql.NullInt64
			var typeCode string
			if err = rows.Scan(&t.Argument, &t.Owner, &t.Subname, &t.Name,
				&t.Length, &t.Prec, &t.Scale, &t.Charset, &attrNo, &typeCode,
			); err != nil {
				return plus, fmt.Errorf("%v: %w", attrStmt, err)
			}
			t.Data = typeCode
			if typeCode == "COLLECTION" {
				t.Data = "PL/SQL TABLE"
			}
			if t.Owner == "" && t.Subname != "" {
				t.Data = t.Subname
			}
			if t.Data == "PL/SQL INDEX TABLE" {
				t.Data = "PL/SQL TABLE"
			}
			t.Level = 1
			plus = append(plus, t)
		}
	default:
		return nil, fmt.Errorf("%s: %w", typ, errors.New("unknown type"))
	}
	if rows != nil {
		err = rows.Err()
	}
	if len(plus) == 0 && err == nil {
		err = fmt.Errorf("%s/%s.%s.%s: %w", typ, owner, pkg, sub, errors.New("not found"))
	}
	return plus, err
}

// SUBPROGRAM_ID	ARGUMENT_NAME	SEQUENCE	DATA_LEVEL	DATA_TYPE	IN_OUT
//	P_KARSZAM   	1	0	NUMBER	IN
//	P_TSZAM	        2	0	NUMBER	IN
//	P_OUTPUT    	3	0	PL/SQL TABLE	OUT
//           		4	1	PL/SQL RECORD	OUT
//	F_SZERZ_AZON	5	2	NUMBER	OUT

/*
ARGUMENT_NAME	SEQUENCE	DATA_LEVEL	DATA_TYPE	TYPE_OWNER	TYPE_NAME	TYPE_SUBNAME
P_SZERZ_AZON	1	0	NUMBER
P_OUTPUT    	2	0	PL/SQL TABLE	ABLAK	DB_SPOOLSYS3	TYPE_OUTLIST_078
	            3	1	PL/SQL RECORD	ABLAK	DB_SPOOLSYS3	TYPE_OUTPUT_078
TRANZ_KEZDETE	4	2	DATE
TRANZ_VEGE    	5	2	DATE
KOLTSEG	        6	2	NUMBER
ERTE..TT_ALAPOK	7	2	PL/SQL TABLE	ABLAK	DB_SPOOLSYS3	ATYPE_OUTLIST_UNIT
             	8	3	PL/SQL RECORD	ABLAK	DB_SPOOLSYS3	ATYPE_OUTPUT_UNIT
F_UNIT_RNEV  	9	4	VARCHAR2
F_UNIT_NEV  	10	4	VARCHAR2
F_ISIN       	11	4	VARCHAR2
UNIT_DB	        12	4	NUMBER
UNIT_ARF	    13	4	NUMBER
VASAROLT_ALAPOK	14	2	PL/SQL TABLE	ABLAK	DB_SPOOLSYS3	ATYPE_OUTLIST_UNIT
	            15	3	PL/SQL RECORD	ABLAK	DB_SPOOLSYS3	ATYPE_OUTPUT_UNIT
F_UNIT_RNEV	    16	4	VARCHAR2
F_UNIT_NEV    	17	4	VARCHAR2
F_ISIN        	18	4	VARCHAR2
UNIT_DB       	19	4	NUMBER
UNIT_ARF     	20	4	NUMBER
*/

func expandArgs(ctx context.Context, plus []dbType, resolveTypeShort func(ctx context.Context, typ, owner, name, sub string) ([]dbType, error)) ([]dbType, error) {
	for i := 0; i < len(plus); i++ {
		p := plus[i]
		if p.Data == "PL/SQL INDEX TABLE" {
			p.Data = "PL/SQL TABLE"
		}
		if p.Data == "TABLE" || p.Data == "PL/SQL TABLE" || p.Data == "PL/SQL RECORD" || p.Data == "REF CURSOR" {
			q, err := resolveTypeShort(ctx, p.Data, p.Owner, p.Name, p.Subname)
			if err != nil {
				return plus, fmt.Errorf("%+v: %w", p, err)
			}
			for i, x := range q {
				if x.Data == "PL/SQL INDEX TABLE" {
					x.Data = "PL/SQL TABLE"
				}
				x.Level += p.Level
				q[i] = x
			}
			plus = append(plus[:i+1], append(q, plus[i+1:]...)...)
		}
	}
	return plus, nil
}

// vim: set fileencoding=utf-8 noet:
