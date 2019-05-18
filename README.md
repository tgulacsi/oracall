# OraCall
OraCall is a package for calling Oracle stored procedures with [gRPC](grpc.io).
As OCI does not allow using PL/SQL record types, this causes some difficulty.

# Getting Started
## Prerequisities
Of course you'll need a working Oracle environment. For instructions, see
[goracle](https://github.com/go-goracle/goracle/blob/master/README.md).

As OraCall uses [gRPC](grpc.io) for the interface, you'll need `protoc`, the
[Protocol Buffers](https://developers.google.com/protocol-buffers/)
[code generator](https://github.com/google/protobuf/releases) and a Go code generator,
such as [protoc-gen-gofast](https://github.com/gogo/protobuf/tree/master/protoc-gen-gofast).
But these two is downloaded by `go generate`.

## Installing

	go get github.com/tgulacsi/oracall
	go generate github.com/tgulacsi/oracall  # this will download protoc and protoc-gen-gofast

Then you can use it:

	oracall -db-out ./pkg/db -pb-out ./pkg/pb -connect 'user/passw@sid' 'MY_PKG.%'

This will generate
  * `my_pkg_functions.go` with the calling machinery,
  * `my_pkg.proto` with the Protocol Buffers messages and RPC service definitions,
  * and call `protoco-gen-gofast` with `my_pkg.proto`, which will generate
    `my_pkg.pb.go` with the Protocol Buffers (un)marshal code.

# How does it work?
## 1. read stored procedures' definitions from the database
First, it reads the functions, procedures' names and their arguments' types from
the given database with the following query:

    SELECT object_id, subprogram_id, package_name, object_name,
           data_level, sequence, argument_name, in_out,
           data_type, data_precision, data_scale, character_set_name,
           pls_type, char_length, type_owner, type_name, type_subname, type_link
      FROM user_arguments
      ORDER BY object_id, subprogram_id, SEQUENCE;

So the argument's type must be readable in `user_arguments`!
For a `SYS_REFCURSOR` (returning a cursor, a query's result set in Oracle parlance),
you have to specialize the type for the returned columns -- see below.

## 2. generate calling machinery

## 3. generate .proto file

## 4. call protoc-gen-gofast

## 5. profit!

# Restrictions
Supported types:
  * PL/SQL simple types
  * PL/SQL record types (defined at stored package level)
  * PL/SQL associative arrays, but just "INDEX BY BINARY_INTEGER" and this arrays
  must be one of the previously supported types (but not arrays!)
  'Cause of OCI restrictions, these arrays must be indexed from 1.
  * cursors.

## Tweaks
If you have a package with mixed content, you can force oracall to ignore them
either by

  * specify command-line flag: `-private pkg.func_to_be_ignored,pkg2.other_private`
  * or add a comment: `--oracall:private func_to_be_ignored` to the package header.

The latter can be used to implement two other hacks:

  1. *rename* a function (if you have a non-oracall-compliant and a replacement, and have to keep both):
     `--oracall:rename non_compliant => compliant
  2. *replace* a function's innards with another function getting and receiving xml as CLOB:
     `--oracall:replace non_compliant_complex => xml_replacement`
	 and this will create the `non_compliant_complex` function's call type in the protobuf file
	 (so this will look like the original complex function), but will call the `xml_replacement`
	 function with the protobuf serialized to XML, and deserialized from the returned XML.


## REF_CURSOR
For example for

    FUNCTION ret_cur(p_id IN INTEGER) RETURN SYS_REFCURSOR IS
	  v_cur SYS_REFCURSOR;
	BEGIN
	  OPEN v_cur FOR
	    SELECT state, amount FROM table;
	  RETURN v_cur;
	END;

You'll have to define the type in the package HEAD:

    TYPE state_amount_rec_typ IS RECORD (state table.state%TYPE, amount table.amount%TYPE);
	TYPE state_amount_cur_typ IS REF CURSOR RETURN state_amount_rec_typ;

Then only change the type in the BODY:

    FUNCTION ret_cur(p_id IN INTEGER) RETURN state_amount_cur_typ IS
	  v_cur state_amount_cur_typ;
	BEGIN
	  OPEN v_cur FOR
	    SELECT state, amount FROM table;
	  RETURN v_cur;
	END;

TL;DR; oracall needs "strongly typed" REF CURSOR - see http://www.dba-oracle.com/plsql/t_plsql_cursor_variables.htm for example!

## Examples
### Minimal
Minimal is a minimal example using OraCall: a simple main package which
connects to the database and calls the function specified on the command line.
Args are specified using JSON.

This must be compiled with your oracall output - such as

    oracall <one.csv >examples/minimal/generated_functions.go \
    && go fmt ./examples/minimal/ \
    && go build ./examples/minimal/ \
    && ./minimal DB_web.sendpreoffer_31101

This calls oracall with one.csv as stdin, and redirects its output to
examples/minimal/generated_functions.go.
Then formats the output (good for safety check, too)
after this it builds the "minimal" binary (into the current dir).
If all this was successful, than calls the resulting binary
to call the specified function.
