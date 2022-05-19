module github.com/tgulacsi/oracall

go 1.17

require (
	github.com/antzucaro/matchr v0.0.0-20210222213004-b04723ef80f0
	github.com/fatih/structs v1.1.0
	github.com/go-stack/stack v1.8.1
	github.com/godror/godror v0.33.1
	github.com/google/go-cmp v0.5.8
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0-rc.2
	github.com/kylelemons/godebug v1.1.0
	github.com/oklog/ulid v1.3.1
	github.com/tgulacsi/go v0.20.2
	golang.org/x/net v0.0.0-20210917221730-978cfadd31cf // indirect
	golang.org/x/sync v0.0.0-20220513210516-0976fa681c29
	google.golang.org/genproto v0.0.0-20210917145530-b395a37504d4 // indirect
	google.golang.org/grpc v1.46.2
	google.golang.org/protobuf v1.28.0
)

require (
	github.com/go-logr/logr v1.2.3
	github.com/go-logr/zerologr v1.2.1
	github.com/godror/knownpb v0.1.0
	github.com/google/renameio/v2 v2.0.0
	github.com/rs/zerolog v1.26.1
)

require (
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	golang.org/x/sys v0.0.0-20210917161153-d61c044b1678 // indirect
	golang.org/x/text v0.3.7 // indirect
)

//replace github.com/godror/godror => ../../godror/godror
