module github.com/tgulacsi/oracall

go 1.21

toolchain go1.21.0

require (
	github.com/antzucaro/matchr v0.0.0-20210222213004-b04723ef80f0
	github.com/fatih/structs v1.1.0
	github.com/go-stack/stack v1.8.1
	github.com/godror/godror v0.37.0
	github.com/google/go-cmp v0.5.8
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0-rc.2
	github.com/kylelemons/godebug v1.1.0
	github.com/oklog/ulid v1.3.1
	github.com/tgulacsi/go v0.24.3
	golang.org/x/net v0.1.0 // indirect
	golang.org/x/sync v0.3.0
	google.golang.org/genproto v0.0.0-20220602131408-e326c6e8e9c8 // indirect
	google.golang.org/grpc v1.47.0
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/UNO-SOFT/zlog v0.7.3
	github.com/godror/knownpb v0.1.1
	github.com/google/renameio/v2 v2.0.0
	github.com/peterbourgon/ff/v3 v3.4.0
	golang.org/x/exp v0.0.0-20230810033253-352e893a4cad
)

require (
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/term v0.11.0 // indirect
	golang.org/x/text v0.4.0 // indirect
)

//replace github.com/godror/godror => ../../godror/godror
