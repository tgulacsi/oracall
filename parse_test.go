/*
Copyright 2017 Tamás Gulácsi

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
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kylelemons/godebug/diff"
)

func TestParseDocs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	type testCase struct {
		Source string
		Want   map[string]string
	}

	for tcName, tc := range map[string]testCase{
		"nil": testCase{
			Source: "",
			Want:   nil,
		},
		"dbx": testCase{
			Source: `CREATE OR REPLACE PACKAGE DB_web_dbx IS

  /*
  login
    bejelentkezés

  INPUT:
    - p_login_nev - VARCHAR2 - bejelentkezési név
    - p_jelszo - VARCHAR2 - bejelentkezési jelszó

  OUTPUT:
    - p_sessionid - VARCHAR2 - belépési azonosító
    - p_torzsszam - VARCHAR2(10) - dolgozó törzsszáma
    - p_hiba_kod - PLS_INTEGER - hiba kódja
    - p_hiba_szov - VARCHAR2 - hiba szöveges leírása
  */
  PROCEDURE login(p_login_nev IN VARCHAR2, p_jelszo IN VARCHAR2,
                  p_sessionid OUT VARCHAR2, p_torzsszam OUT VARCHAR2,
                  p_hiba_kod OUT PLS_INTEGER, p_hiba_szov OUT VARCHAR2);

  /*
  logout
    kijelentkezés

  Input:
    - p_sessionid - VARCHAR2 - belépés azonosító
  */
  PROCEDURE logout(p_sessionid IN VARCHAR2);

  TYPE kozter_rec_typ IS RECORD (helynev VARCHAR2(25),
                                 ktid NUMBER(6),
                                 irszam VARCHAR2(5),
                                 utcanev VARCHAR2(25),
                                 uttipus VARCHAR2(20));
  TYPE kozter_tab_typ IS TABLE OF kozter_rec_typ INDEX BY BINARY_INTEGER;

  /*
  irszam2kozterulet
    Irányítószámra visszaadjuk a helységnevet, irányítószámot, utcanevet, úttipust és ktid-t (egyedi)

  Input:
    - p_sessionid - VARCHAR2 - belépés azonosító
    - p_irszam - VARCHAR2 - 4 jegyű irányítószám

  Output:
    - p_kozterulet
      --helynev - VARCHAR2(25) - helység neve
      --ktid - NUMBER(6) - közterület egyedi azonosítója
      --irszam - VARCHAR2(5) - 5 jegyű irányítószám
      --utcanev - VARCHAR2(25) - út/utca/... neve
      --uttipus - VARCHAR2(20) - út/utca/tér/...
    - p_hiba_kod - PLS_INTEGER - hiba kódja
    - p_hiba_szov - VARCHAR2 - hiba szöveges leírása
  */

  PROCEDURE irszam2kozterulet(p_sessionid IN VARCHAR2,
                              p_irszam IN VARCHAR2,
                              p_kozterulet OUT kozter_tab_typ,
                              p_hiba_kod OUT PLS_INTEGER,
                              p_hiba_szov OUT VARCHAR2);

  TYPE hitelintezet_rec_typ IS RECORD(ugyazon NUMBER(9),
                                      ugyfelnev VARCHAR2(40),
                                      irszam VARCHAR2(5),
                                      helynev VARCHAR2(40),
                                      utcanev VARCHAR2(25),
                                      uttipus VARCHAR2(20),
                                      hazszam VARCHAR2(15));
  TYPE hitelintezet_tab_typ IS TABLE OF hitelintezet_rec_typ INDEX BY BINARY_INTEGER;

  /*
  hitelintezetek
    Az eljárás visszaadja az általunk ismert hitelintézeteket

  INPUT:
    - p_sessionid - belépési azonosító

  OUTPUT:
    - p_hitelintezetek
      -- ugyazon - NUMBER(9) - Egyedi azonosító
      -- ugyfelnev - VARCHAR2(40) - név
      -- irszam - VARCHAR2(5) - 5 jegyű irányítószám
      -- helynev - VARCHAR2(40) - helységnév
      -- utcanev - VARCHAR2(25) - utca neve
      -- uttipus - VARCHAR2(20) - út tipusa
      -- hazszam - VARCHAR2(15) - házszám
    - p_hiba_kod - NUMBER - hiba kód
    - p_hiba_szov VARCHAR2 - hiba szöveg
  */

  PROCEDURE hitelintezetek(p_sessionid IN VARCHAR2,
                           p_hitelintezetek OUT hitelintezet_tab_typ,
                           p_hiba_kod OUT PLS_INTEGER,
                           p_hiba_szov OUT VARCHAR2);

  /*
  zaradekok
    Az eljárás visszaadja a záradékokat egy-egy azonosítóval

  INPUT:
    - p_sessionid - belépési azonosító

  OUTPUT:
    - p_zaradekok
      -- azon - NUMBER(3) - Egyedi azonosító
      -- szoveg - VARCHAR2(2000) - szöveg, ahol ha van "$" jel, annak a helyére írható szabad szöveg is
    - p_hiba_kod - NUMBER - hiba kód
    - p_hiba_szov VARCHAR2 - hiba szöveg
  */

  PROCEDURE zaradekok(p_sessionid IN VARCHAR2,
                      p_zaradekok OUT DB_web_elektr.zaradek_tab_typ,
                      p_hiba_kod OUT PLS_INTEGER,
                      p_hiba_szov OUT VARCHAR2);

  TYPE nyilatkozat_rec_typ IS RECORD (alairas CHAR(1),
                                      ugyfeltajekoztato CHAR(1),
                                      adatkezeles CHAR(1),
                                      ekomm CHAR(1),
                                      adatcsere CHAR(1),
                                      bizt_kozv_adat CHAR(1),
                                      biztosito_adat CHAR(1),
                                      elevules CHAR(1));

  /*
  sendPreOffer_21113
    Lakossági CASCO ajánlat feltöltése AKR-be

  INPUT:
    - p_sessionid - Session ID
    - p_vegleges - Végleges? [I/N]
    - p_elso_csekk_atadva - első csekk átadásra kerül? I/N
    - p_szerzo - Szerző/fenntartó törzsszáma
    - p_modkod - módozat kódja (21111 a default, de lehet pl. 21113 is)
    - p_vonalkod - Az ajánlat vonalkódja
    - p_kotveny - Általános kötvényadatok
    - p_fe_ajanlat - frontend ajánlat specifikus adatok (felv: ld. fe_tipus, forras: ld. fe_tipus)
    - p_szerzodo - Szerződő adatai
    - p_szerzodo_cim - Szerződő címe
    - p_szerzodo_levelcim - Szerződő levelezési címe
    - p_biztositott - Biztosított adatai
    - p_biztositott_cim - Biztosított címe
    - p_biztositott_levelcim - Biztosított levelezési címe
    - p_uzembentarto - Üzembentartó adatai
    - p_uzembentarto_cim - Üzembentartó címe
    - p_uzembentarto_levelcim - Üzembentartó levelezési címe
    - p_tulajdonos - Tulajdonos adatai
    - p_tulajdonos_cim - Tulajdonos címe
    - p_tulajdonos_levelcim - Tulajdonos levelezési címe
    - p_engedmenyezett - Engedményezett adatai
    - p_engedmenyezettek_cim - Engedményezett címe
    - p_engedmeny - Engedmény adatai
    - p_gepjarmu - Gépjárműadatok
    - p_gepjarmu_laca - Egyéb (módozatspecifikus) gépjárműadatok
    - p_casco_allapotlap - Állapotlap adatok
    - p_casco_tartozekok - Tartozékok tömbje
    - p_bonusz_elozmeny - előzmény adatok
    - p_kedvezmenyek - Kedvezmények tömbje
    - p_gfb_szerz_azon - partner szerződés azonosítója
    - p_laca_spec - Egyéb módozatspecifikus adatok

  OUTPUT:
    - p_szerz_azon - Szerződésszám (csak akkor nem üres, ha sikeres és végleges az ajánlat feltöltés)
    - p_ajanlat_url - BRUNO által nyomtatott ajánlat elérési útja (egyelőre üres)
    - p_evesdij - A szerződés éves díja
    - p_hibalista - Hibalista
    - p_hiba_kod - Hibakód (0 ha nincs hiba)
    - p_hiba_szov - Hibaszöveg
  */
  PROCEDURE sendPreOffer_21113(p_sessionid IN VARCHAR2,
                               p_messagenumber IN VARCHAR2,
                               p_vegleges IN VARCHAR2 DEFAULT 'N',
                               p_szerzo IN VARCHAR2,
                               p_modkod IN VARCHAR2,
                               p_ajanlatszam IN VARCHAR2,
                               p_vonalkod IN OUT PLS_INTEGER,
                               p_kotveny IN OUT DB_web_elektr.kotveny_rec_typ,-- OUT is ????
                               p_szerzodo IN DB_web_portal.mod_ugyfel_rec_typ,
                               p_szerzodo_cim IN DB_web_elektr.mod_cim_rec_typ,
                               p_szerzodo_levelcim IN DB_web_elektr.mod_cim_rec_typ,
                               p_biztositott IN DB_web_portal.mod_ugyfel_rec_typ,
                               p_biztositott_cim IN DB_web_elektr.mod_cim_rec_typ,
                               p_biztositott_levelcim IN DB_web_elektr.mod_cim_rec_typ,
                               p_uzembentarto IN DB_web_portal.mod_ugyfel_rec_typ,
                               p_uzembentarto_cim IN DB_web_elektr.mod_cim_rec_typ,
                               p_uzembentarto_levelcim IN DB_web_elektr.mod_cim_rec_typ,
                               p_tulajdonos IN DB_web_portal.mod_ugyfel_rec_typ,
                               p_tulajdonos_cim IN DB_web_elektr.mod_cim_rec_typ,
                               p_tulajdonos_levelcim IN DB_web_elektr.mod_cim_rec_typ,
                               p_engedmenyezett IN PLS_INTEGER,
                               p_engedmeny IN DB_web_elektr.engedmeny_rec_typ,
                               p_gepjarmu IN OUT DB_gfb_portal.gepjarmu_rec_typ,
                               p_gepjarmu_21113 IN DB_21113_portal.gepjarmu_21113_rec_typ,
                               p_casco_allapotlap IN DB_21113_portal.casco_allapotlap_rec_typ,
                               p_casco_tartozekok IN DB_21113_portal.casco_tartozekok_tab_typ,
                               p_bonusz_elozmeny IN DB_gfb_portal.bonusz_elozmeny_rec_typ,
                               p_kedvezmenyek IN DB_21113_portal.kedvezmeny_tab_typ,
                               p_gfb_szerz_azon IN PLS_INTEGER,
                               p_21113_spec IN DB_21113_portal.casco_21113_spec_rec_typ,
                               p_nyilatkozat IN nyilatkozat_rec_typ,
                               p_zaradek IN DB_web_elektr.zaradek_tab_typ,
                               p_args# IN VARCHAR2,
                               p_szerz_azon OUT PLS_INTEGER,
                               p_ajanlat_url OUT VARCHAR2,
                               p_evesdij OUT NUMBER,
                               p_hibalista OUT DB_web_elektr.hiba_tab_typ,
                               p_hiba_kod OUT PLS_INTEGER,
                               p_hiba_szov OUT VARCHAR2);
