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

go build .

if [ -e testdata/db_web.pck ] && echo "$dsn" | grep -q '@XE'; then
    echo '# compiling testdata/db_web.pck'
    go run examples/sp_compile.go -connect="$dsn" testdata/db_web.pck || exit $?
fi

{
if [ -n "$CSV" ] || echo "$dsn" | grep -q '@XE'; then
    ./oracall -F -logtostderr <${1:-testdata/db_web.getriskvagyondetails.csv}
else
    echo ./oracall -F -connect="$dsn" DB_WEB.%} >&2
    ./oracall -F -logtostderr -connect="$dsn" DB_WEB.%
fi
} >examples/db_web/generated_functions.go
go build ./examples/db_web
echo
echo '-----------------------------------------------'
CMD='./db_web -connect='${dsn}" -login="$login" ${2:-DB_web.getriskvagyondetails}"
echo "$CMD"
#$CMD '{"p_lang":"hu", "p_sessionid": "123", "p_kotveny_vagyon":{"teaor": "1233", "forgalom": 0}, "p_telep":[{"telep_azon":"A", "telep_kod":"C"},{"telep_azon":"z", "telep_kod":"x"}]}'
time $CMD '{"p_lang":"hu", "p_sessionid": "123", "p_szerz_azon": 31047441}'
