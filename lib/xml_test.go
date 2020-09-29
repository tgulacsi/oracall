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

	"github.com/google/go-cmp/cmp"
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
	if len(functions) != 2 {
		t.Errorf("parsed %d functions, wanted %d!", len(functions), 1)
	}
	functions[0].Replacement = &functions[1]
	var buf strings.Builder
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(functions[0]); err != nil {
		t.Fatal(functions[0], err)
	}
	if d := cmp.Diff(strings.Split(buf.String(), "\n"), strings.Split(query078WantXML, "\n")); d != "" {
		t.Error(d)
	}

	buf.Reset()
	if err = SaveProtobuf(&buf, functions, "spl3", "unosoft.hu/ws/aeg/pb/spl3"); err != nil {
		t.Fatal(err)
	}
	t.Log(buf.String())

	buf.Reset()
	err = SaveFunctions(&buf, functions[:1], "DB_spoolsys3", "unosoft.hu/ws/aeg/pb", true)
	t.Log(buf.String())
	if err != nil {
		t.Error(err)
	}
	if strings.Contains(buf.String(), "PL/SQLTABLE_tab_typ;") {
		t.Error("PL/SQLTABLE_tab_typ is nonsense!")
	}
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
35325,81,20,DB_SPOOLSYS3,QUERY_078,4,5,UNIT_ARF,OUT,NUMBER,24,12,,NUMBER,0,,,,
35325,82,1,DB_SPOOLSYS3,QUERY_078_XML,0,1,P_OUT,OUT,,,,,XMLTYPE,0,,,,
35325,82,1,DB_SPOOLSYS3,QUERY_078_XML,0,2,P_IN,IN,,,,,XMLTYPE,0,,,,
`

const query078WantXML = `<Function>
  <Package>DB_SPOOLSYS3</Package>
  <Args>
    <Name>p_szerz_azon</Name>
    <Type>NUMBER</Type>
    <TypeName></TypeName>
    <AbsType>NUMBER(9)</AbsType>
    <Charset></Charset>
    <Charlength>0</Charlength>
    <Flavor>SIMPLE</Flavor>
    <Direction>IN</Direction>
    <Precision>9</Precision>
    <Scale>0</Scale>
  </Args>
  <Args>
    <Name>p_output</Name>
    <Type>PL/SQL TABLE</Type>
    <TypeName>BRUNO_OWNER.DB_SPOOLSYS3.TYPE_OUTLIST_078</TypeName>
    <AbsType>PL/SQL TABLE</AbsType>
    <Charset></Charset>
    <Charlength>0</Charlength>
    <TableOf>
      <RecordOf>
        <Name>tranz_kezdete</Name>
        <Type>DATE</Type>
        <TypeName></TypeName>
        <AbsType>DATE</AbsType>
        <Charset></Charset>
        <Charlength>0</Charlength>
        <Flavor>SIMPLE</Flavor>
        <Direction>OUT</Direction>
        <Precision>0</Precision>
        <Scale>0</Scale>
      </RecordOf>
      <RecordOf>
        <Name>tranz_vege</Name>
        <Type>DATE</Type>
        <TypeName></TypeName>
        <AbsType>DATE</AbsType>
        <Charset></Charset>
        <Charlength>0</Charlength>
        <Flavor>SIMPLE</Flavor>
        <Direction>OUT</Direction>
        <Precision>0</Precision>
        <Scale>0</Scale>
      </RecordOf>
      <RecordOf>
        <Name>koltseg</Name>
        <Type>NUMBER</Type>
        <TypeName></TypeName>
        <AbsType>NUMBER(12, 5)</AbsType>
        <Charset></Charset>
        <Charlength>0</Charlength>
        <Flavor>SIMPLE</Flavor>
        <Direction>OUT</Direction>
        <Precision>12</Precision>
        <Scale>5</Scale>
      </RecordOf>
      <RecordOf>
        <Name>ertekesitett_alapok</Name>
        <Type>PL/SQL TABLE</Type>
        <TypeName>BRUNO_OWNER.DB_SPOOLSYS3.ATYPE_OUTLIST_UNIT</TypeName>
        <AbsType>PL/SQL TABLE</AbsType>
        <Charset></Charset>
        <Charlength>0</Charlength>
        <TableOf>
          <RecordOf>
            <Name>f_unit_rnev</Name>
            <Type>VARCHAR2</Type>
            <TypeName></TypeName>
            <AbsType>VARCHAR2(6)</AbsType>
            <Charset>CHAR_CS</Charset>
            <Charlength>6</Charlength>
            <Flavor>SIMPLE</Flavor>
            <Direction>OUT</Direction>
            <Precision>0</Precision>
            <Scale>0</Scale>
          </RecordOf>
          <RecordOf>
            <Name>f_unit_nev</Name>
            <Type>VARCHAR2</Type>
            <TypeName></TypeName>
            <AbsType>VARCHAR2(40)</AbsType>
            <Charset>CHAR_CS</Charset>
            <Charlength>40</Charlength>
            <Flavor>SIMPLE</Flavor>
            <Direction>OUT</Direction>
            <Precision>0</Precision>
            <Scale>0</Scale>
          </RecordOf>
          <RecordOf>
            <Name>f_isin</Name>
            <Type>VARCHAR2</Type>
            <TypeName></TypeName>
            <AbsType>VARCHAR2(12)</AbsType>
            <Charset>CHAR_CS</Charset>
            <Charlength>12</Charlength>
            <Flavor>SIMPLE</Flavor>
            <Direction>OUT</Direction>
            <Precision>0</Precision>
            <Scale>0</Scale>
          </RecordOf>
          <RecordOf>
            <Name>unit_db</Name>
            <Type>NUMBER</Type>
            <TypeName></TypeName>
            <AbsType>NUMBER(24, 12)</AbsType>
            <Charset></Charset>
            <Charlength>0</Charlength>
            <Flavor>SIMPLE</Flavor>
            <Direction>OUT</Direction>
            <Precision>24</Precision>
            <Scale>12</Scale>
          </RecordOf>
          <RecordOf>
            <Name>unit_arf</Name>
            <Type>NUMBER</Type>
            <TypeName></TypeName>
            <AbsType>NUMBER(24, 12)</AbsType>
            <Charset></Charset>
            <Charlength>0</Charlength>
            <Flavor>SIMPLE</Flavor>
            <Direction>OUT</Direction>
            <Precision>24</Precision>
            <Scale>12</Scale>
          </RecordOf>
          <Name></Name>
          <Type>PL/SQL RECORD</Type>
          <TypeName>BRUNO_OWNER.DB_SPOOLSYS3.ATYPE_OUTPUT_UNIT</TypeName>
          <AbsType>PL/SQL RECORD</AbsType>
          <Charset></Charset>
          <Charlength>0</Charlength>
          <Flavor>RECORD</Flavor>
          <Direction>OUT</Direction>
          <Precision>0</Precision>
          <Scale>0</Scale>
        </TableOf>
        <Flavor>TABLE</Flavor>
        <Direction>OUT</Direction>
        <Precision>0</Precision>
        <Scale>0</Scale>
      </RecordOf>
      <RecordOf>
        <Name>vasarolt_alapok</Name>
        <Type>PL/SQL TABLE</Type>
        <TypeName>BRUNO_OWNER.DB_SPOOLSYS3.ATYPE_OUTLIST_UNIT</TypeName>
        <AbsType>PL/SQL TABLE</AbsType>
        <Charset></Charset>
        <Charlength>0</Charlength>
        <TableOf>
          <RecordOf>
            <Name>f_unit_rnev</Name>
            <Type>VARCHAR2</Type>
            <TypeName></TypeName>
            <AbsType>VARCHAR2(6)</AbsType>
            <Charset>CHAR_CS</Charset>
            <Charlength>6</Charlength>
            <Flavor>SIMPLE</Flavor>
            <Direction>OUT</Direction>
            <Precision>0</Precision>
            <Scale>0</Scale>
          </RecordOf>
          <RecordOf>
            <Name>f_unit_nev</Name>
            <Type>VARCHAR2</Type>
            <TypeName></TypeName>
            <AbsType>VARCHAR2(40)</AbsType>
            <Charset>CHAR_CS</Charset>
            <Charlength>40</Charlength>
            <Flavor>SIMPLE</Flavor>
            <Direction>OUT</Direction>
            <Precision>0</Precision>
            <Scale>0</Scale>
          </RecordOf>
          <RecordOf>
            <Name>f_isin</Name>
            <Type>VARCHAR2</Type>
            <TypeName></TypeName>
            <AbsType>VARCHAR2(12)</AbsType>
            <Charset>CHAR_CS</Charset>
            <Charlength>12</Charlength>
            <Flavor>SIMPLE</Flavor>
            <Direction>OUT</Direction>
            <Precision>0</Precision>
            <Scale>0</Scale>
          </RecordOf>
          <RecordOf>
            <Name>unit_db</Name>
            <Type>NUMBER</Type>
            <TypeName></TypeName>
            <AbsType>NUMBER(24, 12)</AbsType>
            <Charset></Charset>
            <Charlength>0</Charlength>
            <Flavor>SIMPLE</Flavor>
            <Direction>OUT</Direction>
            <Precision>24</Precision>
            <Scale>12</Scale>
          </RecordOf>
          <RecordOf>
            <Name>unit_arf</Name>
            <Type>NUMBER</Type>
            <TypeName></TypeName>
            <AbsType>NUMBER(24, 12)</AbsType>
            <Charset></Charset>
            <Charlength>0</Charlength>
            <Flavor>SIMPLE</Flavor>
            <Direction>OUT</Direction>
            <Precision>24</Precision>
            <Scale>12</Scale>
          </RecordOf>
          <Name></Name>
          <Type>PL/SQL RECORD</Type>
          <TypeName>BRUNO_OWNER.DB_SPOOLSYS3.ATYPE_OUTPUT_UNIT</TypeName>
          <AbsType>PL/SQL RECORD</AbsType>
          <Charset></Charset>
          <Charlength>0</Charlength>
          <Flavor>RECORD</Flavor>
          <Direction>OUT</Direction>
          <Precision>0</Precision>
          <Scale>0</Scale>
        </TableOf>
        <Flavor>TABLE</Flavor>
        <Direction>OUT</Direction>
        <Precision>0</Precision>
        <Scale>0</Scale>
      </RecordOf>
      <Name></Name>
      <Type>PL/SQL RECORD</Type>
      <TypeName>BRUNO_OWNER.DB_SPOOLSYS3.TYPE_OUTPUT_078</TypeName>
      <AbsType>PL/SQL RECORD</AbsType>
      <Charset></Charset>
      <Charlength>0</Charlength>
      <Flavor>RECORD</Flavor>
      <Direction>OUT</Direction>
      <Precision>0</Precision>
      <Scale>0</Scale>
    </TableOf>
    <Flavor>TABLE</Flavor>
    <Direction>OUT</Direction>
    <Precision>0</Precision>
    <Scale>0</Scale>
  </Args>
  <Documentation></Documentation>
</Function>`
