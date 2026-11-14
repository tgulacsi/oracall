// Copyright 2026
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package oracall

// isNullablePLSInteger identifies singular Oracle integer scalars; repeated
// elements need a separate nullable-message representation.
func isNullablePLSInteger(arg Argument) bool {
	if !NullablePLSInteger || arg.Flavor != FLAVOR_SIMPLE {
		return false
	}
	switch arg.Type {
	case "PLS_INTEGER", "BINARY_INTEGER", "PL/SQL PLS INTEGER", "PL/SQL BINARY INTEGER":
		return true
	}
	return false
}
