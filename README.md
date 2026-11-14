# OraCall

OraCall generates Go code for calling Oracle PL/SQL procedures and functions. The main `oracall call` generator reads procedure signatures, creates Go request/response types and database-call wrappers, emits a Protocol Buffers schema and service, and invokes `protoc` to generate Go protobuf/gRPC code. The generated service wrappers call Oracle through [`godror`](https://github.com/godror/godror).

The repository also contains `protoc-gen-oracall`, a separate Protocol Buffers plugin for generating conversions/accessors for godror Oracle objects. It is distinct from the metadata-driven `oracall call` workflow described below.

## Requirements

- A Go toolchain matching the version in [`go.mod`](go.mod).
- `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc` on `PATH` when generating protobuf output. The repository's `go generate` setup provisions protobuf tools, but check that the plugins used by the current generator are installed.
- Oracle access and the prerequisites for godror (including Oracle client requirements) when reading metadata from a live database or running generated callers. See godror's [installation documentation](https://github.com/godror/godror#installation).

Oracle is not needed to generate from a metadata CSV or a package cache, although Oracle is needed to run the generated database callers.

## Generate callers and protobuf service

Install/build the command from this module, then generate for a package or procedure pattern. For example:

```sh
go install .
oracall call \
  -connect 'user/password@service' \
  -base-dir . \
  -db-out github.com/tgulacsi/oracall/generated/db:db \
  -pb-out github.com/tgulacsi/oracall/generated/pb:pb \
  'MY_PKG.%'
```

`-connect` accepts a godror connection string. The pattern is Oracle `LIKE`-style (for example, `MY_PKG.%` selects procedures in that package). `-base-dir` is the filesystem root for generated paths; by default it is the current directory. The `-db-out` and `-pb-out` values are paths, optionally followed by `:package-name`, as in the example. They may be **relative to the current directory** (e.g. `-pb-out testdata/pb:pb`) — the generator then derives the full Go import path (used for `option go_package` and the generated imports) from the target directory's `go.mod` (walking up to the module root; falling back to `$GOPATH/src`). Alternatively they may be full import paths, in which case `-base-dir` should resolve them (e.g. a `$GOPATH`-style tree). When both are specified, the generator writes:

- Go caller/server code and generated tests under the `-db-out` path;
- a `.proto` schema under the `-pb-out` path;
- Go protobuf and gRPC files produced by `protoc`.

The generated Go package exposes a server constructor and procedure methods that accept protobuf request messages. The wrappers manage Oracle transactions, bind arguments, convert results, and can run as gRPC service implementations.

### Metadata input alternatives

The `call` command selects its metadata source in this order:

1. With `-connect`, it reads Oracle dictionary metadata and package source. It normally uses `USER_ARGUMENTS`; for patterns beginning with `DBMS_` or `UTL_` it uses `ALL_ARGUMENTS`.
2. With `-pkg-cache-dir`, it reads the per-package compressed JSON caches from that directory, without connecting to Oracle.
3. Otherwise, it reads semicolon-delimited `USER_ARGUMENTS` CSV from standard input. The expected columns are documented in [`lib/readcsv.go`](lib/readcsv.go); [`one.csv`](one.csv) is a sample.

Examples:

```sh
# Generate from a previously exported USER_ARGUMENTS CSV.
oracall call -base-dir . \
  -db-out github.com/tgulacsi/oracall/generated/db:db \
  -pb-out github.com/tgulacsi/oracall/generated/pb:pb 'DB_WEB.%' < one.csv

# Refresh a package cache from Oracle, then generate from it offline.
oracall update -connect 'user/password@service' \
  -pkg-cache-dir ./oracall-cache 'MY_PKG.%'
oracall call -pkg-cache-dir ./oracall-cache -base-dir . \
  -db-out github.com/tgulacsi/oracall/generated/db:db \
  -pb-out github.com/tgulacsi/oracall/generated/pb:pb 'MY_PKG.%'
```

`-dump FILE` can export metadata read from the database as CSV. The `model` subcommand is a separate database-backed model generator (`oracall model -connect DSN ...`). Run `oracall call -h`, `oracall update -h`, or `oracall model -h` for command flags.

## How generation works

1. **Read metadata.** The generator reads procedure/function arguments and type metadata from Oracle. It also reads package source for documentation and `--oracall:` annotations. Alternatively, it parses the CSV or package cache described above.
2. **Build a PL/SQL argument model.** Metadata rows are grouped by package and subprogram and assembled into arguments, records, and supported associative-array shapes.
3. **Generate callers.** The generator emits Go request/response types and a wrapper for each selected procedure/function. It creates an anonymous PL/SQL block to bridge PL/SQL record and collection arguments to database binds, then executes it using godror. The default maximum table size is 128; `-max-table-size` overrides it.
4. **Generate the API schema.** It emits protobuf messages and a service definition. A `SYS_REFCURSOR` output is represented as a streaming RPC. `protoc` plus its Go and gRPC plugins generate the Go API files.

Package source comments can customize generation. For example, `--oracall:private proc_name` excludes a procedure, `--oracall:rename old_name => new_name` changes its generated name, and `--oracall:replace complex_proc => xml_proc` substitutes an XML/CLOB-backed implementation. The source parser also supports JSON replacements and other annotations; see [`source/parse.go`](source/parse.go).

## Supported types and limitations

The main `call` generator supports these PL/SQL argument shapes:

- simple/scalar PL/SQL types;
- package-level PL/SQL records whose fields are simple types;
- associative arrays (`INDEX BY BINARY_INTEGER`) of simple types;
- associative arrays (`INDEX BY BINARY_INTEGER`) of records whose fields are simple types;
- strongly typed `REF CURSOR` outputs.

In short: **only** simple types, arrays of simple types, records of simple types, and arrays of such records are supported. **Array of array and array in record are not supported.**

Nested composite shapes are not supported: **arrays of arrays are unsupported, and records containing arrays are unsupported**. Nested records and other composite-in-composite shapes are outside the supported record model as well. Associative arrays must use `BINARY_INTEGER` indexing; the existing caller path expects the arrays to be indexed from 1. A returned `SYS_REFCURSOR` must have a declared, strongly typed cursor return type (a `REF CURSOR RETURN` package type); an untyped cursor does not provide enough output-column metadata for generation.

These restrictions apply to the main `oracall call` generator. The separate `lib/objects` and `protoc-gen-oracall` code paths have their own metadata/type behavior and should not be assumed to remove these limitations from the main caller generator.

## Roadmap: Oracle Objects

A planned transition from the flattened PL/SQL binding shims to direct Oracle Objects binding (godror `SliceScan`, `SliceWriteObject`, `ObjectScanner`, `ObjectCollectionWriter` and the related interfaces/helpers) is documented in [PLAN.md](PLAN.md). That work aims to lift the nested composite-shape limitations described above.

## Tests

Run the repository tests with:

```sh
go test ./...
```

The Oracle-free tests run everywhere. The supported-shape surface is covered by `lib/surface_test.go` (metadata CSV → PL/SQL + protobuf) and, end-to-end, by `TestStubSurfaceE2E`, which installs the stub package in `testdata/stub/stub.sql`, generates callers from it, and exercises every supported shape. Oracle-dependent tests need an `ORACALL_DSN` (and, for the end-to-end test, `protoc` plus the Go protoc plugins); they skip cleanly when those are absent.

Some integration and object-generation tests additionally require Oracle privileges or external protobuf tooling. The root integration tests exercise simple values, simple arrays, records, and arrays of records against stored procedures; set up the required database before running those tests.
