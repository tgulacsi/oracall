# OraCall
OraCall is a package for calling Oracle stored procedures through OCI.
As OCI does not allow using PL/SQL record types, this causes some difficulty.

    SELECT object_id, subprogram_id, package_name, object_name,
           data_level, position, argument_name, in_out,
           data_type, data_precision, data_scale, character_set_name,
           pls_type, char_length, type_owner, type_name, type_subname, type_link
      FROM user_arguments
      ORDER BY object_id, subprogram_id, SEQUENCE;
