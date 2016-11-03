#!/bin/sh
set -u
set -e
set -x
tmpdir="${TMPDIR:-/tmp}/$(basename $0)-$$"
mkdir -p "$tmpdir"
if ! protoc --version 2>/dev/null | fgrep 'libprotoc 3'; then
	dst="$tmpdir/protoc-linux_x86_64.zip"
	if ! [ -e "$dst" ]; then
		curl -L -sS $(curl -sS https://github.com/google/protobuf/releases/ \
			| sed -n -e 's/^.* href="\([^"]*linux-x86_64.zip\)".*$/https:\/\/github.com\1/p' \
			| fgrep -v java \
			| sort -ur \
			| head -n 1) -o "$dst"
	fi
	cd $GOPATH && unzip -o "$dst"
	rsync -a include/google/ $GOPATH/src/google/
	rm -rf include/google
	rmdir include || echo ''
fi

if [ ! -e $GOPATH/src/google/api/annotations.proto ]; then
	dst="$tmpdir/common-protos.zip"
	curl -L -sS https://github.com/googleapis/googleapis/archive/common-protos-1_3_1.zip -o "$dst"
	cd $GOPATH/src && unzip -o "$dst"
	rsync -a googleapis-common-protos-1_3_1/google/ $GOPATH/src/google/
	rm -rf googleapis-common-protos-1_3_1
fi
rm -rf "$tmpdir"

if which protoc-gen-gofast 2>/dev/null && which protoc-gen-go 2>/dev/null; then
	exit 0
fi
go get -u github.com/gogo/protobuf/protoc-gen-gofast
