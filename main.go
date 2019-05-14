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
	"github.com/pkg/errors"
	"github.com/tgulacsi/go/loghlp/kitloghlp"
	custom "github.com/tgulacsi/oracall/custom"
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

	flag.Parse()
	if *flagPbOut == "" {
		if *flagDbOut == "" {
			return errors.New("-pb-out or -db-out is required!")
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
		functions, err = oracall.ParseCsvFile("", filter)
	} else {
		var cx *sql.DB
		if cx, err = sql.Open("goracle", *flagConnect); err != nil {
			return errors.Wrap(err, "connect to "+*flagConnect)
		}
		defer cx.Close()
		if *flagVerbose {
			goracle.Log = log.With(logger, "lib", "goracle").Log
		}
		if err = cx.Ping(); err != nil {
			return errors.Wrap(err, "Ping "+*flagConnect)
		}

		functions, annotations, err = parseDB(ctx, cx, pattern, *flagDump, filter)
	}
	if err != nil {
		return errors.Wrap(err, "read "+flag.Arg(0))
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
			return errors.Wrap(err, "create "+fn)
		}
		defer func() {
			if err := out.Close(); err != nil {
				Log("msg", "close", "file", out.Name(), "error", err)
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
	functions = oracall.ApplyAnnotations(functions, annotations)

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
			"--proto_path="+*flagBaseDir+":.",
			"--"+goOut+"=Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,plugins=grpc:"+*flagBaseDir,
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
		return err
	}
	return nil
}

type dbRow struct {
	Package, Object, InOut sql.NullString
	OID, Seq               int
	SubID                  sql.NullInt64
	dbType
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

func parseDB(ctx context.Context, cx *sql.DB, pattern, dumpFn string, filter func(string) bool) (functions []oracall.Function, annotations []oracall.Annotation, err error) {
	tbl := "user_arguments"
	if strings.HasPrefix(pattern, "DBMS_") || strings.HasPrefix(pattern, "UTL_") {
		tbl = "all_arguments"
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

	dbCh := make(chan dbRow)
	grp, grpCtx := errgroup.WithContext(ctx)
	grp.Go(func() error {
		defer close(dbCh)
		var collStmt, attrStmt *sql.Stmt
		qry := `SELECT coll_type, elem_type_owner, elem_type_name, elem_type_package,
				   length, precision, scale, character_set_name, index_by,
				   (SELECT MIN(typecode) FROM all_plsql_types B
				      WHERE B.owner = A.elem_type_owner AND B.type_name = A.elem_type_name AND B.package_name = A.elem_type_package) typecode
			  FROM all_plsql_coll_types A
			  WHERE owner = :1 AND package_name = :2 AND type_name = :3`
		var err error
		if collStmt, err = cx.PrepareContext(grpCtx, qry); err != nil {
			logger.Log("WARN", errors.Wrap(err, qry))
		} else {
			defer collStmt.Close()
			qry = `SELECT attr_name, attr_type_owner, attr_type_name, attr_type_package,
                      length, precision, scale, character_set_name, attr_no,
				      (SELECT MIN(typecode) FROM all_plsql_types B
				         WHERE B.owner = A.attr_type_owner AND B.type_name = A.attr_type_name AND B.package_name = A.attr_type_package) typecode
			     FROM all_plsql_type_attrs A
				 WHERE owner = :1 AND package_name = :2 AND type_name = :3
				 ORDER BY attr_no`
			if attrStmt, err = cx.PrepareContext(grpCtx, qry); err != nil {
				return errors.Wrap(err, qry)
			}
			defer attrStmt.Close()
		}
		resolveTypeShort := func(ctx context.Context, typ, owner, name, sub string) ([]dbType, error) {
			return resolveType(ctx, collStmt, attrStmt, typ, owner, name, sub)
		}

		qry = argumentsQry
		rows, err := cx.QueryContext(grpCtx, qry, pattern, pattern)
		if err != nil {
			logger.Log("qry", qry, "error", err)
			return errors.Wrap(err, qry)
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
				return errors.Wrapf(err, "reading row=%v", rows)
			}
			row.Seq = seq
			seq++
			select {
			case <-grpCtx.Done():
				return grpCtx.Err()
			case dbCh <- row:
			}
			if row.Data == "PL/SQL TABLE" || row.Data == "PL/SQL RECORD" || row.Data == "REF CURSOR" {
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
		return errors.Wrap(err, "walking rows")
	})

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
			return functions, annotations, errors.Wrap(err, dumpFn)
		}
		defer func() {
			cw.Flush()
			if err = cw.Error(); err != nil {
				logger.Log("msg", "flush", "csv", fh.Name(), "error", err)
			}
			if err = fh.Close(); err != nil {
				logger.Log("msg", "close", "dump", fh.Name(), "error", err)
			}
		}()
		cw = csv.NewWriter(fh)
		if err = cw.Write(colNames); err != nil {
			logger.Log("msg", "write header to csv", "error", err)
			return functions, annotations, errors.Wrap(err, "write header")
		}
	}

	var prevPackage string
	var docsMu sync.Mutex
	var replMu sync.Mutex
	docs := make(map[string]string)
	userArgs := make(chan oracall.UserArgument, 16)
	grp.Go(func() error {
		defer close(userArgs)
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
			}
			//logger.Log("row", row)
			var ua oracall.UserArgument
			ua.DataType = row.Data
			ua.InOut = row.InOut.String
			if cw != nil {
				N := i64ToString
				if err = cw.Write([]string{
					strconv.Itoa(row.OID), N(row.SubID), strconv.Itoa(row.Seq), row.Package.String, row.Object.String,
					strconv.Itoa(row.Level), row.Argument, ua.InOut,
					ua.DataType, N(row.Prec), N(row.Scale), row.Charset,
					row.PLS, N(row.Length),
					row.Owner, row.Name, row.Subname, row.Link,
				}); err != nil {
					return errors.Wrapf(err, "writing csv")
				}
			}
			if !row.Package.Valid {
				continue
			}
			ua.PackageName = row.Package.String
			if ua.PackageName != prevPackage {
				prevPackage = ua.PackageName
				grp.Go(func() error {
					buf := bufPool.Get().(*bytes.Buffer)
					defer bufPool.Put(buf)
					buf.Reset()

					Log := log.With(logger, "package", ua.PackageName).Log
					if srcErr := getSource(ctx, buf, cx, ua.PackageName); srcErr != nil {
						Log("msg", "getSource", "error", srcErr)
						return errors.WithMessage(srcErr, ua.PackageName)
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
							a.Name = string(bytes.TrimSpace(b))
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
	functions, err = oracall.ParseArguments(filteredArgs, filter)
	if grpErr := grp.Wait(); grpErr != nil {
		if err == nil {
			err = grpErr
		}
		logger.Log("msg", "ParseArguments", "error", grpErr)
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

var rReplace = regexp.MustCompile("\\s*=>\\s*")
var rAnnotation = regexp.MustCompile("--oracall:((replace(_json)?|rename)\\s+[a-zA-Z0-9_#]+\\s*=>\\s*[a-zA-Z0-9_#]+)|private\\s+[a-zA-Z0-9_#]+")

func resolveType(ctx context.Context, collStmt, attrStmt *sql.Stmt, typ, owner, pkg, sub string) ([]dbType, error) {
	plus := make([]dbType, 0, 4)
	var rows *sql.Rows
	var err error

	switch typ {
	case "PL/SQL TABLE":
		/*SELECT coll_type, elem_type_owner, elem_type_name, elem_type_package,
			   length, precision, scale, character_set_name, index_by
		  FROM all_plsql_coll_types
		  WHERE owner = :1 AND package_name = :2 AND type_name = :3*/
		if rows, err = collStmt.QueryContext(ctx, owner, pkg, sub); err != nil {
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
			t.Level = 1
			plus = append(plus, t)
		}

	case "PL/SQL RECORD", "REF CURSOR":
		/*SELECT attr_name, attr_type_owner, attr_type_name, attr_type_package,
		                      length, precision, scale, character_set_name, attr_no
					     FROM all_plsql_type_attrs
						 WHERE owner = :1 AND package_name = :2 AND type_name = :3
						 ORDER BY attr_no*/
		if rows, err = attrStmt.QueryContext(ctx, owner, pkg, sub); err != nil {
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
				return plus, err
			}
			t.Data = typeCode
			if typeCode == "COLLECTION" {
				t.Data = "PL/SQL TABLE"
			}
			if t.Owner == "" && t.Subname != "" {
				t.Data = t.Subname
			}
			t.Level = 1
			plus = append(plus, t)
		}
	default:
		return nil, errors.Wrap(errors.New("unknown type"), typ)
	}
	err = rows.Err()
	if len(plus) == 0 && err == nil {
		err = errors.Wrapf(errors.New("not found"), "%s/%s.%s.%s", typ, owner, pkg, sub)
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
		if p.Data == "PL/SQL TABLE" || p.Data == "PL/SQL RECORD" || p.Data == "REF CURSOR" {
			q, err := resolveTypeShort(ctx, p.Data, p.Owner, p.Name, p.Subname)
			if err != nil {
				return plus, errors.Wrapf(err, "%+v", p)
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
