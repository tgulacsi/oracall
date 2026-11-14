# Plan: Transition OraCall to Oracle Objects (godror)

## Goal and scope

Replace OraCall's flattened PL/SQL-shim binding with direct binding of Oracle
composite types through godror's Oracle Object interfaces. The user-facing goal
is expressed in terms of `godror.SliceScan`, `godror.SliceWriteObjects & Co`;
this plan maps that request onto the APIs that actually exist in the pinned
godror version (see [Target API](#target-godror-oracle-objects-api)).

OraCall is a Go code generator: `oracall call` reads Oracle procedure metadata
and emits Go callers plus a protobuf/gRPC service (`main.go`, `lib/`). This plan
covers the generator, its emitted code, and the tests/verification needed to
switch how composite arguments are bound. It does **not** propose changing the
public protobuf/gRPC service contract.

## Progress

- **Phase 0** (test suite + green baseline): **done** — `testdata/stub/stub.sql`,
  `stub_suite_test.go`, `lib/surface_test.go`; `go build`/`go vet`/`go test ./...`
  green.
- **Phase 1** (type-name & metadata mapping): **done** — `lib/objects/typename.go`
  adds `Type.QualifiedName`, `GoTypeNameFromQualified`, `TypeRegistry`, and
  `Type.ShapeOf`/`Type.ValidateShape` (rejects array-of-array, array-in-record,
  nested records), with unit tests in `lib/objects/typename_test.go`.
- **Phase 2 research**: **done** — the `user_%` views were inspected on a live
  Oracle Free 23 instance with `testdata/stub/stub.sql` as the test case, and
  godror object binding was probed end-to-end. Findings and the re-scoped
  implementation plan are in [Phase 2](#phase-2--arbitrary-oracle-plsql-types-via-oracle-objects-research-grounded);
  evidence is committed under `testdata/stub/` and the feasibility tests live in
  `lib/objects/plsql_object_test.go`.
- Phases 2 (implementation), 3–5: pending.

## Why

Today the main generator cannot bind PL/SQL records or collections directly, so
it emits an anonymous PL/SQL block that declares local record/table variables,
binds each record field (or each field of a table-of-records) as an individual
Go value, and copies data in and out with PL/SQL `FIRST`/`NEXT` loops
(`Function.PlsqlBlock` / `prepareCall` in `lib/plsql.go`). This:

- limits arguments to a small, flat set of shapes;
- produces large, hard-to-read shim blocks;
- is capped by `MaxTableSize` (default 128, generated-call cap 65536) via
  `godror.PlSQLArrays` / `godror.ArraySize`;
- semantically flattens associative arrays into Go slices, losing index keys.

Oracle Objects (schema-level or package-level object/collection types) let the
driver bind a composite value as a single `*godror.Object` with typed
`WriteObject` / `Scan` implementations. That removes the shim, allows nested
shapes, and is the direction godror itself has moved since v0.50.

## Current state

### Two code paths

| Path | Entry points | Binding strategy | Wired into `main.go`? |
| --- | --- | --- | --- |
| Main generator | `main.go` `call` -> `lib.ParseArguments` -> `lib/writestructs.go` `SaveFunctions`, `lib/proto.go` `SaveProtobuf`, `lib/plsql.go` `PlsqlBlock`/`prepareCall` | Anonymous PL/SQL block; record fields and table-of-record fields are flattened into individual binds; simple arrays bind as Go slices under `godror.PlSQLArrays` | Yes |
| Object path | `lib/objects/objects.go` (`Types`, `FromParams`, `resolveType`), `lib/objects/type.go`, `lib/objects/functions.go`, `lib/objects/procedure.go`; `protoc-gen-oracall/protoc_gen_oracall.go` | Binds complex params as `*godror.Object` with handwritten `GetAttribute`/`SetAttribute` conversion and collection iteration; generates `ToObject`/`FromObject` conversions | No (parallel/experimental) |

`protoc-gen-oracall` is a separate protoc plugin that generates object
conversions from annotated `.proto` files (`oracall_object_type`,
`oracall_field_type` extensions). It is not part of the metadata-driven `call`
flow.

### Supported and unsupported type shapes (main generator)

`lib/readcsv.go` (`ParseArgumentsIter`) explicitly enumerates the modeled
shapes: simple, record at level 0, table of simple, and table-of-record whose
fields are simple at level 2.

| Shape | Supported today |
| --- | --- |
| Simple/scalar PL/SQL types | Yes |
| Array (associative `INDEX BY BINARY_INTEGER`) of simple types | Yes |
| Record of simple types (package-level) | Yes |
| Array of records whose fields are simple types | Yes |
| Strongly typed `REF CURSOR` output | Yes (streaming RPC) |
| Array of array | No — `FLAVOR_TABLE`-of-`FLAVOR_TABLE` branch in `lib/plsql.go` `prepareCall` is empty |
| Array inside a record / record containing a collection | No |
| Nested records / other composite-in-composite | No |

Associative arrays must be `INDEX BY BINARY_INTEGER` and are expected to be
indexed from 1. These limits are documented in `README.md`.

## Target: godror Oracle Objects API

The root `go.mod` pins
`github.com/godror/godror v0.51.6-0.20261108103743-d5815217c841`. In that exact
version:

- There is **no** `godror.SliceWriteObjects` symbol. The requested
  "SliceWriteObjects" concept maps to `godror.SliceWriteObject[E](o, slice)`
  (singular) and `godror.SliceScan[E](&slice, value)` (`obj.go`, ~1576/1630).
- Related helpers: `StructWriteObject`, `StructScan`, `SliceIter`,
  `SliceToObjCollWriter`.
- Interfaces: `ObjectWriter`, `ObjectScanner`, `ObjectCollectionWriter`
  (`stmt.go`, ~2975). The driver has bind paths for these interfaces
  (~`stmt.go:1560`) and an output path for `ObjectScanner` (~`stmt.go:3135`).
- godror v0.50 removed the old `godror:"typename"` struct-tag binding
  (CHANGELOG); these interfaces/helpers are the replacement.

Contract details that matter for the generator:

- `ObjectScanner` includes `ObjectTypeName()`; OUT parameters bound through the
  interface path need a value implementing it, not a bare Go slice.
- `SliceScan` expects a `*godror.Object` as its source, while
  `SliceWriteObject`/`SliceToObjCollWriter` append slice elements to an object
  collection. A plain Go slice cannot be used directly as an OUT destination.
- `SliceWriteObject` appends; it does not preserve arbitrary associative-array
  keys. Sparse or non-1 indexes must be validated explicitly.
- Created `godror.Object` values must be closed; the driver closes the scanner's
  object after `Scan` returns, so generated conversions must not retain it.
- The standalone `protoc-gen-oracall/ORA21062` fixture module pins godror
  v0.45.1 and must be reconciled with the root version.

### Shape -> object approach mapping

| Current shape | Object-based approach |
| --- | --- |
| Simple/scalar | Keep today's direct bind (no object needed) |
| Record of simple types | Generated struct implementing `ObjectTypeName()` + `WriteObject` (via `StructWriteObject`) and `Scan` (via `StructScan`) |
| Array of simple types | Generated wrapper implementing `ObjectCollectionWriter` / `ObjectScanner` using `SliceWriteObject` / `SliceScan` |
| Array of records of simple types | Generated record wrapper (as above) plus a collection wrapper over it |
| Newly enabled: array of array, collection/record nested in record | Recursive wrapper generation; each nested level gets its own `ObjectTypeName`/`WriteObject`/`Scan` |
| Strongly typed `REF CURSOR` | Leave on the existing cursor/streaming path initially; evaluate later |

## Phased plan

Work is staged so the scalar path and the existing curated output remain
available until the object path reaches parity. Each phase lists deliverables
and acceptance checks.

### Phase 0 — Baseline, fixtures, and a green verification loop

- **Build the proper test suite first.** Author a dedicated stored-procedure
  stub package that exercises every *supported* shape (simple scalars in
  IN/OUT/IN OUT positions and as function returns, arrays of simple types,
  records of simple types, arrays of records) plus boundary cases (NULL,
  empty/one-based collections, numeric precision/scale, dates). Install that
  package in the test setup, drive generation from its metadata, and use it as
  the shared, reproducible test surface instead of the ad-hoc fixtures in
  `integration_test.go` (`TST_oracall`, `mix_rec_typ`, `mix_tab_typ`,
  `simple_sum_nums`, `rec_tab_in`). This must land **before** the object-binding
  migration so the old and new paths can be compared on the same surface.
- Record the current generated output for that fixed procedure set as golden
  fixtures.
- Inventory Oracle types resolvable by name (schema-level objects/collections
  in `ALL_TYPES`/`ALL_COLL_TYPES` and package-level types in
  `ALL_PLSQL_TYPES`/`ALL_PLSQL_COLL_TYPES`) and the grants needed.
- **Acceptance:** a repeatable `go test ./...` / generation command with a
  documented Oracle DSN; golden outputs committed; any pre-existing build/test
  failures catalogued as a baseline.

### Phase 1 — Type-name and metadata mapping

- Extend the metadata layer (`lib/readcsv.go`, `lib/struct.go` `Argument`, and
  `lib/objects/objects.go` `Types`) to resolve and carry a fully qualified
  Oracle type name (`OWNER.PACKAGE.TYPE` or `OWNER.TYPE`) for every composite
  argument, for both IN, OUT, IN/OUT, and function RETURN.
- Define a stable mapping from a qualified Oracle type name to a generated Go
  type name; keep a registry so shared types are generated once.
- **Acceptance:** unit tests that map each dictionary `TYPECODE`/`COLL_TYPE`
  (record, nested table, varray, index-by) to a generated Go type and reject
  unsupported/unknown shapes with a clear error; no changes to generated output
  yet.

### Phase 2 — Arbitrary Oracle PL/SQL types via Oracle Objects (research-grounded)

Phase 2 was re-scoped after inspecting the dictionary views on a live Oracle
Free 23 instance, using `testdata/stub/stub.sql` as the test case. The evidence
is committed under `testdata/stub/`: `oracle_views.txt` (view materialization),
`nested_types.txt` (nested PL/SQL types), `godror_object_probe.txt` (godror
binding behaviour). The feasibility tests live in
`lib/objects/plsql_object_test.go`.

#### 2.1 What the dictionary actually contains

There are **two disjoint type universes**, and `USER_ARGUMENTS` points at
whichever one an argument belongs to:

| Universe | Type list | Fields | Collections | In `USER_ARGUMENTS` |
| --- | --- | --- | --- | --- |
| Package-level PL/SQL types | `USER_PLSQL_TYPES` (`TYPECODE` = `PL/SQL RECORD` or `COLLECTION`) | `USER_PLSQL_TYPE_ATTRS` | `USER_PLSQL_COLL_TYPES` | `DATA_TYPE` = `PL/SQL RECORD` or `PL/SQL TABLE`; `TYPE_OWNER`=schema, `TYPE_NAME`=**package**, `TYPE_SUBNAME`=**type**, `TYPE_OBJECT_TYPE`=`PACKAGE` |
| Schema-level SQL types | `USER_TYPES` (`TYPECODE` = `OBJECT` or `COLLECTION`) | `USER_TYPE_ATTRS` | `USER_COLL_TYPES` | `DATA_TYPE` = `OBJECT` or `TABLE`; `TYPE_OWNER`=schema, `TYPE_NAME`=**type**, `TYPE_SUBNAME`=NULL, `TYPE_OBJECT_TYPE`=`TYPE` |

Verified facts that shape the implementation:

1. **The universes do not overlap.** `DEMO_OBJ_T` (a schema-level
   `CREATE TYPE ... AS OBJECT`) appears only in `ALL_TYPES`/`USER_TYPES`;
   `SIMPLE_REC` and `NUM_TAB` (package-level PL/SQL types) appear only in
   `ALL_PLSQL_TYPES`/`USER_PLSQL_TYPES`. A metadata layer must consult both
   and key them differently: package-level types are addressed by
   `OWNER.PACKAGE.TYPE`, schema-level ones by `OWNER.TYPE`.
2. **`USER_ARGUMENTS` does not expand composite arguments.** For a
   `PL/SQL RECORD` or `PL/SQL TABLE` argument there is exactly **one** row at
   `DATA_LEVEL=0`; the nested attributes are *not* repeated as child rows, and
   `ALTER PACKAGE ... COMPILE plscope_settings='identifiers:all'` does not
   change this. The current `main.go` query that unions `ALL_TYPE_ATTRS` for
   `DATA_TYPE='OBJECT'` therefore covers only the schema-level universe; the
   package-level structure must be joined from `USER_PLSQL_TYPE_ATTRS` and
   `USER_PLSQL_COLL_TYPES`.
3. **Composite vs scalar fields are distinguishable in the catalog.** In
   `USER_PLSQL_TYPE_ATTRS`, a scalar field has `ATTR_TYPE_OWNER` and
   `ATTR_TYPE_PACKAGE` **NULL** and `ATTR_TYPE_NAME` set to the base type
   (`NUMBER`, `VARCHAR2`, `DATE`, ...), while a field that is itself a record
   or collection has `ATTR_TYPE_OWNER`/`ATTR_TYPE_NAME`/`ATTR_TYPE_PACKAGE`
   **populated**. This discriminator is what makes nested shapes resolvable.
4. **Collections carry their index type.** `USER_PLSQL_COLL_TYPES.COLL_TYPE` is
   `PL/SQL INDEX TABLE` for associative arrays and `INDEX_BY` gives the index
   type (`BINARY_INTEGER`, `VARCHAR2`, ...). `ELEM_TYPE_OWNER`/`ELEM_TYPE_NAME`/
   `ELEM_TYPE_PACKAGE` are NULL for scalar elements and populated for composite
   elements. `USER_PLSQL_TYPES.CONTAINS_PLSQL` is `YES` exactly when the
   collection's element is itself PL/SQL (i.e. an array of records).
5. **Object-type constructors pollute `USER_ARGUMENTS`.** A schema-level
   `CREATE TYPE` also shows up as a pseudo-argument row (`PACKAGE_NAME` = the
   type name, `TYPE_OBJECT_TYPE`=`TYPE`, `ARGUMENT_NAME`=`SELF`,
   `IN_OUT`=`IN/OUT`) and as an implicit constructor subprogram. The generator
   must skip those, or it will emit callers for types that are not procedures.
6. `USER_PLSQL_TYPES` has **no `OWNER` column** (it is implicitly the current
   schema), unlike `ALL_PLSQL_TYPES`; `USER_TYPES.TYPE_OID` is `BINARY_DOUBLE`
   and is not usable as a join key.

#### 2.2 What godror can bind (the decisive result)

Verified against the pinned godror (`v0.51.6-0...`) on the same instance:

- **Package-level PL/SQL types are bindable as Oracle Objects.**
  `godror.GetObjectType(ctx, db, "ORACALL_STUB.SIMPLE_REC")` resolves to
  `DEMO.ORACALL_STUB.SIMPLE_REC` with 3 attributes, and `"ORACALL_STUB.NUM_TAB"`
  and `"ORACALL_STUB.SIMPLE_REC_TAB"` resolve as collections whose element type
  is the sibling PL/SQL type. The unqualified name fails with `ORA-22303`, so
  **the qualified `OWNER.PACKAGE.TYPE` name is mandatory**.
- **The tag-driven helpers work directly.** `godror.StructWriteObject` and
  `godror.StructScan` round-trip a package-level PL/SQL record using plain
  `godror:"FIELD"` struct tags — no hand-written per-attribute code needed.
- **Collections of records work.** `coll.AppendObject(elem)` accepts an
  element object and `coll.GetItem` returns it.
- `ObjectType.CollectionOf` is **not** reliable for PL/SQL index tables: for
  `NUM_TAB` it reports a synthetic `DEMO.NUM_TAB` rather than the catalog
  element type. The element type must be taken from
  `USER_PLSQL_COLL_TYPES`.

This removes the main risk of Phase 2: the transition is **not** limited to
schema-level SQL object types, and the generated code can be plain structs with
tags rather than bespoke accessors.

#### 2.3 Implementation steps

1. **Metadata layer (Oracle-free, unit-testable part).** Extend the existing
   `lib/objects` resolver (`Types.Names` already unions `user_types` and
   `user_plsql_types`; `Types.get` already unions `all_coll_types` /
   `user_plsql_coll_types` and `all_type_attrs` / `user_plsql_type_attrs`) to:
   - carry the **kind** of each type (`simple` / `record` / `collection`) and
     its **index type** (`INDEX_BY`), using the discriminators in 2.1;
   - distinguish package-level (`OWNER.PACKAGE.TYPE`) from schema-level
     (`OWNER.TYPE`) names and expose a single `ObjectTypeName()` string that
     godror can resolve;
   - resolve **nested** fields recursively using the `ATTR_TYPE_*`
     populated-vs-NULL discriminator, so array-of-array and
     collection-in-record become representable;
   - reuse the Phase 1 `Type.QualifiedName()` / `TypeRegistry` work so a
     shared type is generated once and Go names stay stable.
2. **Argument metadata (`main.go` `parseDB`).**
   - Resolve composite arguments from `USER_ARGUMENTS` **plus** the
     `USER_PLSQL_*` views instead of expecting nested rows in `USER_ARGUMENTS`
     (fact 2);
   - skip object-type constructor rows and pseudo-arguments (fact 5);
   - keep `USER_IDENTIFIERS`/`getSubtypes` for the declared field list of
     package specs, since it is the only source that survives the flattening
     above.
3. **Generated wrappers.** For each composite type emit a Go struct with
   `godror:"ATTR"` field tags (from `USER_PLSQL_TYPE_ATTRS.ATTR_NAME`) and:
   - `ObjectTypeName() string` returning the qualified Oracle name;
   - `WriteObject(*godror.Object) error` → `godror.StructWriteObject`;
   - `Scan(*godror.Object) error` → `godror.StructScan`;
   - for collections, an `ObjectCollectionWriter`/`ObjectScanner` wrapper over
     the element type using `SliceWriteObject`/`SliceScan` (and
     `SliceToObjCollWriter` where the driver wants an `ObjectWriter`).
   Because the helpers are tag-driven, one generated struct can serve the
   protobuf model, the object binding, and the round-trip tests.
4. **Field type mapping.** Map `ATTR_TYPE_NAME`/`ATTR_TYPE_PACKAGE` to Go types
   using the existing `lib/objects` scalar mapping and recurse for composite
   fields. Numeric precision/scale and `CHAR_LENGTH` come from the same views,
   so the generator can honour them instead of guessing.
5. **Lifecycle and null/empty semantics.** Close every object the generated
   code creates, and never retain the object handed to `Scan` (the driver
   closes it). Decide NULL-vs-empty explicitly per direction: an empty
   collection must be written as an empty collection, not skipped (the current
   `lib/objects/type.go` skips the attribute when `len(items) == 0`).
6. **Index semantics.** `INDEX BY BINARY_INTEGER` collections are append-only
   through `SliceWriteObject`/`AppendObject`: the associative-array keys are
   *not* preserved. The generated caller must therefore either (a) document
   1-based contiguous indexing as the contract, or (b) keep the legacy
   `FIRST`/`NEXT` shim for indexed tables. `INDEX BY VARCHAR2` stays out of
   scope for the object path (the current generator already rejects it).

#### 2.4 Acceptance checks

- Unit tests (Oracle-free) over `USER_ARGUMENTS`/`USER_PLSQL_*` fixtures:
  every dictionary shape maps to a Go type and to a stable qualified
  `ObjectTypeName()`, and unsupported/unknown shapes are rejected with a clear
  error (extend `lib/objects/typename_test.go`).
- Oracle-backed tests (`lib/objects/plsql_object_test.go`, skipped without
  `ORACALL_DSN`): install `testdata/stub/stub.sql`, resolve
  `ORACALL_STUB.SIMPLE_REC`, `NUM_TAB` and `SIMPLE_REC_TAB`, and round-trip a
  record and a collection through the godror object API.
- End-to-end: generate callers for `ORACALL_STUB` and execute each shape
  through the generated code.
- Parity: for every shape currently supported, the object path and the legacy
  flattened-shim path must return identical values.

#### 2.5 Open items for Phase 2

- Where do the generated `ObjectTypeName()` strings come from at generation
  time — the connected schema's owner, or the owner's schema as seen by the
  runtime user? (`lib/objects` currently assumes the current schema.)
- Should nested PL/SQL types be emitted as nested Go structs or flattened? The
  catalog supports both; nested structs match the existing `lib/objects`
  snapshot model.
- Does the runtime user need `EXECUTE` on the target package plus read access
  to the type owner's schema for ODPI to resolve the type?

### Phase 3 — Generator integration behind a feature flag

- Add a generator switch (e.g. `-objects` / `--oracall:objects`) that selects
  object binding for composite arguments while keeping scalar binds unchanged.
- When the flag is on, replace the per-function flattened `PlsqlBlock` shim
  with a direct `BEGIN pkg.proc(:1, :2, ...); END;` call binding the generated
  wrappers as `*godror.Object`s; drop `godror.PlSQLArrays`/`ArraySize` for those
  arguments.
- Consolidate the useful parts of `lib/objects` and `protoc-gen-oracall`
  conversion generation into the main `SaveFunctions` path rather than
  maintaining a second generator.
- Keep a fallback: if a type cannot be resolved by name or lacks privileges,
  keep the legacy flattened shim for that argument (and log it), so generation
  never silently breaks.
- **Acceptance:** with the flag off, generated output is byte-identical to the
  Phase 0 golden files; with the flag on, generated code compiles and no
  flattened PL/SQL loops are emitted for composite arguments; the fallback path
  is covered by a unit test.

### Phase 4 — Parity and Oracle-backed integration tests

- Run old and new paths against the same stored procedures and compare results
  for: scalar IN/OUT, simple arrays, records, tables of records, nested shapes
  enabled by objects, nulls, empty collections, sparse/non-1 indexed
  associative arrays, large collections, dates, numbers with precision/scale,
  and LOBs.
- Verify Oracle client/server version requirements (godror's own
  `z_plsql_types_test.go` skips below 12) and execute privileges on the types.
- Update the standalone `ORA21062` fixture module to the root godror version.
- **Acceptance:** parity test passes on a supported Oracle instance; mismatches
  are triaged; version/grant failures produce clear diagnostics, not silent
  fallbacks.

### Phase 5 — Rollout and deprecation

- Make the object path the default after parity, keep the legacy path behind a
  deprecation flag for one release, then remove the flattening code in
  `lib/plsql.go` (`prepareCall` table-of-record loops, `getTableType`,
  `MaxTableSize` shims) and the now-unused `PlSQLArrays` handling.
- Update `README.md` limitations to reflect the newly supported shapes.
- **Acceptance:** full test suite green with the default path; legacy flag still
  documented until removal; README accurate.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| `SliceWriteObject` appends and does not preserve associative-array keys; sparse/non-1 indexes differ from the old `FIRST`/`NEXT` behaviour | Validate index semantics in Phase 4; keep the legacy path for indexed tables until parity; document the difference |
| NULL vs empty collection differs between paths | Define explicit semantics in Phase 2, test both, audit `lib/objects/type.go` |
| OUT binding needs `ObjectScanner` + `ObjectTypeName`; `SliceScan` wants `*godror.Object` | Generate wrappers against the real interfaces; compile-check against pinned godror |
| Object leaks / double-close of the scanner's object | Centralize lifecycle in generated helpers; never retain `Scan` input objects |
| Oracle type visibility, grants, and client/server version (godror tests skip below 12) | Phase 0 inventory, Phase 4 version/grant checks, clear diagnostics, fallback |
| `lib/objects` reader is current-schema oriented; its wrapper test only generates code, not an end-to-end call | Do not assume drop-in; add end-to-end tests in Phase 4 before relying on it |
| godror version mismatch (root 0.51.6-0 vs `ORA21062` 0.45.1) | Reconcile in Phase 4; pin one version |
| Feature-flag complexity during rollout | Keep flag off byte-identical to golden output until Phase 5 |

## Verification and acceptance criteria

- `go build ./...` and `go test ./...` run (unit + generator golden tests;
  Oracle-backed integration tests require a DSN and a supported instance).
- Generator golden tests: flag-off output unchanged; flag-on output compiles
  and contains no flattened composite loops.
- Wrapper round-trip tests for every supported and newly enabled shape, plus
  null/empty/numeric/date/LOB cases.
- Old-vs-new parity on shared fixtures with identical results.
- Oracle client/server version and grant checks with actionable errors.
- `README.md` limitations updated to match what the object path actually
  supports at rollout.

> Note on the current baseline (verified; the branch is **not** green):
> `go run . --help` works and the main package builds. This is a WIP branch
> (`objects`) whose `lib/objects` package is being migrated from
> `github.com/go-json-experiment/json` to `encoding/json/v2`; that migration is
> still in progress and currently leaves `lib/objects` failing to compile.
> `go vet` findings in `lib`, `flag.Parse()` in `integration_test.go`'s `init`,
> `source` documentation-formatting expectations, `custom` datetime tests that
> are timezone-dependent (pass under `TZ=Europe/Budapest`, fail under UTC), and
> a `protoc-gen-oracall` test that needs an Oracle DSN via `env.sh` also keep
> `go test ./...` red. These are pre-existing conditions unrelated to this plan;
> Phase 0 must establish a green baseline before generator changes are judged.

## Open questions / decisions

- Are the target types schema-level SQL object types, package-level PL/SQL
  types, or both? godror's object binding and `ALL_PLSQL_TYPES` metadata differ
  for each; confirm before Phase 1.
- Should `INDEX BY VARCHAR2` / sparse associative arrays remain on the legacy
  path permanently, or be represented with an explicit key-bearing Go type?
- Keep `REF CURSOR` on the streaming path or migrate it too?
- One consolidated generator (main `SaveFunctions`) or keep
  `protoc-gen-oracall` as a separate conversion-only plugin?
- Minimum supported Oracle client/server version and required grants.

## Key files and symbols

- `main.go` — `Main`, `call`/`model`/`update`, `parseDB`, `resolveType`
- `lib/readcsv.go` — `ParseArgumentsIter`, supported-shape enumeration
- `lib/struct.go` — `Argument` (`TableOf`, `RecordOf`, `Flavor`)
- `lib/plsql.go` — `PlsqlBlock`, `prepareCall`, `MaxTableSize`, flattened loops
- `lib/writestructs.go` — `SaveFunctions`, `SaveFunctionTests`
- `lib/proto.go` — `SaveProtobuf`, `HasCursorOut` streaming
- `lib/pkgcache.go` — `PackageCache`, offline metadata
- `lib/objects/` — `Types`, `FromParams`, `resolveType`, `type.go`, `procedure.go`, `functions.go`
- `protoc-gen-oracall/protoc_gen_oracall.go` — `ToObject`/`FromObject` generation
- `integration_test.go` — Oracle-backed fixtures
- godror (pinned): `obj.go` (`SliceWriteObject`, `SliceScan`, `StructWriteObject`, `StructScan`), `stmt.go` (`ObjectWriter`, `ObjectScanner`, `ObjectCollectionWriter`, `SliceIter`, `SliceToObjCollWriter`), `z_plsql_types_test.go`, `CHANGELOG.md`
