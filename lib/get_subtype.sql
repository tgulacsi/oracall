--ALTER PACKAGE DB_web COMPILE plscope_settings = 'identifiers:all';
SELECT B.object_name, C.name, B.name
  FROM user_identifiers A
    INNER JOIN user_identifiers B ON B.declared_object_type = A.declared_object_type AND B.object_type = A.object_type AND B.object_name = A.objecT_name AND B.usage_id = A.usage_context_id
    INNER JOIN user_identifiers C ON C.declared_object_type = A.declared_object_type AND C.object_name = B.object_name AND C.object_type = B.object_type AND C.usage_id = B.usage_context_id
  WHERE C.usage = 'DECLARATION' AND
        B.type = 'FORMAL IN' AND B.usage = 'DECLARATION' AND
        A.declared_object_type = 'PACKAGE' AND
        A.object_name = 'DB_WEB' AND A.type = 'SUBTYPE' AND A.USAGE = 'REFERENCE' AND
        A.NAME = 'PASSWORD_T';

