#!/bin/sh
set -e
envfn=$(dirname $0)/../goracle/env
if [ -e "$envfn" ]; then
    . "$envfn"
fi
dsn=${DSN}
if [ -z "$dsn" ]; then
    for fn in .dsn ../goracle/.dsn; do
        if ! [ -e "$fn" ]; then
            continue
        fi
        dsn=$(cat $fn)
        break
    done
fi
echo "dsn=$dsn"
if [ -z "$dsn" ]; then
    exit 3
fi

echo go test -connect=${dsn} ./...
go test -connect=${dsn} ./...
go build

{
if echo "$dsn" | grep -q '@XE'; then
    ./oracall -F <${1:-one.csv}
else
    echo ./oracall -F -connect="$dsn" ${2:-DB_WEB.SENDPREOFFER_31101} >&2
    ./oracall -F -connect="$dsn" ${2:-DB_WEB.SENDPREOFFER_31101}
fi
} >examples/minimal/generated_functions.go
go build ./examples/minimal
echo
echo '-----------------------------------------------'
CMD='./minimal -connect='${dsn}" ${2:-DB_web.sendpreoffer_31101}"
echo "$CMD"
#$CMD '{"p_lang":"hu", "p_sessionid": "123", "p_kotveny_vagyon":{"teaor": "1233", "forgalom": 0}, "p_telep":[{"telep_azon":"A", "telep_kod":"C"},{"telep_azon":"z", "telep_kod":"x"}]}'
time $CMD '{"p_lang":"hu", "p_sessionid": "123", "p_kotveny_vagyon":{"teaor": "1233", "forgalom": 0}, "p_telep":[{"telep_azon":1, "telep_kod":0}], "p_kotveny": {"dijfizgyak":"N"}, "p_kedvezmenyek": ["KEDV01"]}'
