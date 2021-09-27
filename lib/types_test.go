// Copyright 2017, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import (
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestParseDigits(t *testing.T) {
	for tN, tC := range []struct {
		In          string
		Prec, Scale int
		WantErr     bool
	}{
		{In: ""},
		{In: "0", Prec: 32, Scale: 4},
		{In: "-12.3", Prec: 3, Scale: 1},
		{In: "12.34", Prec: 3, Scale: 1, WantErr: true},
	} {
		if err := ParseDigits(tC.In, tC.Prec, tC.Scale); err == nil && tC.WantErr {
			t.Errorf("%d. wanted error for %q", tN, tC.In)
		} else if err != nil && !tC.WantErr {
			t.Errorf("%d. got error %+v for %q", tN, err, tC.In)
		}
	}
}

func TestQueryError(t *testing.T) {
	const qry6502 = "DECLARE\n i1 PLS_INTEGER;\n i2 PLS_INTEGER;\n v001 BRUNO.DB_KAR_DEALER.GJMU_REC_TYPE; --E=p_okozo_gjmu\n v025 BRUNO.DB_KAR_DEALER.CIM_REC_TYPE; --E=p_karhely\n v039 BRUNO.DB_KAR_DEALER.UGYFEL_REC_TYPE; --E=p_bejelento\n v070 BRUNO.DB_KAR_DEALER.GJMU_REC_TYPE; --E=p_kar_gjmu\n v094 BRUNO.DB_KAR_DEALER.UGYFEL_REC_TYPE; --E=p_kar_szemely\n v125 BRUNO.DB_KAR_DEALER.CIM_REC_TYPE; --E=p_szemlehely\n v139 BRUNO.DB_KAR_DEALER.UGYFEL_TAB_TYPE; --C=pt_ugyfelek\n TYPE VARCHAR2_75_tab_typ IS TABLE OF VARCHAR2(75) INDEX BY BINARY_INTEGER;\n p140#ugyfelnev VARCHAR2_75_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_5_tab_typ IS TABLE OF VARCHAR2(5) INDEX BY BINARY_INTEGER;\n p140#irszam VARCHAR2_5_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_25_tab_typ IS TABLE OF VARCHAR2(25) INDEX BY BINARY_INTEGER;\n p140#utcanev VARCHAR2_25_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_20_tab_typ IS TABLE OF VARCHAR2(20) INDEX BY BINARY_INTEGER;\n p140#uttipus VARCHAR2_20_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_11_tab_typ IS TABLE OF VARCHAR2(11) INDEX BY BINARY_INTEGER;\n p140#hazszam VARCHAR2_11_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_10_tab_typ IS TABLE OF VARCHAR2(10) INDEX BY BINARY_INTEGER;\n p140#emelet VARCHAR2_10_tab_typ; --D=pt_ugyfelek\n p140#ajto VARCHAR2_10_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_27_tab_typ IS TABLE OF VARCHAR2(27) INDEX BY BINARY_INTEGER;\n p140#szulhely VARCHAR2_27_tab_typ; --D=pt_ugyfelek\n TYPE DATE_tab_typ IS TABLE OF DATE INDEX BY BINARY_INTEGER;\n p140#szuldat DATE_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_9_tab_typ IS TABLE OF VARCHAR2(9) INDEX BY BINARY_INTEGER;\n p140#tbazon VARCHAR2_9_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_15_tab_typ IS TABLE OF VARCHAR2(15) INDEX BY BINARY_INTEGER;\n p140#tel_otthon VARCHAR2_15_tab_typ; --D=pt_ugyfelek\n p140#tel_mhely VARCHAR2_15_tab_typ; --D=pt_ugyfelek\n p140#tel_mobil VARCHAR2_15_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_255_tab_typ IS TABLE OF VARCHAR2(255) INDEX BY BINARY_INTEGER;\n p140#email VARCHAR2_255_tab_typ; --D=pt_ugyfelek\n p140#fax VARCHAR2_15_tab_typ; --D=pt_ugyfelek\n p140#adoszam VARCHAR2_11_tab_typ; --D=pt_ugyfelek\n p140#adoaz VARCHAR2_10_tab_typ; --D=pt_ugyfelek\n p140#taj VARCHAR2_20_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_24_tab_typ IS TABLE OF VARCHAR2(24) INDEX BY BINARY_INTEGER;\n p140#szamlaszam VARCHAR2_24_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_1_tab_typ IS TABLE OF VARCHAR2(1) INDEX BY BINARY_INTEGER;\n p140#tipus VARCHAR2_1_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_100_tab_typ IS TABLE OF VARCHAR2(100) INDEX BY BINARY_INTEGER;\n p140#kapcsolat VARCHAR2_100_tab_typ; --D=pt_ugyfelek\n p140#ugyfeltip VARCHAR2_1_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_3_tab_typ IS TABLE OF VARCHAR2(3) INDEX BY BINARY_INTEGER;\n p140#orsz_kod VARCHAR2_3_tab_typ; --D=pt_ugyfelek\n p140#kulf_irszam VARCHAR2_10_tab_typ; --D=pt_ugyfelek\n p140#kulf_helynev VARCHAR2_27_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_38_tab_typ IS TABLE OF VARCHAR2(38) INDEX BY BINARY_INTEGER;\n p140#kulf_utca VARCHAR2_38_tab_typ; --D=pt_ugyfelek\n p140#kulf_hszajto VARCHAR2_38_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_1000_tab_typ IS TABLE OF VARCHAR2(1000) INDEX BY BINARY_INTEGER;\n p140#kulf_egyeb VARCHAR2_1000_tab_typ; --D=pt_ugyfelek\n TYPE VARCHAR2_40_tab_typ IS TABLE OF VARCHAR2(40) INDEX BY BINARY_INTEGER;\n p140#anyanev VARCHAR2_40_tab_typ; --D=pt_ugyfelek\n v170 BRUNO.DB_KAR_DEALER.HIBA_TAB_TYPE; --C=pt_hiba\n p171#hiba_szov VARCHAR2_1000_tab_typ; --D=pt_hiba\n TYPE INTEGER_10_tab_typ IS TABLE OF INTEGER(10) INDEX BY BINARY_INTEGER;\n p171#hiba_szint INTEGER_10_tab_typ; --D=pt_hiba\n\nBEGIN\n v001.jelleg := :1;\n v001.gyartmany := :2;\n v001.rendszam := :3;\n v001.alvazszam := :4;\n v001.motorszam := :5;\n v001.henger := :6;\n v001.forgbalep := :7;\n v001.tipus := :8;\n v001.utasszam := :9;\n v001.stomeg := :10;\n v001.mtomeg := :11;\n v001.gyartev := :12;\n v001.uzemmod := :13;\n v001.modell := :14;\n v001.uzjelleg := :15;\n v001.teljesitmeny := :16;\n v001.szin := :17;\n v001.torzskonyv := :18;\n v001.eurotax_kod := :19;\n v001.feng_szam := :20;\n v001.kv_osztaly := :21;\n v001.potkocsi := :22;\n v025.irszam := :23;\n v025.utcanev := :24;\n v025.uttipus := :25;\n v025.hazszam := :26;\n v025.emelet := :27;\n v025.ajto := :28;\n v025.orsz_kod := :29;\n v025.kulf_irszam := :30;\n v025.kulf_helynev := :31;\n v025.kulf_utca := :32;\n v025.kulf_hszajto := :33;\n v025.kulf_egyeb := :34;\n v039.ugyfelnev := :35;\n v039.irszam := :36;\n v039.utcanev := :37;\n v039.uttipus := :38;\n v039.hazszam := :39;\n v039.emelet := :40;\n v039.ajto := :41;\n v039.szulhely := :42;\n v039.szuldat := :43;\n v039.tbazon := :44;\n v039.tel_otthon := :45;\n v039.tel_mhely := :46;\n v039.tel_mobil := :47;\n v039.email := :48;\n v039.fax := :49;\n v039.adoszam := :50;\n v039.adoaz := :51;\n v039.taj := :52;\n v039.szamlaszam := :53;\n v039.tipus := :54;\n v039.kapcsolat := :55;\n v039.ugyfeltip := :56;\n v039.orsz_kod := :57;\n v039.kulf_irszam := :58;\n v039.kulf_helynev := :59;\n v039.kulf_utca := :60;\n v039.kulf_hszajto := :61;\n v039.kulf_egyeb := :62;\n v039.anyanev := :63;\n v070.jelleg := :64;\n v070.gyartmany := :65;\n v070.rendszam := :66;\n v070.alvazszam := :67;\n v070.motorszam := :68;\n v070.henger := :69;\n v070.forgbalep := :70;\n v070.tipus := :71;\n v070.utasszam := :72;\n v070.stomeg := :73;\n v070.mtomeg := :74;\n v070.gyartev := :75;\n v070.uzemmod := :76;\n v070.modell := :77;\n v070.uzjelleg := :78;\n v070.teljesitmeny := :79;\n v070.szin := :80;\n v070.torzskonyv := :81;\n v070.eurotax_kod := :82;\n v070.feng_szam := :83;\n v070.kv_osztaly := :84;\n v070.potkocsi := :85;\n v094.ugyfelnev := :86;\n v094.irszam := :87;\n v094.utcanev := :88;\n v094.uttipus := :89;\n v094.hazszam := :90;\n v094.emelet := :91;\n v094.ajto := :92;\n v094.szulhely := :93;\n v094.szuldat := :94;\n v094.tbazon := :95;\n v094.tel_otthon := :96;\n v094.tel_mhely := :97;\n v094.tel_mobil := :98;\n v094.email := :99;\n v094.fax := :100;\n v094.adoszam := :101;\n v094.adoaz := :102;\n v094.taj := :103;\n v094.szamlaszam := :104;\n v094.tipus := :105;\n v094.kapcsolat := :106;\n v094.ugyfeltip := :107;\n v094.orsz_kod := :108;\n v094.kulf_irszam := :109;\n v094.kulf_helynev := :110;\n v094.kulf_utca := :111;\n v094.kulf_hszajto := :112;\n v094.kulf_egyeb := :113;\n v094.anyanev := :114;\n v125.irszam := :115;\n v125.utcanev := :116;\n v125.uttipus := :117;\n v125.hazszam := :118;\n v125.emelet := :119;\n v125.ajto := :120;\n v125.orsz_kod := :121;\n v125.kulf_irszam := :122;\n v125.kulf_helynev := :123;\n v125.kulf_utca := :124;\n v125.kulf_hszajto := :125;\n v125.kulf_egyeb := :126;\n p140#ugyfelnev := :127;\n p140#irszam := :128;\n p140#utcanev := :129;\n p140#uttipus := :130;\n p140#hazszam := :131;\n p140#emelet := :132;\n p140#ajto := :133;\n p140#szulhely := :134;\n p140#szuldat := :135;\n p140#tbazon := :136;\n p140#tel_otthon := :137;\n p140#tel_mhely := :138;\n p140#tel_mobil := :139;\n p140#email := :140;\n p140#fax := :141;\n p140#adoszam := :142;\n p140#adoaz := :143;\n p140#taj := :144;\n p140#szamlaszam := :145;\n p140#tipus := :146;\n p140#kapcsolat := :147;\n p140#ugyfeltip := :148;\n p140#orsz_kod := :149;\n p140#kulf_irszam := :150;\n p140#kulf_helynev := :151;\n p140#kulf_utca := :152;\n p140#kulf_hszajto := :153;\n p140#kulf_egyeb := :154;\n p140#anyanev := :155;\n \n i1 := p140#ugyfelnev.FIRST;\n WHILE i1 IS NOT NULL LOOP\n v139(i1).ugyfelnev := p140#ugyfelnev(i1);\n v139(i1).irszam := p140#irszam(i1);\n v139(i1).utcanev := p140#utcanev(i1);\n v139(i1).uttipus := p140#uttipus(i1);\n v139(i1).hazszam := p140#hazszam(i1);\n v139(i1).emelet := p140#emelet(i1);\n v139(i1).ajto := p140#ajto(i1);\n v139(i1).szulhely := p140#szulhely(i1);\n v139(i1).szuldat := p140#szuldat(i1);\n v139(i1).tbazon := p140#tbazon(i1);\n v139(i1).tel_otthon := p140#tel_otthon(i1);\n v139(i1).tel_mhely := p140#tel_mhely(i1);\n v139(i1).tel_mobil := p140#tel_mobil(i1);\n v139(i1).email := p140#email(i1);\n v139(i1).fax := p140#fax(i1);\n v139(i1).adoszam := p140#adoszam(i1);\n v139(i1).adoaz := p140#adoaz(i1);\n v139(i1).taj := p140#taj(i1);\n v139(i1).szamlaszam := p140#szamlaszam(i1);\n v139(i1).tipus := p140#tipus(i1);\n v139(i1).kapcsolat := p140#kapcsolat(i1);\n v139(i1).ugyfeltip := p140#ugyfeltip(i1);\n v139(i1).orsz_kod := p140#orsz_kod(i1);\n v139(i1).kulf_irszam := p140#kulf_irszam(i1);\n v139(i1).kulf_helynev := p140#kulf_helynev(i1);\n v139(i1).kulf_utca := p140#kulf_utca(i1);\n v139(i1).kulf_hszajto := p140#kulf_hszajto(i1);\n v139(i1).kulf_egyeb := p140#kulf_egyeb(i1);\n v139(i1).anyanev := p140#anyanev(i1);\n i1 := p140#ugyfelnev.NEXT(i1);\n END LOOP;\n v170.DELETE;\n p171#hiba_szov.DELETE;\n p171#hiba_szint.DELETE;\n\n :156 := DB_kar_dealer.gfb_karbejelentes(p_szerz_azon=>:157,\n\t\tp_vonalkod=>:158,\n\t\tp_okozo_gjmu=>v001,\n\t\tp_karido=>:159,\n\t\tp_megjegyzes=>:160,\n\t\tp_leiras=>:161,\n\t\tp_karhely_kulso=>:162,\n\t\tp_karhely=>v025,\n\t\tp_bejelento=>v039,\n\t\tp_szemelyserules=>:163,\n\t\tp_kar_gjmu=>v070,\n\t\tp_kar_szemely=>v094,\n\t\tp_kar_vagyontargy=>:164,\n\t\tp_szemlehely_kulso=>:165,\n\t\tp_szemlehely=>v125,\n\t\tpt_ugyfelek=>v139,\n\t\tp_hatosagi_int=>:166,\n\t\tp_hatosagi_int_dat=>:167,\n\t\tp_feljelentes=>:168,\n\t\tp_feljelentes_dat=>:169,\n\t\tp_becsult_kosszeg=>:170,\n\t\tp_mehet=>:171,\n\t\tp_kar_vonalkod=>:172,\n\t\tpt_hiba=>v170,\n\t\tp_commit=>:173);\n\n \n i1 := v170.FIRST; i2 := 1;\n WHILE i1 IS NOT NULL LOOP\n p171#hiba_szov(i2) := v170(i1).hiba_szov;\n p171#hiba_szint(i2) := v170(i1).hiba_szint;\n i1 := v170.NEXT(i1); i2 := i2 + 1;\n END LOOP;\n :174 := p171#hiba_szov;\n :175 := p171#hiba_szint;\n\nEND;\n"
	err6502 := &fakeErr{query: qry6502, params: "[01 HONDA FAG486 0 0001-01-01 00:00:00 +0000 UTC CIVIC 0 0 0 0 0 11140 Bocskai út 22 Kárpáti Márton 11480 Kalapács utca 16 fszt 2 0001-01-01 00:00:00 +0000 UTC karpati.marton6@gmail.com E 01 PEUGEOT IFA387 0 0001-01-01 00:00:00 +0000 UTC 307 0 0 0 0 0 <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> <nil> 11030 Kőér utca 3 Argo abroncs gumiszerviz [László Marianna Kárpáti Márton dr Bodnár Bence dr Hitesy Edina László Péter] [8451A 11480 10910 10920 11430] [Vörösteleki Kalapács Üllői Hőgyes Endre Stefánia] [utca utca út utca út] [29 16 11 15 35] [ fszt ] [ 2 ] [ ] [0001-01-01 00:00:00 +0000 UTC 0001-01-01 00:00:00 +0000 UTC 0001-01-01 00:00:00 +0000 UTC 0001-01-01 00:00:00 +0000 UTC 0001-01-01 00:00:00 +0000 UTC] [ ] [ ] [ ] [ ] [vallalkozasmarketing@gmail.com karpati.marton6@gmail.com ] [ ] [ ] [ ] [ ] [ ] [ ] [ ] [Z V z v N] [ ] [ ] [ ] [ ] [ ] [ ] [ ] {_Named_Fields_Required:{} Dest:0xc001065ee8 In:false} 0 0 2021-09-19 12:50:00 +0000 UTC Az IFA387 forgalmi rendszámú Peugewot 307 típusú auóval a lámpánál szabályosan állóra fékeztem az autót, majd a mögöttem lévő FAG486 forgalmi rendszámú Honda Civic (mivel nem vette észre, hogy megálltam, vagy későn vette észre), nekem jött hátulról. Személyi sérülés nem történt. 0 0001-01-01 00:00:00 +0000 UTC 0001-01-01 00:00:00 +0000 UTC 200000 {_Named_Fields_Required:{} Dest: In:false} {_Named_Fields_Required:{} Dest:0xc000ffd8c0 In:false} {_Named_Fields_Required:{} Dest:0xc000ffd8f0 In:false}]",
		code: 6502, errMsg: "PL/SQL: numerikus- vagy értékhiba (: a karakterlánc-puffer túl kicsi) ORA-06512: a(z) helyen a(z) 183. sornál"}

	for nm, elt := range map[string]struct {
		Query string
		Err   error
		Want  QueryError
	}{
		"EOF": {"", io.EOF, QueryError{err: io.EOF}},
		"6502": {
			Query: qry6502,
			Err:   err6502,
			Want:  QueryError{query: qry6502, err: err6502, code: 6502, lineNo: 183},
		},
	} {
		got := NewQueryError(elt.Query, elt.Err)
		if g, w := got.Code(), elt.Want.Code(); g != w {
			t.Errorf("%q. got code %d, wanted %d", nm, g, w)
		}
		if g, w := errors.Unwrap(got), elt.Err; g != w {
			t.Errorf("%q: got error %+v, wanted %+v", nm, g, w)
		}
		if g, w := got.lineNo, elt.Want.lineNo; g != w {
			t.Errorf("%q. got lineNo %d, wanted %d", nm, g, w)
		}
		t.Logf("%q. line=%q", nm, got.Line())
	}
}

type fakeErr struct {
	query, params, errMsg string
	code                  int
}

func (fe *fakeErr) Error() string {
	return fmt.Sprintf("%s %s ORA-%05d: %s", fe.query, fe.params, fe.code, fe.errMsg)
}
func (fe *fakeErr) Code() int { return fe.code }
