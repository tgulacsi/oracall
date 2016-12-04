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

/*
Package main for minimal is a minimal example for oracall usage

    oracall <one.csv >examples/minimal/generated_functions.go \
    && go fmt ./examples/minimal/ \
    && (cd examples/minimal/ && go build) \
    && ./examples/minimal/minimal DB_web.sendpreoffer_31101
*/
package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/rana/ora.v4"
)

var flagConnect = flag.String("connect", "", "Oracle database connection string")
var flagBypassMultipleArgs = flag.Bool("bypassmultipleargs", false, "bypass multiple args - experimental, probably worsens ORA-01008")

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatalf("at least one argument is needed: the function's name to be called")
	}
	if *flagConnect == "" {
		log.Fatalf("connect string is needed")
	}
	funName := flag.Arg(0)
	fun, ok := Functions[funName]
	if !ok {
		log.Fatalf("cannot find function named %q", funName)
	}
	log.Printf("fun to be called is %v", fun)

	// parse stdin as json into the proper input struct
	var (
		input []byte
		err   error
	)
	if flag.NArg() < 2 {
		if input, err = ioutil.ReadAll(os.Stdin); err != nil {
			log.Fatalf("error reading from stdin: %s", err)
		}
	} else {
		input = []byte(flag.Arg(1))
	}
	inp := InputFactories[funName]()
	if err = inp.FromJSON(input); err != nil {
		log.Fatalf("error unmarshaling %s into %T: %s", input, inp, err)
	}
	b, err := xml.Marshal(inp)
	if err != nil {
		log.Fatalf("error marshaling %v to xml: %s", inp, err)
	}
	log.Printf("input marshaled to xml: %s", b)

	DebugLevel = 1
	log.Printf("calling %s(%#v)", funName, inp)

	// get cursor
	env, srv, ses, err := ora.NewEnvSrvSes(*flagConnect, nil)
	if err != nil {
		log.Fatalf("error creating connection to %s: %s", *flagConnect, err)
	}
	defer env.Close()
	defer srv.Close()
	defer ses.Close()

	// call the function
	out, err := fun(ses, inp)
	if err != nil {
		log.Fatalf("error calling %s(%s): %s", funName, inp, err)
	}

	// present the output as json
	if b, err = json.Marshal(out); err != nil {
		log.Fatalf("error marshaling output: %s", err)
	}
	log.Printf("output marshaled to JSON: %s", b)

	if b, err = xml.Marshal(out); err != nil {
		log.Fatalf("error marshaling output to XML: %s", err)
	}
	log.Printf("output marshaled to XML: %s", b)
}
