-- Stub package for the OraCall test suite.
--
-- It exercises every argument shape the main generator supports today:
--   * simple scalars (IN / OUT / IN OUT and a function return);
--   * arrays of simple types (INDEX BY PLS_INTEGER, indexed from 1);
--   * records of simple types;
--   * arrays of records of simple types.
--
-- Deliberately absent, because they are unsupported:
--   * array of array;
--   * an array inside a record;
--   * nested records.
--
-- Install with the statements below (each terminated by a line containing only
-- '/') before running `oracall call 'ORACALL_STUB.%'`.

CREATE OR REPLACE PACKAGE ORACALL_STUB AS
  TYPE num_tab IS TABLE OF NUMBER INDEX BY PLS_INTEGER;
  TYPE txt_tab IS TABLE OF VARCHAR2(100) INDEX BY PLS_INTEGER;
  TYPE simple_rec IS RECORD (num NUMBER, txt VARCHAR2(100), dt DATE);
  TYPE simple_rec_tab IS TABLE OF simple_rec INDEX BY PLS_INTEGER;

  -- simple scalars
  FUNCTION f_char_echo(txt IN VARCHAR2) RETURN VARCHAR2;
  PROCEDURE p_char_inout(txt IN OUT VARCHAR2);
  PROCEDURE p_num_inout(num IN OUT NUMBER);
  FUNCTION f_num_add(a IN NUMBER, b IN NUMBER) RETURN NUMBER;
  PROCEDURE p_date_out(dt OUT DATE);

  -- arrays of simple types
  FUNCTION f_num_sum(nums IN num_tab) RETURN NUMBER;
  PROCEDURE p_num_double(nums IN num_tab, outnums OUT num_tab);
  FUNCTION f_txt_count(txts IN txt_tab) RETURN PLS_INTEGER;

  -- records of simple types
  FUNCTION f_rec_echo(rec IN simple_rec) RETURN VARCHAR2;
  PROCEDURE p_rec_out(num OUT NUMBER, txt OUT VARCHAR2, dt OUT DATE);

  -- arrays of records of simple types
  FUNCTION f_rec_tab_count(tab IN simple_rec_tab) RETURN PLS_INTEGER;
  PROCEDURE p_rec_tab_out(tab OUT simple_rec_tab);
END ORACALL_STUB;
/

CREATE OR REPLACE PACKAGE BODY ORACALL_STUB AS
  FUNCTION f_char_echo(txt IN VARCHAR2) RETURN VARCHAR2 IS
  BEGIN
    RETURN txt;
  END f_char_echo;

  PROCEDURE p_char_inout(txt IN OUT VARCHAR2) IS
  BEGIN
    txt := UPPER(txt) || '#';
  END p_char_inout;

  PROCEDURE p_num_inout(num IN OUT NUMBER) IS
  BEGIN
    num := num + 1;
  END p_num_inout;

  FUNCTION f_num_add(a IN NUMBER, b IN NUMBER) RETURN NUMBER IS
  BEGIN
    RETURN a + b;
  END f_num_add;

  PROCEDURE p_date_out(dt OUT DATE) IS
  BEGIN
    dt := DATE '2024-01-02';
  END p_date_out;

  FUNCTION f_num_sum(nums IN num_tab) RETURN NUMBER IS
    i PLS_INTEGER;
    s NUMBER := 0;
  BEGIN
    i := nums.FIRST;
    WHILE i IS NOT NULL LOOP
      s := s + NVL(nums(i), 0);
      i := nums.NEXT(i);
    END LOOP;
    RETURN s;
  END f_num_sum;

  PROCEDURE p_num_double(nums IN num_tab, outnums OUT num_tab) IS
    i PLS_INTEGER;
  BEGIN
    outnums.DELETE;
    i := nums.FIRST;
    WHILE i IS NOT NULL LOOP
      outnums(i) := NVL(nums(i), 0) * 2;
      i := nums.NEXT(i);
    END LOOP;
  END p_num_double;

  FUNCTION f_txt_count(txts IN txt_tab) RETURN PLS_INTEGER IS
  BEGIN
    RETURN txts.COUNT;
  END f_txt_count;

  FUNCTION f_rec_echo(rec IN simple_rec) RETURN VARCHAR2 IS
  BEGIN
    RETURN rec.num || ';' || NVL(rec.txt, '') || ';' ||
           TO_CHAR(rec.dt, 'YYYY-MM-DD');
  END f_rec_echo;

  PROCEDURE p_rec_out(num OUT NUMBER, txt OUT VARCHAR2, dt OUT DATE) IS
  BEGIN
    num := 42;
    txt := 'hello';
    dt := DATE '2024-01-02';
  END p_rec_out;

  FUNCTION f_rec_tab_count(tab IN simple_rec_tab) RETURN PLS_INTEGER IS
  BEGIN
    RETURN tab.COUNT;
  END f_rec_tab_count;

  PROCEDURE p_rec_tab_out(tab OUT simple_rec_tab) IS
  BEGIN
    tab.DELETE;
    tab(1).num := 1;
    tab(1).txt := 'a';
    tab(1).dt := DATE '2024-01-02';
    tab(2).num := 2;
    tab(2).txt := 'b';
    tab(2).dt := DATE '2024-01-03';
  END p_rec_tab_out;
END ORACALL_STUB;
/
