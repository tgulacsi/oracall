package structs

import (
	"testing"

	"github.com/kylelemons/godebug/diff"

	//"github.com/kylelemons/godebug/diff"
)

func TestOne(t *testing.T) {
	for i, tc := range testCases {
		functions := tc.ParseCsv(t, i)
		got, _ := functions[0].PlsqlBlock()
		d := diff.Diff(tc.PlSql, got)
		if d != "" {
			//FIXME(tgulacsi): this should be an error!
			t.Logf("plsql block diff:\n" + d)
			//fmt.Printf("GOT:\n", got)
		}
	}
}
