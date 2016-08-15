#!/bin/sh
set -e
if [ -z "$CGO_CFLAGS" ]; then
	export CGO_CFLAGS="$ORA_CGO_CFLAGS"
	export "CGO_LDFLAGS=$ORA_CGO_LDFLAGS"
fi
csv=${1:-cig.db_web.csv}

go install

{
	if [ -n "$1" ]; then
		cat "$1"
	else
		gzip -dc examples/db_web/cig.db_web.csv.gz
	fi
} | oracall -proto examples/db_web/generated.proto -F - >examples/db_web/generated_functions.go

protoc --proto_path=$GOPATH/src:. --gofast_out=plugins=grpc:. examples/db_web/generated.proto


go build -o /tmp/db_web ./examples/db_web
