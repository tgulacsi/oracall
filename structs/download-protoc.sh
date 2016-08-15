#!/bin/sh
set -u
set -e
set -x
protoc --version 2>/dev/null | fgrep 'libprotoc 3' || {
curl -L https://github.com/google/protobuf/releases/download/v3.0.0/protoc-3.0.0-linux-x86_64.zip -o /tmp/protoc-3.0.0-linux-x86_64.zip && unzip -p /tmp/protoc-3.0.0-linux-x86_64.zip bin/protoc >$HOME/bin/protoc
chmod 0755 $HOME/bin/protoc
}

which protoc-gen-gofast 2>/dev/null && exit 0
dst=${TMPDIR:-/tmp}/protoc-linux_x86_64.zip
if ! [ -e "$dst" ]; then
	curl -L -sS $(curl -sS https://github.com/google/protobuf/releases/ \
		| sed -n -e 's/^.* href="\([^"]*linux-x86_64.zip\)".*$/https:\/\/github.com\1/p' \
		| sort -ur \
		| head -n 1) -o "$dst"
fi
cd $HOME/bin && unzip "$dst"
