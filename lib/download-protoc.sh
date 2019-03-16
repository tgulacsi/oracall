#!/bin/sh
set -u
set -e
set -x
tmpdir="${TMPDIR:-/tmp}/$(basename "$0")-$$"
mkdir -p "$tmpdir"
GOPATH="${GOPATH:-$(go env GOPATH)}"
if ! protoc --version 2>/dev/null | grep -F 'libprotoc 3'; then
	dst="$tmpdir/protoc-linux_x86_64.zip"
	if ! [ -e "$dst" ]; then
		curl -L -sS "$(curl -L -sS https://github.com/google/protobuf/releases/ \
			| sed -n -e 's/^.* href="\([^"]*linux-x86_64.zip\)".*$/https:\/\/github.com\1/p' \
			| grep -F -v java \
			| sort -ur \
			| head -n 1)" -o "$dst"
	fi
	cd "$GOPATH" && unzip -o "$dst"
	rsync -a include/google/ "$GOPATH/src/google/"
	rm -rf include/google
	rmdir include || echo ''
fi

if [ ! -e $GOPATH/src/google/api/annotations.proto ]; then
	rm -rf $GOPATH/src/google/api
	go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
	ln -s ../github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis/google/api $GOPATH/src/google/api
fi

rm -rf "$tmpdir"

if which protoc-gen-gofast 2>/dev/null && which protoc-gen-go 2>/dev/null; then
	exit 0
fi
go get -u github.com/gogo/protobuf/protoc-gen-gofast
