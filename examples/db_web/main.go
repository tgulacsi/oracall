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
Package main for db_web is a db_web example for oracall usage

    oracall -connect="$dsn" DB_WEB.% >examples/db_web/generated_functions.go \
    && go fmt ./examples/db_web/ \
    && (cd examples/db_web/ && go build) \
    && ./examples/db_web/db_web DB_web.sendpreoffer_31101
*/
package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"

	"gopkg.in/goracle.v1/oracle"
)

var (
	flagConnect    = flag.String("connect", "", "Oracle database connection string")
	flagLogin      = flag.String("login", "", "username/password to call DB_web.login with")
	flagSkipLogout = flag.Bool("skip-logout", false, "skip log out at the end")
)

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		log.Fatalf("at least one argument is needed: the function's name to be called")
	}
	if *flagConnect == "" {
		log.Fatalf("connect string is needed")
	}
	if *flagLogin == "" {
		log.Fatalf("login string (login/password) is needed")
	}
	funName := flag.Arg(0)
	fun, ok := Functions[funName]
	if !ok {
		log.Fatalf("cannot find function named %q", funName)
	}
	log.Printf("fun to be called is %s", funName)

	// get cursor
	user, passw, sid := oracle.SplitDSN(*flagConnect)
	conn, err := oracle.NewConnection(user, passw, sid, false)
	if err != nil {
		log.Fatalf("error creating connection to %s: %s", *flagConnect, err)
	}
	if err = conn.Connect(0, false); err != nil {
		log.Fatalf("error connecting: %s", err)
	}
	defer conn.Close()
	cur := oracle.NewCursor(conn)
	defer cur.Close()

	userpass := strings.SplitN(*flagLogin, "/", 2)
	if len(userpass) < 2 {
		userpass = append(userpass, userpass[0])
	}
	sessionid, err := login(cur, userpass[0], userpass[1])
	if err != nil {
		log.Fatalf("error logging in (%s): %v", userpass[0], err)
	}
	if !*flagSkipLogout {
		defer logout(cur, sessionid)
	}

	// parse stdin as json into the proper input struct
	var input []byte
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

	if err = StructSet(inp, "P_sessionid", sessionid); err != nil {
		log.Fatalf("error setting sessionid on %v: %v", inp, err)
	}

	b, err := xml.Marshal(inp)
	if err != nil {
		log.Fatalf("error marshaling %v to xml: %s", inp, err)
	}
	log.Printf("input marshaled to xml: %s", b)

	DebugLevel = 1
	log.Printf("calling %s(%s)", funName, inp)

	// call the function
	out, err := fun(cur, inp)
	if err != nil {
		log.Fatalf("error calling %s(%#v): %s", funName, inp, err)
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

func login(cur *oracle.Cursor, username, password string) (string, error) {
	lang, addr := "hu", "127.0.0.1"
	out, err := Functions["DB_web.login"](cur, Db_web__login__input{
		P_login_nev: username,
		P_jelszo:    password,
		P_lang:      lang,
		P_addr匿:     addr,
	})
	if err != nil {
		return "", fmt.Errorf("DB_web.login: %v", err)
	}
	log.Printf("Login(%q): %#v", username, out.(Db_web__login__output))
	return (out.(Db_web__login__output).P_sessionid), nil
}

func logout(cur *oracle.Cursor, sessionID string) error {
	_, err := Functions["DB_web.logout"](cur, Db_web__logout__input{
		P_sessionid: sessionID,
	})
	return err
}

func StructSet(st interface{}, key string, value interface{}) error {
	v := reflect.ValueOf(st)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	fv := v.FieldByName(key)
	if fv.Kind() == reflect.Ptr {
		fv = fv.Elem()
	}
	fv.Set(reflect.ValueOf(value))
	return nil
}
