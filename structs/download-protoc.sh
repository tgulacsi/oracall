#!/bin/sh
protoc --version && exit 0
set -u
set -e
dst=${TMPDIR:-/tmp}/protoc-linux_x86_64.zip
curl -L -sS $(curl -sS https://github.com/google/protobuf/releases/ \
	| sed -n -e 's/^.* href="\([^"]*linux-x86_64.zip\)".*$/https:\/\/github.com\1/p' \
	| sort -ur \
	| head -n 1) -o "$dst"
cd $HOME/bin && unzip "$dst"
