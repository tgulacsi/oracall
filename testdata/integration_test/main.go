// Copyright 2017, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

/*
Package main for minimal is a minimal example for oracall usage

	oracall <one.csv >examples/minimal/generated_functions.go \
	&& go fmt ./examples/minimal/ \
	&& (cd examples/minimal/ && go build) \
	&& ./examples/minimal/minimal DB_web.sendpreoffer_31101
*/
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"reflect"
	"time"

	"github.com/davecgh/go-spew/spew"
	oracall "github.com/tgulacsi/oracall/lib"
	"golang.org/x/net/context"
)

var flagConnect = flag.String("connect", "", "Oracle database connection string")

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatalf("at least one argument is needed: the function's name to be called")
	}
	if *flagConnect == "" {
		log.Fatalf("connect string is needed")
	}
	db, err := sql.Open("godror", *flagConnect)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	srv := NewServer(db)
	funName := flag.Arg(0)
	rs := reflect.ValueOf(srv)
	rf := rs.MethodByName(funName)
	if !rf.IsValid() {
		rf = rs.MethodByName(oracall.CamelCase(funName))
	}
	if !rf.IsValid() {
		rt := rs.Type()
		methods := make([]string, rt.NumMethod())
		for i := range methods {
			methods[i] = rt.Method(i).Name
		}
		log.Fatalf("cannot find function named %q/%q, only %q", funName, oracall.CamelCase(funName), methods)
	}
	log.Printf("fun to be called is %s", rf)

	// parse stdin as json into the proper input struct
	var input []byte
	if flag.NArg() < 2 {
		if input, err = io.ReadAll(os.Stdin); err != nil {
			log.Fatalf("error reading from stdin: %s", err)
		}
	} else {
		input = []byte(flag.Arg(1))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	rft := rf.Type()
	args := make([]reflect.Value, 0, rft.NumIn())
	if rft.In(0).Name() == "Context" {
		args = append(args, reflect.ValueOf(ctx))
	}
	rinp := reflect.New(rft.In(len(args)).Elem())
	args = append(args, rinp)
	if err := json.Unmarshal(input, rinp.Interface()); err != nil {
		log.Fatalf("error unmarshaling %s into %T: %s", input, rinp.Interface(), err)
	}
	inp := rinp.Interface()
	log.Printf("calling %s(%#v)", funName, inp)

	// get cursor

	// call the function
	outs := rf.Call(args)
	log.Printf("outs: %+v", outs)
	err, _ = outs[1].Interface().(error)
	if err != nil {
		log.Fatalf("error calling %s(%#v): %v", funName, inp, err)
	}

	out := outs[0].Interface()
	log.Printf("outs: (%T,%T) (%+v, %+v)", out, err, out, err)
	// present the output as json
	err = json.NewEncoder(os.Stdout).Encode(out)
	os.Stdout.Close()
	if err != nil {
		log.Fatalf("error marshaling output: %s\n%+v\n%s", err, out,
			spew.Sdump(out))
	}
}
