// Copyright 2017, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
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

	"github.com/go-kit/kit/log"
	"github.com/tgulacsi/go/loghlp/kitloghlp"
	custom "github.com/tgulacsi/oracall/custom"
	oracall "github.com/tgulacsi/oracall/lib"

	// for Oracle-specific drivers
	"github.com/godror/godror"
)

//go:generate go generate ./lib
// Should install protobuf-compiler to use it, like
// curl -L https://github.com/google/protobuf/releases/download/v3.0.0-beta-2/protoc-3.0.0-beta-2-linux-x86_64.zip -o /tmp/protoc-3.0.0-beta-2-linux-x86_64.zip && unzip -p /tmp/protoc-3.0.0-beta-2-linux-x86_64.zip protoc >$HOME/bin/protoc

var logger = kitloghlp.New(os.Stderr)

var flagConnect = flag.String("connect", "", "connect to DB for retrieving function arguments")

func main() {
	oracall.Log = log.With(logger, "lib", "oracall").Log
	if err := Main(os.Args); err != nil {
		logger.Log("error", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}
}

func Main(args []string) error {
	os.Args = args

	gopSrc := filepath.Join(os.Getenv("GOPATH"), "src")

	flag.BoolVar(&oracall.SkipMissingTableOf, "skip-missing-table-of", true, "skip functions with missing TableOf info")
	flagDump := flag.String("dump", "", "dump to this csv")
	flagBaseDir := flag.String("base-dir", gopSrc, "base dir for the -pb-out, -db-out flags")
	flagPbOut := flag.String("pb-out", "", "package import path for the Protocol Buffers files, optionally with the package name, like \"my/pb-pkg:main\"")
	flagDbOut := flag.String("db-out", "-:main", "package name of the generated functions, optionally with the package name, like \"my/db-pkg:main\"")
	flagGenerator := flag.String("protoc-gen", "gogofast", "use protoc-gen-<generator>")
	flag.BoolVar(&oracall.NumberAsString, "number-as-string", false, "add ,string to json tags")
	flag.BoolVar(&custom.ZeroIsAlmostZero, "zero-is-almost-zero", false, "zero should be just almost zero, to distinguish 0 and non-set field")
	flagVerbose := flag.Bool("v", false, "verbose logging")
	flagExcept := flag.String("except", "", "except these functions")
	flagReplace := flag.String("replace", "", "funcA=>funcB")
	flag.IntVar(&oracall.MaxTableSize, "max-table-size", oracall.MaxTableSize, "maximum table size for PL/SQL associative arrays")
	flagTranIDName := flag.String("tran-id-name", "p_tran_id", "transaction ID argument's name")

	flag.Parse()
	if *flagPbOut == "" {
		if *flagDbOut == "" {
			return errors.New("-pb-out or -db-out is required")
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

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
		Log("except", except)
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
	if *flagConnect == "" {
		if pattern != "%" {
			rPattern := regexp.MustCompile("(?i)" + strings.Replace(strings.Replace(pattern, ".", "[.]", -1), "%", ".*", -1))
			filters = append(filters, func(s string) bool {
				return rPattern.MatchString(s)
			})
		}
		functions, err = oracall.ParseCsvFile("", filter, *flagTranIDName)
	} else {
		var cx *sql.DB
		P, parseErr := godror.ParseConnString(*flagConnect)
		if parseErr != nil {
			return fmt.Errorf("%s: %w", *flagConnect, parseErr)
		}
		P.StandaloneConnection = false
		cx = sql.OpenDB(godror.NewConnector(P))
		defer cx.Close()
		cx.SetMaxIdleConns(0)
		if *flagVerbose {
			godror.Log = log.With(logger, "lib", "godror").Log
		}
		if err = cx.Ping(); err != nil {
			return fmt.Errorf("ping %s: %w", *flagConnect, err)
		}

		functions, annotations, err = parseDB(ctx, cx, pattern, *flagDump, filter, *flagTranIDName)
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", flag.Arg(0), err)
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
		Log("msg", "Writing generated functions", "file", fn)
		os.MkdirAll(filepath.Dir(fn), 0775)
		if out, err = os.Create(fn); err != nil {
			return fmt.Errorf("create %s: %w", fn, err)
		}
		testFn := fn[:len(fn)-3] + "_test.go"
		if testOut, err = os.Create(testFn); err != nil {
			return fmt.Errorf("create %s: %w", testFn, err)
		}
		defer func() {
			if err := out.Close(); err != nil {
				Log("msg", "close", "file", out.Name(), "error", err)
			}
			if err := testOut.Close(); err != nil {
				Log("msg", "close", "file", testOut.Name(), "error", err)
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
	Log("annotations", annotations)
	functions = oracall.ApplyAnnotations(functions, annotations)
	sort.Slice(functions, func(i, j int) bool { return functions[i].Name() < functions[j].Name() })

	var grp errgroup.Group
	grp.Go(func() error {
		pbPath := pbPath
		if pbPath == dbPath {
			pbPath = ""
		}
		if err := oracall.SaveFunctions(
			out, functions,
			dbPkg, pbPath, false,
			*flagTranIDName,
		); err != nil {
			return fmt.Errorf("save functions: %w", err)
		}
		return nil
	})
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

	grp.Go(func() error {
		pbFn := "oracall.proto"
		if pbPkg != "main" {
			pbFn = pbPkg + ".proto"
		}
		pbFn = filepath.Join(*flagBaseDir, pbPath, pbFn)
		os.MkdirAll(filepath.Dir(pbFn), 0775)
		Log("msg", "Writing Protocol Buffers", "file", pbFn)
		fh, err := os.Create(pbFn)
		if err != nil {
			return fmt.Errorf("create proto: %w", err)
		}
		err = oracall.SaveProtobuf(fh, functions, pbPkg, pbPath, *flagTranIDName)
		if closeErr := fh.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			return fmt.Errorf("SaveProtobuf: %w", err)
		}

		args := make([]string, 0, 4)
		if *flagGenerator == "go" {
			args = append(args,
				"--"+*flagGenerator+"_out=:"+*flagBaseDir,
				"--go-grpc_out=:"+*flagBaseDir)
		} else {
			args = append(args,
				"--"+*flagGenerator+"_out=Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,plugins=grpc:"+*flagBaseDir)
		}
		cmd := exec.CommandContext(ctx,
			"protoc",
			append(args, "--proto_path="+*flagBaseDir+":.",
				pbFn)...,
		)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		err = cmd.Run()
		Log("msg", "protoc", "args", cmd.Args, "error", err)
		if err != nil {
			return fmt.Errorf("%q: %w", cmd.Args, err)
		}
		if *flagGenerator == "go" {
			fn := strings.TrimSuffix(pbFn, ".proto") + ".pb.go"
			cmd = exec.CommandContext(ctx, "sed", "-i", "-e",
				`/timestamp "github.com\/golang\/protobuf\/ptypes\/timestamp"/ s,timestamp.*$,custom "github.com/tgulacsi/oracall/custom",; /timestamp\.Timestamp/ s/timestamp\.Timestamp/custom.Timestamp/g`,
				fn,
			)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			err = cmd.Run()
			Log("msg", "replace timestamppb", "file", fn, "args", cmd.Args, "error", err)
			if err != nil {
				return fmt.Errorf("%q: %w", cmd.Args, err)
			}
		}
		return nil
	})

	if err := grp.Wait(); err != nil {
		return err
	}
	return nil
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
	Argument                                       string
	Data, PLS, Owner, Name, Subname, Link, Charset string
	Level                                          int
	Prec, Scale, Length                            sql.NullInt64
}

func (t dbType) String() string {
	return fmt.Sprintf("%s{%s}[%d](%s/%s.%s.%s@%s)", t.Argument, t.Data, t.Level, t.PLS, t.Owner, t.Name, t.Subname, t.Link)
}

func parseDB(ctx context.Context, cx *sql.DB, pattern, dumpFn string, filter func(string) bool, tranIDName string) (functions []oracall.Function, annotations []oracall.Annotation, err error) {
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
           data_type, data_precision, data_scale, character_set_name,
           pls_type, char_length, type_owner, type_name, type_subname, type_link
      FROM ` + tbl + `
      WHERE data_type <> 'OBJECT' AND package_name||'.'||object_name LIKE UPPER(:1)
     UNION ALL
     SELECT DISTINCT object_id object_id, subprogram_id, A.sequence*100 + B.attr_no,
            package_name, object_name,
            A.data_level, B.attr_name, A.in_out,
            B.ATTR_TYPE_NAME, B.PRECISION, B.scale, B.character_set_name,
            NVL2(B.ATTR_TYPE_OWNER, B.attr_type_owner||'.', '')||B.attr_type_name, B.length,
			NULL, NULL, NULL, NULL
       FROM all_type_attrs B, ` + tbl + ` A
       WHERE B.owner = A.type_owner AND B.type_name = A.type_name AND
             A.data_type = 'OBJECT' AND
             A.package_name||'.'||A.object_name LIKE UPPER(:2)
     ) A
      ORDER BY 1, 2, 3`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
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
			logger.Log("WARN", fmt.Errorf("%s: %w", qry, err))
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
					logger.Log("WARN", fmt.Errorf("%s: %w", qry, err))
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
			logger.Log("qry", qry, "error", err)
			return fmt.Errorf("%s: %w", qry, err)
		}
		defer rows.Close()

		var seq int
		for rows.Next() {
			var row dbRow
			if err = rows.Scan(&row.OID, &row.SubID, &row.Seq, &row.Package, &row.Object,
				&row.Level, &row.Argument, &row.InOut,
				&row.Data, &row.Prec, &row.Scale, &row.Charset,
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
					row.Argument, row.Data, row.Length, row.Prec, row.Scale, row.Charset = p.Argument, p.Data, p.Length, p.Prec, p.Scale, p.Charset
					row.Owner, row.Name, row.Subname, row.Link = p.Owner, p.Name, p.Subname, p.Link
					row.Level = p.Level
					//logger.Log("arg", row.Argument, "row", row.Length, "p", p.Length)
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
			logger.Log("msg", "create", "dump", dumpFn, "error", err)
			return functions, annotations, fmt.Errorf("%s: %w", dumpFn, err)
		}
		defer func() {
			cwMu.Lock()
			cw.Flush()
			err = fmt.Errorf("csv flush: %w", cw.Error())
			cwMu.Unlock()
			if err != nil {
				logger.Log("msg", "flush", "csv", fh.Name(), "error", err)
			}
			if err = fh.Close(); err != nil {
				logger.Log("msg", "close", "dump", fh.Name(), "error", err)
			}
		}()
		cwMu.Lock()
		cw = csv.NewWriter(fh)
		err = cw.Write(colNames)
		cwMu.Unlock()
		if err != nil {
			logger.Log("msg", "write header to csv", "error", err)
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
				//logger.Log("arg", row.Argument, "name", row.Name, "sub", row.Subname, "data", row.Data, "pls", row.PLS)
			}
			//logger.Log("row", row)
			var ua oracall.UserArgument
			ua.DataType = row.Data
			ua.InOut = row.InOut.String
			if cw != nil {
				N := i64ToString
				cwMu.Lock()
				err := cw.Write([]string{
					strconv.Itoa(row.OID), N(row.SubID), strconv.Itoa(row.Seq), row.Package.String, row.Object.String,
					strconv.Itoa(row.Level), row.Argument, ua.InOut,
					ua.DataType, N(row.Prec), N(row.Scale), row.Charset,
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

					Log := log.With(logger, "package", ua.PackageName).Log
					if srcErr := getSource(ctx, buf, cx, ua.PackageName); srcErr != nil {
						Log("WARN", "getSource", "error", srcErr)
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
								if a.Size, err = strconv.Atoi(string(bytes.TrimSpace(b[i+1:]))); err != nil {
									return err
								}
							}
						} else {
							a.Name, a.Other = string(bytes.TrimSpace(b[:i])), string(bytes.TrimSpace(b[i+2:]))
						}
						annotations = append(annotations, a)
					}
					bb := buf.Bytes()
					if len(annotations) != 0 {
						Log("annotations", annotations)
						bb = rAnnotation.ReplaceAll(bb, nil)
					}
					replMu.Unlock()
					subCtx, subCancel := context.WithTimeout(ctx, 1*time.Second)
					funDocs, docsErr := parseDocs(subCtx, string(bb))
					subCancel()
					Log("msg", "parseDocs", "docs", len(funDocs), "error", docsErr)
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
	functions = oracall.ParseArguments(filteredArgs, filter, tranIDName)
	if grpErr := grp.Wait(); grpErr != nil {
		logger.Log("msg", "ParseArguments", "error", fmt.Sprintf("%+v", grpErr))
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
				//logger.Log("msg", "No documentation", "function", f.Name())
				any = true
			} else {
				functions[i] = f
			}
		}
	}
	if any {
		logger.Log("has", docNames)
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
var rAnnotation = regexp.MustCompile(`--oracall:(?:(replace(_json)?|rename)\s+[a-zA-Z0-9_#]+\s*=>\s*[a-zA-Z0-9_#]+|(handle|private)\s+[a-zA-Z0-9_#]+|max-table-size\s+[a-zA-Z0-9_$]+\s*=\s*[0-9]+)`)

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
			var indexBy, typeCode string
			if err = rows.Scan(&t.Data, &t.Owner, &t.Subname, &t.Name,
				&t.Length, &t.Prec, &t.Scale, &t.Charset, &indexBy, &typeCode,
			); err != nil {
				return plus, err
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
		//logger.Log("owner", owner, "pkg", pkg, "sub", sub)
		defer rows.Close()
		for rows.Next() {
			var t dbType
			var attrNo sql.NullInt64
			var typeCode string
			if err = rows.Scan(&t.Argument, &t.Owner, &t.Subname, &t.Name,
				&t.Length, &t.Prec, &t.Scale, &t.Charset, &attrNo, &typeCode,
			); err != nil {
				return plus, err
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
	//logger.Log("expand", plus)
	for i := 0; i < len(plus); i++ {
		p := plus[i]
		if p.Data == "PL/SQL INDEX TABLE" {
			p.Data = "PL/SQL TABLE"
		}
		//logger.Log("i", i, "arg", p.Argument, "data", p.Data, "owner", p.Owner, "name", p.Name, "sub", p.Subname)
		if p.Data == "TABLE" || p.Data == "PL/SQL TABLE" || p.Data == "PL/SQL RECORD" || p.Data == "REF CURSOR" {
			q, err := resolveTypeShort(ctx, p.Data, p.Owner, p.Name, p.Subname)
			if err != nil {
				return plus, fmt.Errorf("%+v: %w", p, err)
			}
			//logger.Log("q", q)
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
