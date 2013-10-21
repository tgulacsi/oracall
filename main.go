package main

import (
	"flag"
	"log"
	"os"

	"github.com/tgulacsi/oracall/structs"
)

func main() {
	flag.Parse()
	functions, err := structs.ReadCsv(flag.Arg(0))
	if err != nil {
		log.Printf("error reading %q: %s", flag.Arg(0), err)
		os.Exit(1)
	}

	defer os.Stdout.Sync()
	if err = structs.SaveFunctions(os.Stdout, functions, "main"); err != nil {
		log.Printf("error saving functions: %s", err)
		os.Exit(1)
	}
}
