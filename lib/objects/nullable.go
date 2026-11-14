// Copyright 2026
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package objects

import (
	"strings"

	oracall "github.com/tgulacsi/oracall/lib"
)

func isNullablePLSIntegerType(name string) bool {
	if !oracall.NullablePLSInteger {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "PLS_INTEGER", "BINARY_INTEGER", "PL/SQL PLS INTEGER", "PL/SQL BINARY INTEGER":
		return true
	}
	return false
}
