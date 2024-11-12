package ORA21062_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/godror/godror"
)

func setup(ctx context.Context, db *sql.DB, t *testing.T) error {
	var errs []error
	for _, qry := range []string{
		`CREATE OR REPLACE PACKAGE test_ora21062 IS
  SUBTYPE vc30 IS VARCHAR2(30);
  TYPE font_rt IS RECORD (
    Bold         BOOLEAN,
    Italic       BOOLEAN,
    Underline    vc30,
    Family       vc30,
    Size_         BINARY_DOUBLE,
    Strike       BOOLEAN,
    Color        vc30,
    ColorIndexed PLS_INTEGER,
    ColorTheme   PLS_INTEGER,
    ColorTint    BINARY_DOUBLE,
    VertAlign    vc30
  );
TYPE rich_text_rt IS RECORD (font font_rt, text VARCHAR2(1000));

END;`,
	} {
		t.Log(qry)
		if _, err := db.ExecContext(ctx, qry); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", qry, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	if cErrs, err := godror.GetCompileErrors(ctx, db, false); err != nil {
		return err
	} else {
		for _, e := range cErrs {
			if e.Name == "TEST_ORA21062" {
				errs = append(errs, fmt.Errorf("%+v", e))
			}
		}
		if len(errs) != 0 {
			return errors.Join(errs...)
		}
	}
	return nil
}
