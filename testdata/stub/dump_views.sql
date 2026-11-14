-- Reproduces the dictionary evidence committed next to this file:
--   oracle_views.txt   -- the stub package (ORACALL_STUB)
--   nested_types.txt   -- a package with nested PL/SQL types (NESTED_PROBE)
--
-- Usage:
--   sqlplus -S -L demo/demo@127.0.0.1:1521/FREEPDB1 @testdata/stub/dump_views.sql
--
-- Run testdata/stub/stub.sql first (it installs the ORACALL_STUB package).
SET PAGESIZE 0 LINESIZE 32767 FEEDBACK OFF HEADING OFF TRIMSPOOL ON WRAP OFF
COLUMN c FORMAT A300

PROMPT ===== USER_ARGUMENTS: package-level PL/SQL composite arguments =====
SELECT 'pkg='||package_name||' obj='||object_name||' seq='||sequence||' pos='||position||' arg='||NVL(argument_name,'-')
  ||' in_out='||in_out||' data_type='||data_type||' pls_type='||NVL(pls_type,'-')
  ||' type_owner='||NVL(type_owner,'-')||' type_name='||NVL(type_name,'-')
  ||' type_subname='||NVL(type_subname,'-')||' type_object_type='||NVL(type_object_type,'-')
  ||' data_level='||data_level||' defaulted='||NVL(TO_CHAR(defaulted),'-')
  c FROM user_arguments WHERE package_name='ORACALL_STUB' AND data_type LIKE 'PL/SQL%'
 ORDER BY object_name, sequence, position;

PROMPT ===== USER_ARGUMENTS: scalar arguments (for comparison) =====
SELECT 'pkg='||package_name||' obj='||object_name||' seq='||sequence||' pos='||position||' arg='||NVL(argument_name,'-')
  ||' in_out='||in_out||' data_type='||data_type||' pls_type='||NVL(pls_type,'-')
  ||' type_owner='||NVL(type_owner,'-')||' type_name='||NVL(type_name,'-')
  ||' data_level='||data_level
  c FROM user_arguments WHERE package_name='ORACALL_STUB' AND data_type NOT LIKE 'PL/SQL%'
 ORDER BY object_name, sequence, position;

PROMPT ===== USER_PLSQL_TYPES: package-level PL/SQL types ONLY =====
SELECT 'pkg='||package_name||' type='||type_name||' typecode='||typecode
  ||' attributes='||attributes||' contains_plsql='||contains_plsql
  c FROM user_plsql_types WHERE package_name='ORACALL_STUB' ORDER BY type_name;

PROMPT ===== USER_PLSQL_TYPE_ATTRS: fields of a package-level PL/SQL record =====
SELECT 'pkg='||package_name||' type='||type_name||' attr_no='||attr_no||' attr='||attr_name
  ||' attr_type_name='||NVL(attr_type_name,'-')||' attr_type_owner='||NVL(attr_type_owner,'-')
  ||' attr_type_package='||NVL(attr_type_package,'-')||' len='||NVL(TO_CHAR(length),'-')
  ||' prec='||NVL(TO_CHAR(precision),'-')||' scale='||NVL(TO_CHAR(scale),'-')
  ||' cs='||NVL(character_set_name,'-')
  c FROM user_plsql_type_attrs WHERE package_name='ORACALL_STUB' ORDER BY type_name, attr_no;

PROMPT ===== USER_PLSQL_COLL_TYPES: package-level PL/SQL collections =====
SELECT 'pkg='||package_name||' type='||type_name||' coll_type='||coll_type
  ||' elem_owner='||NVL(elem_type_owner,'-')||' elem_name='||NVL(elem_type_name,'-')
  ||' elem_package='||NVL(elem_type_package,'-')||' index_by='||NVL(index_by,'-')
  ||' upper_bound='||NVL(TO_CHAR(upper_bound),'-')||' len='||NVL(TO_CHAR(length),'-')
  ||' cs='||NVL(character_set_name,'-')||' nulls_stored='||NVL(nulls_stored,'-')
  c FROM user_plsql_coll_types WHERE package_name='ORACALL_STUB' ORDER BY type_name;

PROMPT ===== USER_TYPES: schema-level SQL object/collection types =====
SELECT 'type='||type_name||' typecode='||typecode||' attributes='||attributes
  ||' methods='||methods||' final='||NVL(TO_CHAR(final),'-')
  c FROM user_types WHERE type_name IN ('DEMO_OBJ_T','DEMO_OBJ_TAB_T','PAIR_PY') ORDER BY type_name;

PROMPT ===== USER_TYPE_ATTRS: fields of a schema-level SQL OBJECT type =====
SELECT 'type='||type_name||' attr_no='||attr_no||' attr='||attr_name
  ||' attr_type_name='||NVL(attr_type_name,'-')||' attr_type_owner='||NVL(attr_type_owner,'-')
  ||' len='||NVL(TO_CHAR(length),'-')||' prec='||NVL(TO_CHAR(precision),'-')
  ||' cs='||NVL(character_set_name,'-')
  c FROM user_type_attrs WHERE type_name IN ('DEMO_OBJ_T','PAIR_PY') ORDER BY type_name, attr_no;

PROMPT ===== USER_COLL_TYPES: schema-level SQL collection types =====
SELECT 'type='||type_name||' coll_type='||coll_type||' elem_owner='||NVL(elem_type_owner,'-')
  ||' elem_name='||NVL(elem_type_name,'-')||' upper_bound='||NVL(TO_CHAR(upper_bound),'-')
  ||' len='||NVL(TO_CHAR(length),'-')
  c FROM user_coll_types WHERE type_name IN ('DEMO_OBJ_TAB_T','TEST_CWO_TBL_T') ORDER BY type_name;

PROMPT ===== USER_ARGUMENTS: schema-level type arguments (constructor rows included!) =====
SELECT 'pkg='||NVL(package_name,'-')||' obj='||object_name||' arg='||NVL(argument_name,'-')
  ||' in_out='||in_out||' data_type='||data_type
  ||' type_owner='||NVL(type_owner,'-')||' type_name='||NVL(type_name,'-')
  ||' type_subname='||NVL(type_subname,'-')||' type_object_type='||NVL(type_object_type,'-')
  c FROM user_arguments WHERE type_object_type='TYPE' ORDER BY package_name, object_name, sequence;
