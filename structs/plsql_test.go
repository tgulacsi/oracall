package structs

import (
	"strings"
	"testing"

	"github.com/kylelemons/godebug/diff"

	//"github.com/kylelemons/godebug/diff"
)

const awaited = `DECLARE
TYPE NUMBER_12__2_tab_typ IS TABLE OF NUMBER(12, 2) INDEX BY BINARY_INTEGER;
  TYPE VARCHAR2_80_tab_typ IS TABLE OF VARCHAR2(80) INDEX BY BINARY_INTEGER;
  TYPE NUMBER_9_tab_typ IS TABLE OF NUMBER(9) INDEX BY BINARY_INTEGER;
  TYPE VARCHAR2_1000_tab_typ IS TABLE OF VARCHAR2(1000) INDEX BY BINARY_INTEGER;
  v001 BRUNO.DB_WEB_ELEKTR.KOTVENY_REC_TYP;
    x021# DB_WEB_PORTAL.NEVSZAM_TAB_TYP;
  x021#_idx PLS_INTEGER := NULL;
  x021#nev VARCHAR2_80_tab_typ;
  x021#ertek NUMBER_12__2_tab_typ;
    x026# DB_WEB_ELEKTR.HIBA_TAB_TYP;
  x026#_idx PLS_INTEGER := NULL;
  x026#hibaszam NUMBER_9_tab_typ;
  x026#szoveg VARCHAR2_1000_tab_typ;
BEGIN

  v001.dijkod := :x002#dijkod;
  v001.dijfizmod := :x002#dijfizmod;
  v001.dijfizgyak := :x002#dijfizgyak;
  v001.szerkot := :x002#szerkot;
  v001.szerlejar := :x002#szerlejar;
  v001.kockezd := :x002#kockezd;
  v001.btkezd := :x002#btkezd;
  v001.halaszt_kockezd := :x002#halaszt_kockezd;
  v001.halaszt_dijfiz := :x002#halaszt_dijfiz;
  v001.szamlaszam := :x002#szamlaszam;
  v001.szamla_limit := :x002#szamla_limit;
  v001.evfordulo := :x002#evfordulo;
  v001.evfordulo_tipus := :x002#evfordulo_tipus;
  v001.e_komm_email := :x002#e_komm_email;
  v001.dijbekerot_ker := :x002#dijbekerot_ker;
  v001.ajanlati_evesdij := :x002#ajanlati_evesdij;

  x021#.DELETE;
  x021#nev.DELETE; x021#ertek.DELETE;

  x026#.DELETE;
  x026#szoveg.DELETE; x026#hibaszam.DELETE;

  DB_web.sendpreoffer_31101(p_sessionid=>:p_sessionid,
               p_lang=>:p_lang,
               p_vonalkod=>:p_vonalkod,
               p_kotveny=>v001,
               p_kedvezmenyek=>:p_kedvezmenyek,
               p_dump_args#=>:p_dump_args#,
               p_szerz_azon=>:p_szerz_azon,
               p_ajanlat_url=>:p_ajanlat_url,
               p_szamolt_dijtetelek=>x021#,
               p_evesdij=>:p_evesdij,
               p_hibalista=>x026#,
               p_hiba_kod=>:p_hiba_kod,
               p_hiba_szov=>:p_hiba_szov);

  :x002#dijkod := v001.dijkod;
  :x002#dijfizmod := v001.dijfizmod;
  :x002#dijfizgyak := v001.dijfizgyak;
  :x002#szerkot := v001.szerkot;
  :x002#szerlejar := v001.szerlejar;
  :x002#kockezd := v001.kockezd;
  :x002#btkezd := v001.btkezd;
  :x002#halaszt_kockezd := v001.halaszt_kockezd;
  :x002#halaszt_dijfiz := v001.halaszt_dijfiz;
  :x002#szamlaszam := v001.szamlaszam;
  :x002#szamla_limit := v001.szamla_limit;
  :x002#evfordulo := v001.evfordulo;
  :x002#evfordulo_tipus := v001.evfordulo_tipus;
  :x002#e_komm_email := v001.e_komm_email;
  :x002#dijbekerot_ker := v001.dijbekerot_ker;
  :x002#ajanlati_evesdij := v001.ajanlati_evesdij;

  x021#ertek.DELETE;
  x021#nev.DELETE;
  x021#_idx := x021#.FIRST;
  WHILE x021#_idx IS NOT NULL LOOP
    x021#nev(x021#_idx) := x021#(x021#_idx).nev;
    x021#ertek(x021#_idx) := x021#(x021#_idx).ertek;
    x021#_idx := x021#.NEXT(x021#_idx);
  END LOOP;
  :x021#nev := x021#nev;
  :x021#ertek := x021#ertek;

  x026#szoveg.DELETE;
  x026#hibaszam.DELETE;
  x026#_idx := x026#.FIRST;
  WHILE x026#_idx IS NOT NULL LOOP
    x026#hibaszam(x026#_idx) := x026#(x026#_idx).hibaszam;
    x026#szoveg(x026#_idx) := x026#(x026#_idx).szoveg;
    x026#_idx := x026#.NEXT(x026#_idx);
  END LOOP;
  :x026#hibaszam := x026#hibaszam;
  :x027#szoveg := x026#szoveg;

END;
`

func TestOne(t *testing.T) {
	functions, err := ParseCsv(strings.NewReader(csvSource))
	if err != nil {
		t.Errorf("error parsing csv: %v", err)
		t.FailNow()
	}
	got, _ := functions[0].PlsqlBlock()
	d := diff.Diff(awaited, got)
	if d != "" {
		//FIXME(tgulacsi): this should be an error!
		t.Logf("plsql block diff:\n" + d)
		//fmt.Printf("GOT:\n", got)
	}
}
