# OraCall
OraCall is a package for calling Oracle stored procedures through OCI.
As OCI does not allow using PL/SQL record types, this causes some difficulty.

    SELECT object_id, subprogram_id, package_name, object_name,
           data_level, position, argument_name, in_out,
           data_type, data_precision, data_scale, character_set_name,
           pls_type, char_length, type_owner, type_name, type_subname, type_link
      FROM user_arguments
      ORDER BY object_id, subprogram_id, SEQUENCE;

## Examples
### Minimal
Minimal is a minimal example using OraCall: a simple main package which
connects to the database and calls the function specified on the command line.
Args are specified using JSON.

This must be compiled with your oracall output - such as

    oracall <one.csv >examples/minimal/generated_functions.go \
    && go fmt ./examples/minimal/ \
    && go build ./examples/minimal/ \
    && ./minimal DB_web.sendpreoffer_31101

This calls oracall with one.csv as stdin, and redirects its output to
examples/minimal/generated_functions.go.
Then formats the output (good for safety check, too)
after this it builds the "minimal" binary (into the current dir).
If all this was successful, than calls the resulting binary
to call the specified function.
