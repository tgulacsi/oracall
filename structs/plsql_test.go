package structs

const awaited = `DECLARE
TYPE NUMBER_12__2_tab_typ IS TABLE OF NUMBER(12, 2) INDEX BY BINARY_INTEGER;
  TYPE VARCHAR2_80_tab_typ IS TABLE OF VARCHAR2(80) INDEX BY BINARY_INTEGER;
  TYPE NUMBER_9_tab_typ IS TABLE OF NUMBER(9) INDEX BY BINARY_INTEGER;
  TYPE VARCHAR2_1000_tab_typ IS TABLE OF VARCHAR2(1000) INDEX BY BINARY_INTEGER;
  x5# DB_WEB_ELEKTR.KOTVENY_REC_TYP;
    x22# DB_WEB_PORTAL.MOD_UGYFEL_REC_TYP;
    x49# DB_WEB_ELEKTR.MOD_CIM_REC_TYP;
    x65# DB_WEB_ELEKTR.MOD_CIM_REC_TYP;
    x66# DB_WEB_PORTAL.MOD_UGYFEL_REC_TYP;
    x67# DB_WEB_ELEKTR.MOD_CIM_REC_TYP;
    x68# DB_WEB_ELEKTR.ENGEDMENY_REC_TYP;
    x80# DB_GFB_PORTAL.KOTVENY_GFB_REC_TYP;
    x83# DB_GFB_PORTAL.GEPJARMU_REC_TYP;
    x95# DB_GFB_PORTAL.BONUSZ_ELOZMENY_REC_TYP;
    x110# DB_WEB_PORTAL.NEVSZAM_TAB_TYP;
  x110#_idx PLS_INTEGER := NULL;
  x110#nev VARCHAR2_80_tab_typ;
  x110#ertek NUMBER_12__2_tab_typ;
    x115# DB_WEB_ELEKTR.HIBA_TAB_TYP;
  x115#_idx PLS_INTEGER := NULL;
  x115#hibaszam NUMBER_9_tab_typ;
  x115#szoveg VARCHAR2_1000_tab_typ;
BEGIN

    x5#.dijkod := :x5#dijkod;
    x5#.dijfizmod := :x5#dijfizmod;
    x5#.dijfizgyak := :x5#dijfizgyak;
    x5#.szerkot := :x5#szerkot;
    x5#.szerlejar := :x5#szerlejar;
    x5#.kockezd := :x5#kockezd;
    x5#.btkezd := :x5#btkezd;
    x5#.halaszt_kockezd := :x5#halaszt_kockezd;
    x5#.halaszt_dijfiz := :x5#halaszt_dijfiz;
    x5#.szamlaszam := :x5#szamlaszam;
    x5#.szamla_limit := :x5#szamla_limit;
    x5#.evfordulo := :x5#evfordulo;
    x5#.evfordulo_tipus := :x5#evfordulo_tipus;
    x5#.e_komm_email := :x5#e_komm_email;
    x5#.dijbekerot_ker := :x5#dijbekerot_ker;
    x5#.ajanlati_evesdij := :x5#ajanlati_evesdij;

      x22#.nem := :x22#nem;
    x22#.ugyfelnev := :x22#ugyfelnev;
    x22#.szulnev := :x22#szulnev;
    x22#.anyanev := :x22#anyanev;
    x22#.szulhely := :x22#szulhely;
    x22#.szuldat := :x22#szuldat;
    x22#.adoszam := :x22#adoszam;
    x22#.adoaz := :x22#adoaz;
    x22#.aht_azon := :x22#aht_azon;
    x22#.szemelyaz := :x22#szemelyaz;
    x22#.jogositvany_kelte := :x22#jogositvany_kelte;
    x22#.tel_otthon := :x22#tel_otthon;
    x22#.tel_mhely := :x22#tel_mhely;
    x22#.tel_mobil := :x22#tel_mobil;
    x22#.email := :x22#email;
    x22#.tbazon := :x22#tbazon;
    x22#.vallalkozo := :x22#vallalkozo;
    x22#.okmany_tip := :x22#okmany_tip;
    x22#.okmanyszam := :x22#okmanyszam;
    x22#.adatkez := :x22#adatkez;
    x22#.cegkepviselo_neve := :x22#cegkepviselo_neve;
    x22#.cegforma := :x22#cegforma;
    x22#.cegjegyzekszam := :x22#cegjegyzekszam;
    x22#.allampolg := :x22#allampolg;
    x22#.tart_eng := :x22#tart_eng;
    x22#.publikus := :x22#publikus;

      x49#.ktid := :x49#ktid;
    x49#.hazszam1 := :x49#hazszam1;
    x49#.hazszam2 := :x49#hazszam2;
    x49#.epulet := :x49#epulet;
    x49#.lepcsohaz := :x49#lepcsohaz;
    x49#.emelet := :x49#emelet;
    x49#.ajto := :x49#ajto;
    x49#.hrsz := :x49#hrsz;
    x49#.kulf_orsz_kod := :x49#kulf_orsz_kod;
    x49#.kulf_irszam := :x49#kulf_irszam;
    x49#.kulf_helynev := :x49#kulf_helynev;
    x49#.kulf_utca := :x49#kulf_utca;
    x49#.kulf_hszajto := :x49#kulf_hszajto;
    x49#.kulf_egyeb := :x49#kulf_egyeb;
    x49#.kulf_pf := :x49#kulf_pf;

      x65#.ktid := :x65#ktid;
    x65#.hazszam1 := :x65#hazszam1;
    x65#.hazszam2 := :x65#hazszam2;
    x65#.epulet := :x65#epulet;
    x65#.lepcsohaz := :x65#lepcsohaz;
    x65#.emelet := :x65#emelet;
    x65#.ajto := :x65#ajto;
    x65#.hrsz := :x65#hrsz;
    x65#.kulf_orsz_kod := :x65#kulf_orsz_kod;
    x65#.kulf_irszam := :x65#kulf_irszam;
    x65#.kulf_helynev := :x65#kulf_helynev;
    x65#.kulf_utca := :x65#kulf_utca;
    x65#.kulf_hszajto := :x65#kulf_hszajto;
    x65#.kulf_egyeb := :x65#kulf_egyeb;
    x65#.kulf_pf := :x65#kulf_pf;

      x66#.nem := :x66#nem;
    x66#.ugyfelnev := :x66#ugyfelnev;
    x66#.szulnev := :x66#szulnev;
    x66#.anyanev := :x66#anyanev;
    x66#.szulhely := :x66#szulhely;
    x66#.szuldat := :x66#szuldat;
    x66#.adoszam := :x66#adoszam;
    x66#.adoaz := :x66#adoaz;
    x66#.aht_azon := :x66#aht_azon;
    x66#.szemelyaz := :x66#szemelyaz;
    x66#.jogositvany_kelte := :x66#jogositvany_kelte;
    x66#.tel_otthon := :x66#tel_otthon;
    x66#.tel_mhely := :x66#tel_mhely;
    x66#.tel_mobil := :x66#tel_mobil;
    x66#.email := :x66#email;
    x66#.tbazon := :x66#tbazon;
    x66#.vallalkozo := :x66#vallalkozo;
    x66#.okmany_tip := :x66#okmany_tip;
    x66#.okmanyszam := :x66#okmanyszam;
    x66#.adatkez := :x66#adatkez;
    x66#.cegkepviselo_neve := :x66#cegkepviselo_neve;
    x66#.cegforma := :x66#cegforma;
    x66#.cegjegyzekszam := :x66#cegjegyzekszam;
    x66#.allampolg := :x66#allampolg;
    x66#.tart_eng := :x66#tart_eng;
    x66#.publikus := :x66#publikus;

      x67#.ktid := :x67#ktid;
    x67#.hazszam1 := :x67#hazszam1;
    x67#.hazszam2 := :x67#hazszam2;
    x67#.epulet := :x67#epulet;
    x67#.lepcsohaz := :x67#lepcsohaz;
    x67#.emelet := :x67#emelet;
    x67#.ajto := :x67#ajto;
    x67#.hrsz := :x67#hrsz;
    x67#.kulf_orsz_kod := :x67#kulf_orsz_kod;
    x67#.kulf_irszam := :x67#kulf_irszam;
    x67#.kulf_helynev := :x67#kulf_helynev;
    x67#.kulf_utca := :x67#kulf_utca;
    x67#.kulf_hszajto := :x67#kulf_hszajto;
    x67#.kulf_egyeb := :x67#kulf_egyeb;
    x67#.kulf_pf := :x67#kulf_pf;

      x68#.engedm_kezdet := :x68#engedm_kezdet;
    x68#.engedm_veg := :x68#engedm_veg;
    x68#.vagyontargy := :x68#vagyontargy;
    x68#.engedm_osszeg := :x68#engedm_osszeg;
    x68#.also_limit := :x68#also_limit;
    x68#.felso_limit := :x68#felso_limit;
    x68#.penznem := :x68#penznem;
    x68#.szamlaszam := :x68#szamlaszam;
    x68#.hitel_szam := :x68#hitel_szam;
    x68#.fed_bejegy := :x68#fed_bejegy;
    x68#.bizt_nyil := :x68#bizt_nyil;

      x80#.bm_tipus := :x80#bm_tipus;
    x80#.kotes_oka := :x80#kotes_oka;

      x83#.jelleg := :x83#jelleg;
    x83#.rendszam := :x83#rendszam;
    x83#.teljesitmeny := :x83#teljesitmeny;
    x83#.ossztomeg := :x83#ossztomeg;
    x83#.ferohely := :x83#ferohely;
    x83#.uzjelleg := :x83#uzjelleg;
    x83#.alvazszam := :x83#alvazszam;
    x83#.gyartev := :x83#gyartev;
    x83#.gyartmany := :x83#gyartmany;
    x83#.tulajdon_ido := :x83#tulajdon_ido;
    x83#.tulajdon_visz := :x83#tulajdon_visz;

      x95#.zaro_bonusz := :x95#zaro_bonusz;
    x95#.kov_bonusz := :x95#kov_bonusz;
    x95#.rendszam := :x95#rendszam;
    x95#.bizt_kod := :x95#bizt_kod;
    x95#.kotvenyszam := :x95#kotvenyszam;
    x95#.torles_oka := :x95#torles_oka;
    x95#.szerkot := :x95#szerkot;
    x95#.szervege := :x95#szervege;

    x110#.DELETE;
  x110#nev.DELETE; x110#ertek.DELETE;

    x115#.DELETE;
  x115#szoveg.DELETE; x115#hibaszam.DELETE;

db_web.sendpreoffer_31101(p_sessionid=>:p_sessionid, p_lang=>:p_lang, p_vonalkod=>:p_vonalkod, p_kotveny=>x5#, p_szerzodo=>x22#, p_szerzodo_cim=>x49#, p_szerzodo_levelcim=>x65#, p_engedmenyezett=>x66#, p_engedmenyezett_cim=>x67#, p_engedmeny=>x68#, p_kotveny_gfb=>x80#, p_gepjarmu=>x83#, p_bonusz_elozmeny=>x95#, p_kedvezmenyek=>:p_kedvezmenyek, p_dump_args#=>:p_dump_args#, p_szerz_azon=>:p_szerz_azon, p_ajanlat_url=>:p_ajanlat_url, p_szamolt_dijtetelek=>x110#, p_evesdij=>:p_evesdij, p_hibalista=>x115#, p_hiba_kod=>:p_hiba_kod, p_hiba_szov=>:p_hiba_szov);

    :x5#dijkod := x5#.dijkod;
    :x5#dijfizmod := x5#.dijfizmod;
    :x5#dijfizgyak := x5#.dijfizgyak;
    :x5#szerkot := x5#.szerkot;
    :x5#szerlejar := x5#.szerlejar;
    :x5#kockezd := x5#.kockezd;
    :x5#btkezd := x5#.btkezd;
    :x5#halaszt_kockezd := x5#.halaszt_kockezd;
    :x5#halaszt_dijfiz := x5#.halaszt_dijfiz;
    :x5#szamlaszam := x5#.szamlaszam;
    :x5#szamla_limit := x5#.szamla_limit;
    :x5#evfordulo := x5#.evfordulo;
    :x5#evfordulo_tipus := x5#.evfordulo_tipus;
    :x5#e_komm_email := x5#.e_komm_email;
    :x5#dijbekerot_ker := x5#.dijbekerot_ker;
    :x5#ajanlati_evesdij := x5#.ajanlati_evesdij;

    :x80#bm_tipus := x80#.bm_tipus;
    :x80#kotes_oka := x80#.kotes_oka;

    :x83#jelleg := x83#.jelleg;
    :x83#rendszam := x83#.rendszam;
    :x83#teljesitmeny := x83#.teljesitmeny;
    :x83#ossztomeg := x83#.ossztomeg;
    :x83#ferohely := x83#.ferohely;
    :x83#uzjelleg := x83#.uzjelleg;
    :x83#alvazszam := x83#.alvazszam;
    :x83#gyartev := x83#.gyartev;
    :x83#gyartmany := x83#.gyartmany;
    :x83#tulajdon_ido := x83#.tulajdon_ido;
    :x83#tulajdon_visz := x83#.tulajdon_visz;

    x110#ertek.DELETE;
  x110#nev.DELETE;
  x110#_idx := x110#.FIRST;
  WHILE x110#_idx IS NOT NULL LOOP
    x110#nev(x110#_idx) := x110#(x110#_idx).nev;
    x110#ertek(x110#_idx) := x110#(x110#_idx).ertek;
    x110#_idx := x110#.NEXT(x110#_idx);
  END LOOP;
  :x110#nev := x110#nev;
  :x110#ertek := x110#ertek;

    x115#szoveg.DELETE;
  x115#hibaszam.DELETE;
  x115#_idx := x115#.FIRST;
  WHILE x115#_idx IS NOT NULL LOOP
    x115#hibaszam(x115#_idx) := x115#(x115#_idx).hibaszam;
    x115#szoveg(x115#_idx) := x115#(x115#_idx).szoveg;
    x115#_idx := x115#.NEXT(x115#_idx);
  END LOOP;
  :x115#hibaszam := x115#hibaszam;
  :x115#szoveg := x115#szoveg;


END;`
