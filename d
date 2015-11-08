#!/bin/sh
set -e
if [ -z "$CGO_CFLAGS" ]; then
	export CGO_CFLAGS="$ORA_CGO_CFLAGS"
	export "CGO_LDFLAGS=$ORA_CGO_LDFLAGS"
fi
dsn=${DSN:-$(grep -v '^#' .dsn | head -n1)}
echo "dsn=$dsn"
if [ -z "$dsn" ]; then
    exit 3
fi

go install

if [ -e testdata/db_web.pck ] && echo "$dsn" | grep -q '@XE'; then
    echo '# compiling testdata/db_web.pck'
    go run examples/sp_compile.go -connect="$dsn" testdata/db_web.pck || exit $?
fi

{
if [ -n "$CSV" ] || echo "$dsn" | grep -q '@XE'; then
    oracall -F <${1:-testdata/db_web.getriskvagyondetails.csv}
else
	set -x
    oracall -F -connect="$dsn" ${1:-DB_WEB.%}
	set +x
fi
} >examples/db_web/generated_functions.go
go build -o /tmp/db_web ./examples/db_web
echo
echo '-----------------------------------------------'
set -x
time /tmp/db_web -connect="${dsn}" -login="$login" "${2:-DB_web.getriskvagyondetails}" \
	'{"p_lang":"hu", "p_sessionid": "123", "p_szerz_azon": 31047441}'
#$CMD '{"p_lang":"hu", "p_sessionid": "123", "p_kotveny_vagyon":{"teaor": "1233", "forgalom": 0}, "p_telep":[{"telep_azon":"A", "telep_kod":"C"},{"telep_azon":"z", "telep_kod":"x"}]}'
