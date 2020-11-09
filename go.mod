module github.com/tgulacsi/oracall

go 1.12

require (
	github.com/antzucaro/matchr v0.0.0-20180616170659-cbc221335f3c
	github.com/davecgh/go-spew v1.1.1
	github.com/fatih/structs v1.1.0
	github.com/go-kit/kit v0.10.0
	github.com/go-stack/stack v1.8.0
	github.com/godror/godror v0.20.6
	github.com/gogo/protobuf v1.3.1
	github.com/google/go-cmp v0.5.1
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/kylelemons/godebug v1.1.0
	github.com/oklog/ulid v1.3.1
	github.com/pkg/errors v0.9.1
	github.com/tgulacsi/go v0.13.2
	go.opentelemetry.io/otel/exporters/stdout v0.10.0 // indirect
	golang.org/dl v0.0.0-20200724191219-e4fbcf8a7a81 // indirect
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/sys v0.0.0-20200806060901-a37d78b92225 // indirect
	golang.org/x/text v0.3.3 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	google.golang.org/genproto v0.0.0-20200804151602-45615f50871c // indirect
	google.golang.org/grpc v1.31.0
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/errgo.v1 v1.0.1 // indirect
	gopkg.in/inconshreveable/log15.v2 v2.0.0-20200109203555-b30bc20e4fd1 // indirect
)

//replace github.com/godror/godror => ../../godror/godror
