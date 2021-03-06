module github.com/tgulacsi/oracall

go 1.15

require (
	github.com/antzucaro/matchr v0.0.0-20180616170659-cbc221335f3c
	github.com/davecgh/go-spew v1.1.1
	github.com/fatih/structs v1.1.0
	github.com/go-kit/kit v0.10.0
	github.com/go-stack/stack v1.8.0
	github.com/godror/godror v0.25.3
	github.com/gogo/protobuf v1.3.1
	github.com/google/go-cmp v0.5.5
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/kortschak/utter v1.0.1 // indirect
	github.com/kylelemons/godebug v1.1.0
	github.com/oklog/ulid v1.3.1
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.6.1 // indirect
	github.com/tgulacsi/go v0.18.4
	golang.org/x/net v0.0.0-20210316092652-d523dce5a7f4
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/genproto v0.0.0-20210506142907-4a47615972c2 // indirect
	google.golang.org/grpc v1.36.1
	google.golang.org/protobuf v1.27.1
)

//replace github.com/godror/godror => ../../godror/godror
