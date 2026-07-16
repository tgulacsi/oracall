// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package orasrv

import "testing"

func TestStripJSON(t *testing.T) {
	for in, want := range map[string]string{
		`{"p_login_nev":"b1000392","p_jelszo":"LTeZMyQG6M4znBePYHHN","p_addr":"CALLER_SYSTEM__"}`: `{"p_login_nev":"b1000392","p_jelszo":"********************","p_addr":"CALLER_SYSTEM__"}`,
		`{"p_login_nev":"b1000392","password":"LTeZMyQG6M4znBePYHHN","p_addr":"CALLER_SYSTEM__"}`: `{"p_login_nev":"b1000392","password":"LTeZMyQG6M4znBePYHHN","p_addr":"CALLER_SYSTEM__"}`,
	} {
		if got := stripJSON(in, "p_jelszo"); got != want {
			t.Errorf("got %s,\n wanted\n %s", got, want)
		}
	}
}
