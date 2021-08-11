#!/usr/bin/env bash
exec sed -i -e '/timestamp "github.com\/golang\/protobuf\/ptypes\/timestamp"/ s,timestamp.*$,custom "github.com/tgulacsi/oracall/custom",; /timestamp\.Timestamp/ s/timestamp\.Timestamp/custom.Timestamp/g' "$@"
