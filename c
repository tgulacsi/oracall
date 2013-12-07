#!/bin/sh
set -e
envfn=$(dirname $0)/../goracle/env
if [ -e "$envfn" ]; then
    . "$envfn"
fi
go build
./oracall -F <one.csv >examples/minimal/generated_functions.go
go build ./examples/minimal
echo
echo '-----------------------------------------------'
CMD='./minimal -connect=tgulacsi/tgulacsi@XE DB_web.sendpreoffer_31101'
echo "$CMD"
$CMD '{"p_lang":"hu", "p_sessionid": "123", "p_vonalkod":123}'
