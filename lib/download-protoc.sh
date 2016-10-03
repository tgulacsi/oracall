#!/bin/sh
set -u
set -e
set -x
tmpdir="${TMPDIR:-/tmp}/$(basename $0)-$$"
mkdir -p "$tmpdir"
if ! protoc --version 2>/dev/null | fgrep 'libprotoc 3'; then
	dir=$(pwd)
	cd "$tmpdir"
	curl -L https://github.com/google/protobuf/releases/download/v3.0.0/protoc-3.0.0-linux-x86_64.zip \
		-o protoc-3.0.0-linux-x86_64.zip
	unzip protoc-3.0.0-linux-x86_64.zip
	chmod 0755 bin/protoc
	mv bin/protoc $GOPATH/bin/
	cp -a include/google $GOPATH/src/
	cd "$dir"
fi


if which protoc-gen-gofast 2>/dev/null && which protoc-gen-go 2>/dev/null; then
	exit 0
fi
go get -u github.com/gogo/protobuf/protoc-gen-gofast
dst="$tmpdir/protoc-linux_x86_64.zip"
if ! [ -e "$dst" ]; then
	curl -L -sS $(curl -sS https://github.com/google/protobuf/releases/ \
		| sed -n -e 's/^.* href="\([^"]*linux-x86_64.zip\)".*$/https:\/\/github.com\1/p' \
		| fgrep -v java \
		| sort -ur \
		| head -n 1) -o "$dst"
fi
cd $GOPATH/bin && unzip -o "$dst"
rm -rf "$tmpdir"
