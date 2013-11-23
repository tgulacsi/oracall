/*
Copyright 2013 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"log"
	"os"

	"github.com/tgulacsi/oracall/structs"
)

var flagSkipFormat = flag.Bool("F", false, "skip formatting")

func main() {
	flag.Parse()
	functions, err := structs.ReadCsv(flag.Arg(0))
	if err != nil {
		log.Printf("error reading %q: %s", flag.Arg(0), err)
		os.Exit(1)
	}

	defer os.Stdout.Sync()
	if err = structs.SaveFunctions(os.Stdout, functions, "main", *flagSkipFormat); err != nil {
		log.Printf("error saving functions: %s", err)
		os.Exit(1)
	}
}
