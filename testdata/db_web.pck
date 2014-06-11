CREATE OR REPLACE PACKAGE DB_web AS

  TYPE telephely_rec_typ IS RECORD (
    telephely_azon                    NUMBER(9),
    biztossz                          NUMBER(12,2),
    bet_lop_szazalek                  NUMBER(6,3),
    ertekelesi_mod                    VARCHAR2(255),
    telephely_funkcio                 VARCHAR2(255),
    kockviseles_hely                  VARCHAR2(1000));
  TYPE telephely_tab_typ IS TABLE OF telephely_rec_typ INDEX BY BINARY_INTEGER;

  TYPE telephely_cur_typ IS REF CURSOR RETURN telephely_rec_typ;

  TYPE telephelyadatok_rec_typ IS RECORD (
      telephely_azon                  NUMBER(9),
      bizt_osszeg                     NUMBER(12,2),
      bizt_vagyontargy                VARCHAR2(255));
  TYPE telephelyadatok_tab_typ IS TABLE OF telephelyadatok_rec_typ INDEX BY BINARY_INTEGER;

  TYPE telephelyadatok_cur_typ IS REF CURSOR RETURN telephelyadatok_rec_typ;

  TYPE get_kedv_rec_typ IS RECORD (
    kedv_azonosito                    VARCHAR2(6),
    kedv_neve                         VARCHAR2(40),
    kedv_szazalek                     NUMBER(6,3),
    kedv_forint                       NUMBER);
  TYPE get_kedv_tab_typ IS TABLE OF get_kedv_rec_typ INDEX BY BINARY_INTEGER;

  TYPE vagyon_kedv_cur_typ IS REF CURSOR RETURN get_kedv_rec_typ;

  TYPE fedezet_rec_typ IS RECORD (
      vagyon_szint                    CHAR(2),
      telephely_azon                  NUMBER,
      fedezet_azon                    VARCHAR2(6),
      fedezet_neve                    VARCHAR2(255),
      prioritas                       BINARY_INTEGER);
  TYPE fedezet_tab_typ IS TABLE OF fedezet_rec_typ INDEX BY BINARY_INTEGER;

  TYPE fedezet_cur_typ IS REF CURSOR RETURN fedezet_rec_typ;

  TYPE fedezetadat_rec_typ IS RECORD (
    modkod                            VARCHAR2(7),
    --modozatnev                        VARCHAR2(100),
    telep_azon                        NUMBER(9),
    kieg_azon                         NUMBER(9),
    tetel_nev                         VARCHAR2(100),
    ertek                             VARCHAR2(1000),
    mertekegyseg                      VARCHAR2(6));

  TYPE fedezetadat_tab_typ IS TABLE OF fedezetadat_rec_typ INDEX BY BINARY_INTEGER;

  TYPE fedezetadat_cur_typ IS REF CURSOR RETURN fedezetadat_rec_typ;

  PROCEDURE login(p_login_nev IN VARCHAR2, p_jelszo IN VARCHAR2, p_lang IN VARCHAR2, p_addr# IN VARCHAR2,
                  p_sessionid OUT VARCHAR2, p_jogcsoport OUT VARCHAR2, p_dazon OUT VARCHAR2,
                  p_ugyfelnev OUT VARCHAR2, p_torzsszam OUT VARCHAR2, p_email OUT VARCHAR2,
                  p_tel_mobil OUT VARCHAR2, 
                  p_hiba_kod OUT PLS_INTEGER, p_hiba_szov OUT VARCHAR2);

  PROCEDURE logout(p_sessionid IN VARCHAR2);

  PROCEDURE GetRiskVagyonDetails(p_sessionid IN VARCHAR2,
      p_szerz_azon IN PLS_INTEGER,
      p_get_telephely OUT telephely_cur_typ,
      p_get_telephelyadatok OUT telephelyadatok_cur_typ,
      p_get_kedv OUT vagyon_kedv_cur_typ,
      p_get_fedezet OUT fedezet_cur_typ,
      p_get_fedezetadat OUT fedezetadat_cur_typ,
      p_hiba_kod OUT PLS_INTEGER, p_hiba_szov OUT VARCHAR2);

END DB_web;
/

CREATE OR REPLACE PACKAGE BODY DB_web AS

  PROCEDURE login(p_login_nev IN VARCHAR2, p_jelszo IN VARCHAR2, p_lang IN VARCHAR2, p_addr# IN VARCHAR2,
                  p_sessionid OUT VARCHAR2, p_jogcsoport OUT VARCHAR2, p_dazon OUT VARCHAR2,
                  p_ugyfelnev OUT VARCHAR2, p_torzsszam OUT VARCHAR2, p_email OUT VARCHAR2,
                  p_tel_mobil OUT VARCHAR2, 
                  p_hiba_kod OUT PLS_INTEGER, p_hiba_szov OUT VARCHAR2) IS
  BEGIN
    p_sessionid := 'NONE';
    p_hiba_kod := 0; p_hiba_szov := NULL;
  END login;

  PROCEDURE logout(p_sessionid IN VARCHAR2) IS
  BEGIN
    NULL;
  END logout;

  PROCEDURE GetRiskVagyonDetails(p_sessionid IN VARCHAR2,
      p_szerz_azon IN PLS_INTEGER,
      p_get_telephely OUT telephely_cur_typ,
      p_get_telephelyadatok OUT telephelyadatok_cur_typ,
      p_get_kedv OUT vagyon_kedv_cur_typ,
      p_get_fedezet OUT fedezet_cur_typ,
      p_get_fedezetadat OUT fedezetadat_cur_typ,
      p_hiba_kod OUT PLS_INTEGER, p_hiba_szov OUT VARCHAR2) IS
  BEGIN
    p_hiba_kod := 0; p_hiba_szov := NULL;
  END GetRiskVagyonDetails;

END DB_web;
/
