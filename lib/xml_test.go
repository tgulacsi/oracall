/*
Copyright 2015 Tamás Gulácsi

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

package oracall

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
)

func TestQuery078(t *testing.T) {
	Log = func(keyvals ...interface{}) error {
		var buf strings.Builder
		var tmp strings.Builder
		for i := 0; i < len(keyvals); i += 2 {
			tmp.Reset()
			fmt.Fprintf(&tmp, "%+v", keyvals[i+1])
			v := strings.ReplaceAll(tmp.String(), "\"", "\\\"")
			if strings.Contains(v, " ") {
				fmt.Fprintf(&buf, "%s=\"%s\" ", keyvals[i], v)
			} else {
				fmt.Fprintf(&buf, "%s=%s ", keyvals[i], v)
			}
		}
		t.Log(buf.String())
		return nil
	}
	functions, err := ParseCsv(strings.NewReader(query078Csv), nil)
	if err != nil {
		t.Errorf("error parsing csv: %v", err)
		t.FailNow()
	}
	if len(functions) != 1 {
		t.Errorf("parsed %d functions, wanted %d!", len(functions), 1)
	}
	b, err := xml.Marshal(functions[0])
	if err != nil {
		t.Fatal(functions[0], err)
	}
	t.Logf("functions: %s", b)
	var buf strings.Builder
	if err = SaveProtobuf(&buf, functions, "spl3"); err != nil {
		t.Fatal(err)
	}
	t.Log(buf.String())
}

const query078Csv = `OBJECT_ID,SUBPROGRAM_ID,SEQUENCE,PACKAGE_NAME,OBJECT_NAME,DATA_LEVEL,POSITION,ARGUMENT_NAME,IN_OUT,DATA_TYPE,DATA_PRECISION,DATA_SCALE,CHARACTER_SET_NAME,PLS_TYPE,CHAR_LENGTH,TYPE_OWNER,TYPE_NAME,TYPE_SUBNAME,TYPE_LINK
35325,81,1,DB_SPOOLSYS3,QUERY_078,0,1,P_SZERZ_AZON,IN,NUMBER,9,,,NUMBER,0,,,,
35325,81,2,DB_SPOOLSYS3,QUERY_078,0,2,P_OUTPUT,OUT,PL/SQL TABLE,,,,,0,BRUNO_OWNER,DB_SPOOLSYS3,TYPE_OUTLIST_078,
35325,81,3,DB_SPOOLSYS3,QUERY_078,1,1,,OUT,PL/SQL RECORD,,,,,0,BRUNO_OWNER,DB_SPOOLSYS3,TYPE_OUTPUT_078,
35325,81,4,DB_SPOOLSYS3,QUERY_078,2,1,TRANZ_KEZDETE,OUT,DATE,,,,DATE,0,,,,
35325,81,5,DB_SPOOLSYS3,QUERY_078,2,2,TRANZ_VEGE,OUT,DATE,,,,DATE,0,,,,
35325,81,6,DB_SPOOLSYS3,QUERY_078,2,3,KOLTSEG,OUT,NUMBER,12,5,,NUMBER,0,,,,
35325,81,7,DB_SPOOLSYS3,QUERY_078,2,4,ERTEKESITETT_ALAPOK,OUT,PL/SQL TABLE,,,,,0,BRUNO_OWNER,DB_SPOOLSYS3,ATYPE_OUTLIST_UNIT,
35325,81,8,DB_SPOOLSYS3,QUERY_078,3,1,,OUT,PL/SQL RECORD,,,,,0,BRUNO_OWNER,DB_SPOOLSYS3,ATYPE_OUTPUT_UNIT,
35325,81,9,DB_SPOOLSYS3,QUERY_078,4,1,F_UNIT_RNEV,OUT,VARCHAR2,,,CHAR_CS,VARCHAR2,6,,,,
35325,81,10,DB_SPOOLSYS3,QUERY_078,4,2,F_UNIT_NEV,OUT,VARCHAR2,,,CHAR_CS,VARCHAR2,40,,,,
35325,81,11,DB_SPOOLSYS3,QUERY_078,4,3,F_ISIN,OUT,VARCHAR2,,,CHAR_CS,VARCHAR2,12,,,,
35325,81,12,DB_SPOOLSYS3,QUERY_078,4,4,UNIT_DB,OUT,NUMBER,24,12,,NUMBER,0,,,,
35325,81,13,DB_SPOOLSYS3,QUERY_078,4,5,UNIT_ARF,OUT,NUMBER,24,12,,NUMBER,0,,,,
35325,81,14,DB_SPOOLSYS3,QUERY_078,2,5,VASAROLT_ALAPOK,OUT,PL/SQL TABLE,,,,,0,BRUNO_OWNER,DB_SPOOLSYS3,ATYPE_OUTLIST_UNIT,
35325,81,15,DB_SPOOLSYS3,QUERY_078,3,1,,OUT,PL/SQL RECORD,,,,,0,BRUNO_OWNER,DB_SPOOLSYS3,ATYPE_OUTPUT_UNIT,
35325,81,16,DB_SPOOLSYS3,QUERY_078,4,1,F_UNIT_RNEV,OUT,VARCHAR2,,,CHAR_CS,VARCHAR2,6,,,,
35325,81,17,DB_SPOOLSYS3,QUERY_078,4,2,F_UNIT_NEV,OUT,VARCHAR2,,,CHAR_CS,VARCHAR2,40,,,,
35325,81,18,DB_SPOOLSYS3,QUERY_078,4,3,F_ISIN,OUT,VARCHAR2,,,CHAR_CS,VARCHAR2,12,,,,
35325,81,19,DB_SPOOLSYS3,QUERY_078,4,4,UNIT_DB,OUT,NUMBER,24,12,,NUMBER,0,,,,
35325,81,20,DB_SPOOLSYS3,QUERY_078,4,5,UNIT_ARF,OUT,NUMBER,24,12,,NUMBER,0,,,,`
