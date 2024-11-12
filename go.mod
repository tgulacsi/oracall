module github.com/tgulacsi/oracall

go 1.23.0

require (
	github.com/UNO-SOFT/zlog v0.8.3
	github.com/antzucaro/matchr v0.0.0-20221106193745-7bed6ef61ef9
	github.com/fatih/structs v1.1.0
	github.com/go-stack/stack v1.8.1
	github.com/godror/godror v0.44.1
	github.com/google/go-cmp v0.6.0
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.1.0
	github.com/kylelemons/godebug v1.1.0
	github.com/tgulacsi/go v0.27.5
	golang.org/x/net v0.27.0
	golang.org/x/sync v0.7.0
	google.golang.org/grpc v1.65.0
	google.golang.org/protobuf v1.34.2
)

require (
	github.com/UNO-SOFT/otel v0.8.6
	github.com/davecgh/go-spew v1.1.1
	github.com/godror/knownpb v0.1.2
	github.com/google/renameio/v2 v2.0.0
	github.com/oklog/ulid/v2 v2.1.0
	github.com/peterbourgon/ff/v3 v3.4.0
	github.com/pkg/errors v0.9.1
)

require (
	github.com/dgryski/go-linebreak v0.0.0-20180812204043-d8f37254e7d3 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.28.0 // indirect
	go.opentelemetry.io/otel/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/sdk v1.28.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.28.0 // indirect
	go.opentelemetry.io/otel/trace v1.28.0 // indirect
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56 // indirect
	golang.org/x/sys v0.22.0 // indirect
	golang.org/x/term v0.22.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240701130421-f6361c86f094 // indirect
)

replace github.com/godror/godror => ../../godror/godror

//replace github.com/UNO-SOFT/zlog => ../../UNO-SOFT/zlog
