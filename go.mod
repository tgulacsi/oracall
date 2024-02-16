module github.com/tgulacsi/oracall

go 1.21

toolchain go1.22.0

require (
	github.com/UNO-SOFT/zlog v0.8.1
	github.com/antzucaro/matchr v0.0.0-20221106193745-7bed6ef61ef9
	github.com/fatih/structs v1.1.0
	github.com/go-stack/stack v1.8.1
	github.com/godror/godror v0.41.1
	github.com/google/go-cmp v0.6.0
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.1
	github.com/kylelemons/godebug v1.1.0
	github.com/oklog/ulid v1.3.1
	github.com/tgulacsi/go v0.27.3
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/sync v0.6.0
	google.golang.org/grpc v1.61.1
	google.golang.org/protobuf v1.32.0
)

require (
	github.com/godror/knownpb v0.1.1
	github.com/google/renameio/v2 v2.0.0
	github.com/peterbourgon/ff/v3 v3.4.0
)

require (
	github.com/dgryski/go-linebreak v0.0.0-20180812204043-d8f37254e7d3 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	golang.org/x/exp v0.0.0-20240213143201-ec583247a57a // indirect
	golang.org/x/sys v0.17.0 // indirect
	golang.org/x/term v0.17.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240213162025-012b6fc9bca9 // indirect
)

//replace github.com/godror/godror => ../../godror/godror
//replace github.com/UNO-SOFT/zlog => ../../UNO-SOFT/zlog
