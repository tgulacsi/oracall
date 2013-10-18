package main

import (
	"flag"

	"github.com/tgulacsi/oracall/structs"
)

func main() {
	flag.Parse()
	structs.ReadCsv(flag.Arg(0))
}
