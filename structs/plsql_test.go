package structs

import (
	"strings"
	"testing"

	"github.com/kylelemons/godebug/diff"
)

const csvSource = `OBJECT_ID;SUBPROGRAM_ID;PACKAGE_NAME;OBJECT_NAME;DATA_LEVEL;POSITION;ARGUMENT_NAME;IN_OUT;DATA_TYPE;DATA_PRECISION;DATA_SCALE;CHARACTER_SET_NAME;PLS_TYPE;CHAR_LENGTH;TYPE_LINK;TYPE_OWNER;TYPE_NAME;TYPE_SUBNAME
19734;35;DB_WEB;SENDPREOFFER_31101;0;1;P_SESSIONID;IN;VARCHAR2;;;CHAR_CS;VARCHAR2;;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;2;P_LANG;IN;VARCHAR2;;;CHAR_CS;VARCHAR2;;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;3;P_VEGLEGES;IN;VARCHAR2;;;CHAR_CS;VARCHAR2;;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;4;P_ELSO_CSEKK_ATADVA;IN;VARCHAR2;;;CHAR_CS;VARCHAR2;;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;5;P_VONALKOD;IN/OUT;BINARY_INTEGER;;;;PLS_INTEGER;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;6;P_KOTVENY;IN/OUT;PL/SQL RECORD;;;;;0;;BRUNO;DB_WEB_ELEKTR;KOTVENY_REC_TYP
19734;35;DB_WEB;SENDPREOFFER_31101;1;1;DIJKOD;IN/OUT;CHAR;;;CHAR_CS;CHAR;2;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;2;DIJFIZMOD;IN/OUT;CHAR;;;CHAR_CS;CHAR;1;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;3;DIJFIZGYAK;IN/OUT;CHAR;;;CHAR_CS;CHAR;1;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;4;SZERKOT;IN/OUT;DATE;;;;DATE;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;5;SZERLEJAR;IN/OUT;DATE;;;;DATE;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;6;KOCKEZD;IN/OUT;DATE;;;;DATE;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;7;BTKEZD;IN/OUT;DATE;;;;DATE;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;8;HALASZT_KOCKEZD;IN/OUT;DATE;;;;DATE;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;9;HALASZT_DIJFIZ;IN/OUT;DATE;;;;DATE;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;10;SZAMLASZAM;IN/OUT;VARCHAR2;;;CHAR_CS;VARCHAR2;24;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;11;SZAMLA_LIMIT;IN/OUT;NUMBER;12;2;;NUMBER;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;12;EVFORDULO;IN/OUT;DATE;;;;DATE;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;13;EVFORDULO_TIPUS;IN/OUT;VARCHAR2;;;CHAR_CS;VARCHAR2;1;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;14;E_KOMM_EMAIL;IN/OUT;VARCHAR2;;;CHAR_CS;VARCHAR2;80;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;15;DIJBEKEROT_KER;IN/OUT;VARCHAR2;;;CHAR_CS;VARCHAR2;1;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;1;16;AJANLATI_EVESDIJ;IN/OUT;NUMBER;12;2;;NUMBER;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;16;P_KEDVEZMENYEK;IN;PL/SQL TABLE;;;;;0;;BRUNO;DB_WEB_ELEKTR;KEDVEZMENY_TAB_TYP
19734;35;DB_WEB;SENDPREOFFER_31101;1;1;;IN;VARCHAR2;;;CHAR_CS;VARCHAR2;6;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;17;P_DUMP_ARGS#;IN;VARCHAR2;;;CHAR_CS;VARCHAR2;;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;18;P_SZERZ_AZON;OUT;BINARY_INTEGER;;;;PLS_INTEGER;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;19;P_AJANLAT_URL;OUT;VARCHAR2;;;CHAR_CS;VARCHAR2;;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;20;P_SZAMOLT_DIJTETELEK;OUT;PL/SQL TABLE;;;;;0;;BRUNO;DB_WEB_PORTAL;NEVSZAM_TAB_TYP
19734;35;DB_WEB;SENDPREOFFER_31101;1;1;;OUT;PL/SQL RECORD;;;;;0;;BRUNO;DB_WEB_PORTAL;NEVSZAM_REC_TYP
19734;35;DB_WEB;SENDPREOFFER_31101;2;1;NEV;OUT;VARCHAR2;;;CHAR_CS;VARCHAR2;80;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;2;2;ERTEK;OUT;NUMBER;12;2;;NUMBER;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;21;P_EVESDIJ;OUT;NUMBER;;;;NUMBER;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;22;P_HIBALISTA;OUT;PL/SQL TABLE;;;;;0;;BRUNO;DB_WEB_ELEKTR;HIBA_TAB_TYP
19734;35;DB_WEB;SENDPREOFFER_31101;1;1;;OUT;PL/SQL RECORD;;;;;0;;BRUNO;DB_WEB_ELEKTR;HIBA_REC_TYP
19734;35;DB_WEB;SENDPREOFFER_31101;2;1;HIBASZAM;OUT;NUMBER;9;;;NUMBER;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;2;2;SZOVEG;OUT;VARCHAR2;;;CHAR_CS;VARCHAR2;1000;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;23;P_HIBA_KOD;OUT;BINARY_INTEGER;;;;PLS_INTEGER;0;;;;
19734;35;DB_WEB;SENDPREOFFER_31101;0;24;P_HIBA_SZOV;OUT;VARCHAR2;;;CHAR_CS;VARCHAR2;;;;;
`
const awaited = `DECLARE
TYPE NUMBER_12__2_tab_typ IS TABLE OF NUMBER(12, 2) INDEX BY BINARY_INTEGER;
  TYPE VARCHAR2_80_tab_typ IS TABLE OF VARCHAR2(80) INDEX BY BINARY_INTEGER;
  TYPE NUMBER_9_tab_typ IS TABLE OF NUMBER(9) INDEX BY BINARY_INTEGER;
  TYPE VARCHAR2_1000_tab_typ IS TABLE OF VARCHAR2(1000) INDEX BY BINARY_INTEGER;
  v001 BRUNO.DB_WEB_ELEKTR.KOTVENY_REC_TYP;
    p021# DB_WEB_PORTAL.NEVSZAM_TAB_TYP;
  p021#_idx PLS_INTEGER := NULL;
  p021#nev VARCHAR2_80_tab_typ;
  p021#ertek NUMBER_12__2_tab_typ;
    p026# DB_WEB_ELEKTR.HIBA_TAB_TYP;
  p026#_idx PLS_INTEGER := NULL;
  p026#hibaszam NUMBER_9_tab_typ;
  p026#szoveg VARCHAR2_1000_tab_typ;
BEGIN

  v001.dijkod := :p002#dijkod;
  v001.dijfizmod := :p002#dijfizmod;
  v001.dijfizgyak := :p002#dijfizgyak;
  v001.szerkot := :p002#szerkot;
  v001.szerlejar := :p002#szerlejar;
  v001.kockezd := :p002#kockezd;
  v001.btkezd := :p002#btkezd;
  v001.halaszt_kockezd := :p002#halaszt_kockezd;
  v001.halaszt_dijfiz := :p002#halaszt_dijfiz;
  v001.szamlaszam := :p002#szamlaszam;
  v001.szamla_limit := :p002#szamla_limit;
  v001.evfordulo := :p002#evfordulo;
  v001.evfordulo_tipus := :p002#evfordulo_tipus;
  v001.e_komm_email := :p002#e_komm_email;
  v001.dijbekerot_ker := :p002#dijbekerot_ker;
  v001.ajanlati_evesdij := :p002#ajanlati_evesdij;

  p021#.DELETE;
  p021#nev.DELETE; p021#ertek.DELETE;

  p026#.DELETE;
  p026#szoveg.DELETE; p026#hibaszam.DELETE;

  DB_web.sendpreoffer_31101(p_sessionid=>:p_sessionid,
               p_lang=>:p_lang,
               p_vonalkod=>:p_vonalkod,
               p_kotveny=>v001,
               p_kedvezmenyek=>:p_kedvezmenyek,
               p_dump_args#=>:p_dump_args#,
               p_szerz_azon=>:p_szerz_azon,
               p_ajanlat_url=>:p_ajanlat_url,
               p_szamolt_dijtetelek=>p021#,
               p_evesdij=>:p_evesdij,
               p_hibalista=>p026#,
               p_hiba_kod=>:p_hiba_kod,
               p_hiba_szov=>:p_hiba_szov);

  :p002#dijkod := v001.dijkod;
  :p002#dijfizmod := v001.dijfizmod;
  :p002#dijfizgyak := v001.dijfizgyak;
  :p002#szerkot := v001.szerkot;
  :p002#szerlejar := v001.szerlejar;
  :p002#kockezd := v001.kockezd;
  :p002#btkezd := v001.btkezd;
  :p002#halaszt_kockezd := v001.halaszt_kockezd;
  :p002#halaszt_dijfiz := v001.halaszt_dijfiz;
  :p002#szamlaszam := v001.szamlaszam;
  :p002#szamla_limit := v001.szamla_limit;
  :p002#evfordulo := v001.evfordulo;
  :p002#evfordulo_tipus := v001.evfordulo_tipus;
  :p002#e_komm_email := v001.e_komm_email;
  :p002#dijbekerot_ker := v001.dijbekerot_ker;
  :p002#ajanlati_evesdij := v001.ajanlati_evesdij;

  p021#ertek.DELETE;
  p021#nev.DELETE;
  p021#_idx := p021#.FIRST;
  WHILE p021#_idx IS NOT NULL LOOP
    p021#nev(p021#_idx) := p021#(p021#_idx).nev;
    p021#ertek(p021#_idx) := p021#(p021#_idx).ertek;
    p021#_idx := p021#.NEXT(p021#_idx);
  END LOOP;
  :p021#nev := p021#nev;
  :p021#ertek := p021#ertek;

  p026#szoveg.DELETE;
  p026#hibaszam.DELETE;
  p026#_idx := p026#.FIRST;
  WHILE p026#_idx IS NOT NULL LOOP
    p026#hibaszam(p026#_idx) := p026#(p026#_idx).hibaszam;
    p026#szoveg(p026#_idx) := p026#(p026#_idx).szoveg;
    p026#_idx := p026#.NEXT(p026#_idx);
  END LOOP;
  :p026#hibaszam := p026#hibaszam;
  :p026#szoveg := p026#szoveg;

END;
`

func TestOne(t *testing.T) {
	functions, err := ParseCsv(strings.NewReader(csvSource))
	if err != nil {
		t.Logf("error parsing csv: %v", err)
		t.FailNow()
	}
	plsBlock, _ := functions[0].PlsqlBlock()
	if plsBlock != awaited {
		t.Logf("plsql block diff: " + diff.Diff(plsBlock, awaited))
		//t.Logf("---------------------------------")
		//t.Logf(plsBlock)
		//t.FailNow()
	}
}
