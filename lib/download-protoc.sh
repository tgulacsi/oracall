#!/bin/sh
set -u
set -e
set -x
tmpdir="${TMPDIR:-/tmp}/$(basename "$0")-$$"
mkdir -p "$tmpdir"
GOPATH="${GOPATH:-$(go env GOPATH)}"
mkdir -p $GOPATH/src/google/protobuf
dst="$tmpdir/protoc-linux_x86_64.zip"
if ! [ -e "$dst" ]; then
	curl -L -sS "$(curl -L -sS https://github.com/protocolbuffers/protobuf/releases/ \
		| sed -n -e 's/^.* href="\([^"]*linux-x86_64.zip\)".*$/https:\/\/github.com\1/p' \
		| grep -F -v java \
		| sort -ur \
		| head -n 1)" -o "$dst"
fi
cd "$GOPATH" && unzip -o "$dst"
rsync -a include/google/ "$GOPATH/src/google/"
rm -rf include/google
rmdir include || echo ''

if [ ! -e $GOPATH/src/google/api/annotations.proto ] || [ ! -e $GOPATH/src/google/api/timestamp.proto ]; then
	rm -rf $GOPATH/src/google/api
	set -x
	go get github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway
	mkdir -p $GOPATH/src/google
	ln -s $GOPATH/src/github.com/grpc-ecosystem/grpc-gateway/v2/third_party/googleapis/google/api $GOPATH/src/google/api
fi

rm -rf "$tmpdir"

if which protoc-gen-gofast 2>/dev/null && which protoc-gen-go 2>/dev/null; then
	exit 0
fi
go get -u github.com/gogo/protobuf/protoc-gen-gofast
