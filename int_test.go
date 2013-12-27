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
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/tgulacsi/goracle/oracle"
)

// TestGen tests the generation - for this, it needs a dsn with privileges
// if you get "ORA-01031: insufficient privileges", then you need
// GRANT CREATE PROCEDURE TO username;
func TestGen(t *testing.T) {
	conn := getConnection(t)

	cu := conn.NewCursor()
	defer cu.Close()

	err := cu.Execute(`CREATE OR REPLACE PACKAGE TST_oracall AS
PROCEDURE char_in(txt IN VARCHAR2);
FUNCTION char_out RETURN VARCHAR2;
PROCEDURE num_in(num IN NUMBER);
PROCEDURE date_in(dat IN DATE);
FUNCTION char_in_char_ret(txt IN VARCHAR2) RETURN VARCHAR2;
END TST_oracall;
    `, nil, nil)
	if err != nil {
		t.Fatalf("error creating package head: %v", err)
	}
	if err = cu.Execute(`CREATE OR REPLACE PACKAGE BODY TST_oracall AS
PROCEDURE char_in(txt IN VARCHAR2) IS
  v_txt VARCHAR2(1000) := SUBSTR(txt, 1, 100);
BEGIN NULL; END char_in;
FUNCTION char_out RETURN VArCHAR2 IS BEGIN RETURN('A'); END char_out;

PROCEDURE num_in(num IN NUMBER) IS
  v_num NUMBER := num;
BEGIN NULL; END num_in;

PROCEDURE date_in(dat IN DATE) IS
  v_dat DATE := dat;
BEGIN NULL; END date_in;

FUNCTION char_in_char_ret(txt IN VARCHAR2) RETURN VARCHAR2 IS
  v_txt CONSTANT VARCHAR2(4000) := SUBSTR(txt, 1, 4000);
  v_ret VARCHAR2(4000);
BEGIN
  SELECT DUMP(txt) INTO v_ret FROM DUAL;
  RETURN v_ret;
END char_in_char_ret;

END TST_oracall;
    `, nil, nil); err != nil {
		t.Fatalf("error creating package body: %v", err)
	}

	var (
		out   []byte
		outFn string
	)
	for _, command := range []string{
		"go build",
		"./oracall -F -connect='" + *dsn + "' TST_ORACALL.% > ./testdata/integration_test/generated_functions.go",
		"go build ./testdata/integration_test",
		"integration_test -connect='" + *dsn + "' TST_oracall.char_in '{\"txt\":\"abraka dabra\"}'",
		"integration_test -connect='" + *dsn + "' TST_oracall.char_out '{}'",
		"integration_test -connect='" + *dsn + "' TST_oracall.num_in '{\"num\": 3, \"txt\":\"abraka dabra\"}'",
		"integration_test -connect='" + *dsn + "' TST_oracall.date_in '{\"dat\": \"2013-12-25T21:15:00+01:00\", \"txt\":\"abraka dabra\"}'",
		"integration_test -connect='" + *dsn + "' TST_oracall.char_in_char_ret '{\"txt\":\"abraka dabra\"}'",
	} {
		if command == "go build ./testdata/integration_test" {
			if outFh, err := ioutil.TempFile("", "oracall-integration_test"); err != nil {
				t.Errorf("cannot create temp file: %v", err)
				t.FailNow()
			} else {
				outFn = outFh.Name()
				outFh.Close()
			}
			os.Remove(outFn)
			command = command[:8] + " -o " + outFn + command[8:]
		} else if strings.HasPrefix(command, "integration_test ") {
			command = outFn + " " + command[16:]
			defer os.Remove(outFn)
		}
		if out, err = exec.Command("sh", "-c", command).CombinedOutput(); err != nil {
			t.Errorf("error '%s': %v\n%s", command, err, out)
			t.FailNow()
		} else {
			t.Logf("%s:\n%s", command, out)
		}
	}
}

var dsn = flag.String("dsn", "", "Oracle DSN (user/passw@sid)")
var dbg = flag.Bool("debug", false, "print debug messages?")

func init() {
	flag.Parse()
}

var conn oracle.Connection

func getConnection(t *testing.T) oracle.Connection {
	if conn.IsConnected() {
		return conn
	}

	if !(dsn != nil && *dsn != "") {
		t.Logf("cannot test connection without dsn!")
		return conn
	}
	user, passw, sid := oracle.SplitDSN(*dsn)
	var err error
	conn, err = oracle.NewConnection(user, passw, sid, false)
	if err != nil {
		log.Panicf("error creating connection to %s: %s", *dsn, err)
	}
	if err = conn.Connect(0, false); err != nil {
		log.Panicf("error connecting: %s", err)
	}
	return conn
}