END DB_web_dbx;`,
			Want: map[string]string{
				"login": `
  login
    bejelentkezés

  INPUT:
    - p_login_nev - VARCHAR2 - bejelentkezési név
    - p_jelszo - VARCHAR2 - bejelentkezési jelszó

  OUTPUT:
    - p_sessionid - VARCHAR2 - belépési azonosító
    - p_torzsszam - VARCHAR2(10) - dolgozó törzsszáma
    - p_hiba_kod - PLS_INTEGER - hiba kódja
    - p_hiba_szov - VARCHAR2 - hiba szöveges leírása`,

				"logout": `
  logout
    kijelentkezés

  Input:
    - p_sessionid - VARCHAR2 - belépés azonosító`,

				"irszam2kozterulet": `
  irszam2kozterulet
    Irányítószámra visszaadjuk a helységnevet, irányítószámot, utcanevet, úttipust és ktid-t (egyedi)

  Input:
    - p_sessionid - VARCHAR2 - belépés azonosító
    - p_irszam - VARCHAR2 - 4 jegyű irányítószám

  Output:
    - p_kozterulet
      --helynev - VARCHAR2(25) - helység neve
      --ktid - NUMBER(6) - közterület egyedi azonosítója
      --irszam - VARCHAR2(5) - 5 jegyű irányítószám
      --utcanev - VARCHAR2(25) - út/utca/... neve
      --uttipus - VARCHAR2(20) - út/utca/tér/...
    - p_hiba_kod - PLS_INTEGER - hiba kódja
    - p_hiba_szov - VARCHAR2 - hiba szöveges leírása`,

				"hitelintezetek": `hitelintezetek
    Az eljárás visszaadja az általunk ismert hitelintézeteket

  INPUT:
    - p_sessionid - belépési azonosító

  OUTPUT:
    - p_hitelintezetek
      -- ugyazon - NUMBER(9) - Egyedi azonosító
      -- ugyfelnev - VARCHAR2(40) - név
      -- irszam - VARCHAR2(5) - 5 jegyű irányítószám
      -- helynev - VARCHAR2(40) - helységnév
      -- utcanev - VARCHAR2(25) - utca neve
      -- uttipus - VARCHAR2(20) - út tipusa
      -- hazszam - VARCHAR2(15) - házszám
    - p_hiba_kod - NUMBER - hiba kód
    - p_hiba_szov VARCHAR2 - hiba szöveg`,

				"zaradekok": `
  zaradekok
    Az eljárás visszaadja a záradékokat egy-egy azonosítóval

  INPUT:
    - p_sessionid - belépési azonosító

  OUTPUT:
    - p_zaradekok
      -- azon - NUMBER(3) - Egyedi azonosító
      -- szoveg - VARCHAR2(2000) - szöveg, ahol ha van "$" jel, annak a helyére írható szabad szöveg is
    - p_hiba_kod - NUMBER - hiba kód
    - p_hiba_szov VARCHAR2 - hiba szöveg`,

				"sendPreOffer_21113": `sendPreOffer_21113
    Lakossági CASCO ajánlat feltöltése AKR-be

  INPUT:
    - p_sessionid - Session ID
    - p_vegleges - Végleges? [I/N]
    - p_elso_csekk_atadva - első csekk átadásra kerül? I/N
    - p_szerzo - Szerző/fenntartó törzsszáma
    - p_modkod - módozat kódja (21111 a default, de lehet pl. 21113 is)
    - p_vonalkod - Az ajánlat vonalkódja
    - p_kotveny - Általános kötvényadatok
    - p_fe_ajanlat - frontend ajánlat specifikus adatok (felv: ld. fe_tipus, forras: ld. fe_tipus)
    - p_szerzodo - Szerződő adatai
    - p_szerzodo_cim - Szerződő címe
    - p_szerzodo_levelcim - Szerződő levelezési címe
    - p_biztositott - Biztosított adatai
    - p_biztositott_cim - Biztosított címe
    - p_biztositott_levelcim - Biztosított levelezési címe
    - p_uzembentarto - Üzembentartó adatai
    - p_uzembentarto_cim - Üzembentartó címe
    - p_uzembentarto_levelcim - Üzembentartó levelezési címe
    - p_tulajdonos - Tulajdonos adatai
    - p_tulajdonos_cim - Tulajdonos címe
    - p_tulajdonos_levelcim - Tulajdonos levelezési címe
    - p_engedmenyezett - Engedményezett adatai
    - p_engedmenyezettek_cim - Engedményezett címe
    - p_engedmeny - Engedmény adatai
    - p_gepjarmu - Gépjárműadatok
    - p_gepjarmu_laca - Egyéb (módozatspecifikus) gépjárműadatok
    - p_casco_allapotlap - Állapotlap adatok
    - p_casco_tartozekok - Tartozékok tömbje
    - p_bonusz_elozmeny - előzmény adatok
    - p_kedvezmenyek - Kedvezmények tömbje
    - p_gfb_szerz_azon - partner szerződés azonosítója
    - p_laca_spec - Egyéb módozatspecifikus adatok

  OUTPUT:
    - p_szerz_azon - Szerződésszám (csak akkor nem üres, ha sikeres és végleges az ajánlat feltöltés)
    - p_ajanlat_url - BRUNO által nyomtatott ajánlat elérési útja (egyelőre üres)
    - p_evesdij - A szerződés éves díja
    - p_hibalista - Hibalista
    - p_hiba_kod - Hibakód (0 ha nincs hiba)
    - p_hiba_szov - Hibaszöveg`,
			},
		},
	} {

		docs, err := parseDocs(ctx, tc.Source)
		if err != nil {
			t.Errorf("%q. %v", tcName, err)
			continue
		}
		got := make([]string, 0, len(docs))
		for k := range docs {
			got = append(got, k)
		}
		sort.Strings(got)
		want := make([]string, 0, len(tc.Want))
		for k := range tc.Want {
			want = append(want, k)
		}
		sort.Strings(want)

		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q. got %q, wanted %q.", tcName, got, want)
			continue
		}
		for _, k := range got {
			g := strings.TrimSpace(docs[k])
			w := strings.TrimSpace(tc.Want[k])
			if d := diff.Diff(w, g); d != "" {
				t.Errorf("%q.\n%s", tcName, d)
			}
		}
	}
}
