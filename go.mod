module github.com/tgulacsi/oracall

go 1.21

toolchain go1.21.0

require (
	github.com/UNO-SOFT/zlog v0.7.7
	github.com/antzucaro/matchr v0.0.0-20221106193745-7bed6ef61ef9
	github.com/fatih/structs v1.1.0
	github.com/go-stack/stack v1.8.1
	github.com/godror/godror v0.40.2
	github.com/google/go-cmp v0.5.9
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0-rc.5
	github.com/kylelemons/godebug v1.1.0
	github.com/oklog/ulid v1.3.1
	github.com/tgulacsi/go v0.25.1
	golang.org/x/net v0.9.0
	golang.org/x/sync v0.3.0
	google.golang.org/grpc v1.57.0
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/godror/knownpb v0.1.1
	github.com/google/renameio/v2 v2.0.0
	github.com/peterbourgon/ff/v3 v3.4.0
	github.com/pkg/errors v0.9.1
)

require (
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	golang.org/x/exp v0.0.0-20230817173708-d852ddb80c63 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/term v0.11.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230525234030-28d5490b6b19 // indirect
)

//replace github.com/godror/godror => ../../godror/godror
//replace github.com/UNO-SOFT/zlog => ../../UNO-SOFT/zlog
