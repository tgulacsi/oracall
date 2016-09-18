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
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"

	"gopkg.in/rana/ora.v3"
	"gopkg.in/rana/ora.v3/lg"
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

	ora.Cfg().Log.Logger = lg.Log
	pool, err := ora.NewPool(*flagConnect, 1)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	s := oracallServer{OraSesPool: pool}
	funV := reflect.ValueOf(&s).MethodByName(funName)
	log.Printf("fun to be called is %s", funName)

	userpass := strings.SplitN(*flagLogin, "/", 2)
	if len(userpass) < 2 {
		userpass = append(userpass, userpass[0])
	}
	sessionid, err := login(&s, userpass[0], userpass[1])
	if err != nil {
		log.Fatalf("error logging in (%s): %v", userpass[0], err)
	}
	if !*flagSkipLogout {
		defer logout(&s, sessionid)
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

	d := json.NewDecoder(bytes.NewReader(input))
	funInputV := reflect.Zero(funV.Type().In(1))
	inp := funInputV.Interface()
	if err := d.Decode(inp); err != nil {
		log.Fatalf("error unmarshaling %s into %T: %s", input, funInputV, err)
	}

	if err = StructSet(inp, "P_sessionid", sessionid); err != nil {
		log.Fatalf("error setting sessionid on %v: %v", inp, err)
	}

	DebugLevel = 1
	log.Printf("calling %s(%s)", funName, inp)

	// call the function
	resV := funV.Call([]reflect.Value{
		reflect.ValueOf(context.Background()),
		reflect.ValueOf(inp),
	})
	if err := resV[1].Interface().(error); err != nil {
		log.Fatalf("error calling %s(%#v): %s", funName, inp, err)
	}

	out := resV[0].Interface()
	// present the output as json
	b, err := json.Marshal(out)
	if err != nil {
		log.Fatalf("error marshaling output: %s", err)
	}
	log.Printf("output marshaled to JSON: %s", b)
}

func login(s *oracallServer, username, password string) (string, error) {
	lang := "hu"
	out, err := s.DBWeb_Login(context.Background(), &DbWeb_Login_Input{
		PLoginNev: username,
		PJelszo:   password,
		PLang:     lang,
	})
	if err != nil {
		return "", fmt.Errorf("DB_web.login: %v", err)
	}
	log.Printf("Login(%q): %#v", username, out)
	return out.PSessionid, nil
}

func logout(s *oracallServer, sessionID string) error {
	_, err := s.DBWeb_Logout(context.Background(), &DbWeb_Logout_Input{
		PSessionid: sessionID,
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
