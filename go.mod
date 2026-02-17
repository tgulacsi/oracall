module github.com/tgulacsi/oracall

go 1.25.4

require (
	github.com/UNO-SOFT/zlog v0.8.6
	github.com/antzucaro/matchr v0.0.0-20221106193745-7bed6ef61ef9
	github.com/fatih/structs v1.1.0
	github.com/go-stack/stack v1.8.1
	github.com/godror/godror v0.50.0
	github.com/google/go-cmp v0.7.0
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.3
	github.com/kylelemons/godebug v1.1.0
	github.com/tgulacsi/go v0.28.13
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sync v0.19.0
	google.golang.org/grpc v1.79.1
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/UNO-SOFT/w3ctrace v0.0.0-20260217182632-62e23a54a05a
	github.com/go-json-experiment/json v0.0.0-20251027170946-4849db3c2f7e
	github.com/godror/knownpb v0.3.0
	github.com/google/renameio/v2 v2.0.0
	github.com/oklog/ulid/v2 v2.1.1
	github.com/peterbourgon/ff/v3 v3.4.0
)

require (
	github.com/VictoriaMetrics/easyproto v1.1.3 // indirect
	github.com/bufbuild/protoplugin v0.0.0-20250106231243-3a819552c9d9 // indirect
	github.com/dgryski/go-linebreak v0.0.0-20180812204043-d8f37254e7d3 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/mfridman/buildversion v0.3.0 // indirect
	github.com/mfridman/protoc-gen-go-json v1.5.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	golang.org/x/exp v0.0.0-20260212183809-81e46e3db34a // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/term v0.40.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.6.1 // indirect
)

// replace github.com/godror/godror => ../../godror/godror

//replace github.com/UNO-SOFT/zlog => ../../UNO-SOFT/zlog

tool (
	github.com/mfridman/protoc-gen-go-json
	github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto
	google.golang.org/grpc/cmd/protoc-gen-go-grpc
	google.golang.org/protobuf/cmd/protoc-gen-go
)
