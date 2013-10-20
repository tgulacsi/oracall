package main

import (
	"flag"
	"log"
	"os"

	"github.com/tgulacsi/oracall/structs"
)

func main() {
	flag.Parse()
	if err := structs.ReadCsv(flag.Arg(0)); err != nil {
		log.Printf("error reading %q: %s", flag.Arg(0), err)
		os.Exit(1)
	}
}
