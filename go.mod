module github.com/tgulacsi/oracall

go 1.15

require (
	github.com/antzucaro/matchr v0.0.0-20180616170659-cbc221335f3c
	github.com/dvyukov/go-fuzz v0.0.0-20201003075337-90825f39c90b // indirect
	github.com/fatih/structs v1.1.0
	github.com/go-kit/kit v0.10.0
	github.com/go-stack/stack v1.8.0
	github.com/godror/godror v0.22.3
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/google/go-cmp v0.5.1
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/kylelemons/godebug v1.1.0
	github.com/oklog/ulid v1.3.1
	github.com/stretchr/testify v1.6.1 // indirect
	github.com/tgulacsi/go v0.13.4
	golang.org/x/net v0.0.0-20200707034311-ab3426394381 // indirect
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/sys v0.0.0-20200806060901-a37d78b92225 // indirect
	golang.org/x/text v0.3.3 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20200804151602-45615f50871c // indirect
	google.golang.org/grpc v1.31.0
	google.golang.org/protobuf v1.25.0
)

//replace github.com/godror/godror => ../../godror/godror
