// Copyright 2024, 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

package objects

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	oracall "github.com/tgulacsi/oracall/lib"
)

// ProcDirection represents the direction of a stored procedure parameter.
type ProcDirection string

const (
	DirIn    ProcDirection = "IN"
	DirOut   ProcDirection = "OUT"
	DirInOut ProcDirection = "IN/OUT"
)

// ProcParameter is a parameter of a stored procedure.
type ProcParameter struct {
	Name      string
	Direction ProcDirection
	OraType   string // e.g. "VARCHAR2", "NUMBER(9)", "BRUNO_OWNER.BZS_XLSX.WORKBOOK_RT"
}

// Procedure is a stored procedure with its parameters.
type Procedure struct {
	Owner, Package, Name string
	Parameters           []ProcParameter
}

// Procedures is a list of stored procedures.
type Procedures struct {
	Items []Procedure
}

// FullName returns the fully-qualified procedure name for SQL.
func (p Procedure) FullName() string {
	if p.Package != "" {
		if p.Owner != "" {
			return p.Owner + "." + p.Package + "." + p.Name
		}
		return p.Package + "." + p.Name
	}
	if p.Owner != "" {
		return p.Owner + "." + p.Name
	}
	return p.Name
}

// ReadProcedures queries ALL_ARGUMENTS and returns procedure metadata for the current schema.
// If funcNames is provided, only those procedures are returned.
func ReadProcedures(ctx context.Context, db querier, funcNames ...string) (*Procedures, error) {
	const qry = `
SELECT a.owner, a.package_name, a.object_name,
       a.argument_name, a.in_out,
       CASE
         WHEN a.type_owner IS NOT NULL THEN
           a.type_owner || '.' ||
           CASE WHEN a.type_subname IS NOT NULL
                THEN a.type_name || '.' || a.type_subname
                ELSE a.type_name
           END
         WHEN a.pls_type IS NOT NULL THEN a.pls_type
         ELSE a.data_type
       END AS ora_type,
       a.position
FROM all_arguments a
WHERE a.owner = SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA')
  AND a.argument_name IS NOT NULL
ORDER BY a.object_id, a.overload, a.subprogram_id, a.sequence`

	rows, err := db.QueryContext(ctx, qry)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", qry, err)
	}
	defer rows.Close()

	var procs Procedures
	var currentKey string
	var currentProc *Procedure

	for rows.Next() {
		var owner, packageName, objectName, argName, inOut, oraType string
		var pos int
		if err := rows.Scan(&owner, &packageName, &objectName, &argName, &inOut, &oraType, &pos); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if len(funcNames) > 0 {
			var found bool
			for _, fn := range funcNames {
				if strings.EqualFold(fn, objectName) || strings.EqualFold(fn, packageName+"."+objectName) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		key := owner + "." + packageName + "." + objectName
		if key != currentKey {
			if currentProc != nil {
				procs.Items = append(procs.Items, *currentProc)
			}
			currentProc = &Procedure{Owner: owner, Package: packageName, Name: objectName}
			currentKey = key
		}
		currentProc.Parameters = append(currentProc.Parameters, ProcParameter{
			Name:      argName,
			Direction: ProcDirection(inOut),
			OraType:   oraType,
		})
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if currentProc != nil {
		procs.Items = append(procs.Items, *currentProc)
	}
	return &procs, nil
}

// goParamType returns the Go type for a procedure parameter.
func goParamType(p ProcParameter, types *Types) string {
	if t := resolveType(p.OraType, types); t != nil {
		return "*" + t.ProtoMessageName()
	}
	return oraTypeToGoType(p.OraType)
}

// resolveType looks up an OraType string (e.g. "BRUNO_OWNER.BZS_XLSX.WORKBOOK_RT") in the
// types registry. Returns nil for simple/scalar types.
func resolveType(oraType string, types *Types) *Type {
	if types == nil {
		return nil
	}
	types.mu.RLock()
	t := types.m[oraType]
	types.mu.RUnlock()
	if t != nil && t.composite() {
		return t
	}
	return nil
}

// oraTypeToGoType maps Oracle simple type names to Go types.
func oraTypeToGoType(oraType string) string {
	base := oraType
	if i := strings.IndexByte(oraType, '('); i >= 0 {
		base = oraType[:i]
	}
	switch strings.TrimSpace(strings.ToUpper(base)) {
	case "VARCHAR2", "CHAR", "NVARCHAR2", "NCHAR", "CLOB", "LONG", "XMLTYPE":
		return "string"
	case "NUMBER", "FLOAT":
		// Could be int, int64, or string depending on precision; default to string
		return "string"
	case "INTEGER", "PLS_INTEGER", "BINARY_INTEGER", "SIMPLE_INTEGER":
		return "int32"
	case "BINARY_DOUBLE", "BINARY_FLOAT":
		return "float64"
	case "DATE", "TIMESTAMP", "TIMESTAMP WITH TIME ZONE", "TIMESTAMP WITH LOCAL TIME ZONE":
		return "time.Time"
	case "BOOLEAN":
		return "bool"
	case "RAW", "LONG RAW", "BLOB":
		return "[]byte"
	default:
		return "string" // fallback
	}
}

// needsTimeImport reports whether any parameter uses time.Time.
func (p Procedure) needsTimeImport(types *Types) bool {
	for _, param := range p.Parameters {
		if resolveType(param.OraType, types) == nil && oraTypeToGoType(param.OraType) == "time.Time" {
			return true
		}
	}
	return false
}

// goInputStructName returns the Go input struct name for this procedure.
func (p Procedure) goInputStructName() string {
	return oracall.CamelCase(strings.ReplaceAll(p.Package+"_"+p.Name, ".", "_")) + "Input"
}

// goOutputStructName returns the Go output struct name for this procedure.
func (p Procedure) goOutputStructName() string {
	return oracall.CamelCase(strings.ReplaceAll(p.Package+"_"+p.Name, ".", "_")) + "Output"
}

// goFuncName returns the exported Go function name for this procedure.
func (p Procedure) goFuncName() string {
	return oracall.CamelCase(strings.ReplaceAll(p.Package+"_"+p.Name, ".", "_"))
}

// constName returns the Go const name for the PL/SQL call text.
func (p Procedure) constName() string {
	return strings.ToLower(strings.ReplaceAll(p.Package+"_"+p.Name, ".", "_")) + "_plsql"
}

// WriteProcedureWrapper generates Go code for calling this stored procedure
// using godror Object binding for complex parameters.
// types is used to look up Oracle object types; pkg is the Go package name.
func (p Procedure) WriteProcedureWrapper(ctx context.Context, w io.Writer, types *Types) error {
	bw := bufio.NewWriter(w)

	// Resolve types once to avoid repeated RLock calls per parameter.
	typeMap := make(map[string]*Type, len(p.Parameters))
	for _, param := range p.Parameters {
		if t := resolveType(param.OraType, types); t != nil {
			typeMap[param.OraType] = t
		}
	}
	resolve := func(oraType string) *Type { return typeMap[oraType] }

	// Classify parameters
	var inParams, outParams, inoutParams []ProcParameter
	for _, param := range p.Parameters {
		switch param.Direction {
		case DirIn:
			inParams = append(inParams, param)
		case DirOut:
			outParams = append(outParams, param)
		default: // IN/OUT
			inoutParams = append(inoutParams, param)
		}
	}

	// PL/SQL call text constant: BEGIN pkg.proc(:P1, :P2, ...); END;
	var callArgs strings.Builder
	for i, param := range p.Parameters {
		if i > 0 {
			callArgs.WriteString(", ")
		}
		fmt.Fprintf(&callArgs, "%s=>:%s", param.Name, param.Name)
	}
	fmt.Fprintf(bw, "\nconst %s = `BEGIN %s(%s); END;`\n\n", p.constName(), p.FullName(), callArgs.String())

	// Input struct
	hasInput := len(inParams)+len(inoutParams) > 0
	hasOutput := len(outParams)+len(inoutParams) > 0

	if hasInput {
		fmt.Fprintf(bw, "type %s struct {\n", p.goInputStructName())
		for _, param := range append(inParams, inoutParams...) {
			fmt.Fprintf(bw, "\t%s %s\n", oracall.CamelCase(param.Name), goParamType(param, types))
		}
		bw.WriteString("}\n\n")
	}

	// Output struct
	if hasOutput {
		fmt.Fprintf(bw, "type %s struct {\n", p.goOutputStructName())
		for _, param := range append(outParams, inoutParams...) {
			fmt.Fprintf(bw, "\t%s %s\n", oracall.CamelCase(param.Name), goParamType(param, types))
		}
		bw.WriteString("}\n\n")
	}

	// Wrapper function signature
	inputArg := ""
	if hasInput {
		inputArg = ", input *" + p.goInputStructName()
	}
	returnType := "error"
	if hasOutput {
		returnType = "(*" + p.goOutputStructName() + ", error)"
	}
	fmt.Fprintf(bw, "func %s(ctx context.Context, db godror.Execer%s) (%s) {\n",
		p.goFuncName(), inputArg, returnType)

	errReturn := func(expr string) string {
		if hasOutput {
			return "return nil, " + expr
		}
		return "return " + expr
	}

	// Declare variables for each param (IN, INOUT, OUT order)
	allParams := append(append(inParams, inoutParams...), outParams...)
	for _, param := range allParams {
		varName := strings.ToLower(param.Name)
		t := resolve(param.OraType)
		if t != nil {
			// complex type: will use *godror.Object
			fmt.Fprintf(bw, "\tvar %sObj *godror.Object\n", varName)
		} else {
			// simple type
			goType := oraTypeToGoType(param.OraType)
			var zeroVal string
			switch goType {
			case "string":
				zeroVal = `""`
			case "int32":
				zeroVal = "0"
			case "int64":
				zeroVal = "0"
			case "float64":
				zeroVal = "0"
			case "bool":
				zeroVal = "false"
			default:
				zeroVal = "nil"
			}
			fmt.Fprintf(bw, "\tvar %s %s = %s\n", varName, goType, zeroVal)
		}
	}

	// Set IN / INOUT object params from input struct
	for _, param := range append(inParams, inoutParams...) {
		varName := strings.ToLower(param.Name)
		t := resolve(param.OraType)
		if t == nil {
			// simple: just copy from input
			if hasInput {
				fmt.Fprintf(bw, "\t%s = input.%s\n", varName, oracall.CamelCase(param.Name))
			}
			continue
		}
		// complex: create Object, call WriteObject
		fmt.Fprintf(bw, "\tif input.%s != nil {\n", oracall.CamelCase(param.Name))
		fmt.Fprintf(bw, "\t\t%sType, err := godror.GetObjectType(ctx, db, %q)\n", varName, param.OraType)
		fmt.Fprintf(bw, "\t\tif err != nil { %s }\n", errReturn(fmt.Sprintf("fmt.Errorf(\"GetObjectType %s: %%w\", err)", param.Name)))
		fmt.Fprintf(bw, "\t\t%sObj, err = %sType.NewObject()\n", varName, varName)
		fmt.Fprintf(bw, "\t\tif err != nil { %s }\n", errReturn(fmt.Sprintf("fmt.Errorf(\"NewObject %s: %%w\", err)", param.Name)))
		fmt.Fprintf(bw, "\t\tdefer %sObj.Close()\n", varName)
		fmt.Fprintf(bw, "\t\tif err = input.%s.WriteObject(%sObj); err != nil { %s }\n",
			oracall.CamelCase(param.Name), varName,
			errReturn(fmt.Sprintf("fmt.Errorf(\"WriteObject %s: %%w\", err)", param.Name)))
		fmt.Fprintf(bw, "\t}\n")
	}

	// Build ExecContext named params
	bw.WriteString("\t_, err := db.ExecContext(ctx, " + p.constName())
	for _, param := range p.Parameters {
		varName := strings.ToLower(param.Name)
		t := resolve(param.OraType)
		switch param.Direction {
		case DirIn:
			if t != nil {
				fmt.Fprintf(bw, ",\n\t\tsql.Named(%q, %sObj)", param.Name, varName)
			} else {
				fmt.Fprintf(bw, ",\n\t\tsql.Named(%q, %s)", param.Name, varName)
			}
		case DirOut:
			if t != nil {
				fmt.Fprintf(bw, ",\n\t\tsql.Named(%q, sql.Out{Dest: &%sObj})", param.Name, varName)
			} else {
				fmt.Fprintf(bw, ",\n\t\tsql.Named(%q, sql.Out{Dest: &%s})", param.Name, varName)
			}
		default: // IN/OUT
			if t != nil {
				fmt.Fprintf(bw, ",\n\t\tsql.Named(%q, sql.Out{Dest: &%sObj, In: true})", param.Name, varName)
			} else {
				fmt.Fprintf(bw, ",\n\t\tsql.Named(%q, sql.Out{Dest: &%s, In: true})", param.Name, varName)
			}
		}
	}
	bw.WriteString(")\n")
	bw.WriteString("\tif err != nil { " + errReturn("err") + " }\n")

	if !hasOutput {
		bw.WriteString("\treturn nil\n}\n")
		return bw.Flush()
	}

	// Build output struct
	fmt.Fprintf(bw, "\toutput := &%s{}\n", p.goOutputStructName())
	for _, param := range append(outParams, inoutParams...) {
		varName := strings.ToLower(param.Name)
		fieldName := oracall.CamelCase(param.Name)
		t := resolve(param.OraType)
		if t != nil {
			fmt.Fprintf(bw, "\tif %sObj != nil {\n", varName)
			fmt.Fprintf(bw, "\t\toutput.%s = new(%s)\n", fieldName, t.ProtoMessageName())
			fmt.Fprintf(bw, "\t\tif err = output.%s.Scan(%sObj); err != nil { return nil, err }\n", fieldName, varName)
			fmt.Fprintf(bw, "\t}\n")
		} else {
			fmt.Fprintf(bw, "\toutput.%s = %s\n", fieldName, varName)
		}
	}
	bw.WriteString("\treturn output, nil\n}\n")
	return bw.Flush()
}

// WriteProcedures writes Go code for all procedures in the list.
// The caller must write the file header (package, imports) before calling this.
func (ps *Procedures) WriteProcedures(ctx context.Context, w io.Writer, types *Types) error {
	for _, p := range ps.Items {
		if err := p.WriteProcedureWrapper(ctx, w, types); err != nil {
			return fmt.Errorf("%s: %w", p.FullName(), err)
		}
	}
	return nil
}

// needsTimeImportAny returns true if any procedure needs the time package.
func (ps *Procedures) needsTimeImportAny(types *Types) bool {
	for _, p := range ps.Items {
		if p.needsTimeImport(types) {
			return true
		}
	}
	return false
}

// WriteProceduresFile writes a complete Go file for all procedures.
func (ps *Procedures) WriteProceduresFile(ctx context.Context, w io.Writer, pkg string, types *Types) error {
	bw := bufio.NewWriter(w)
	bw.WriteString("// File generated by oracall. DO NOT EDIT.\n\n")
	fmt.Fprintf(bw, "package %s\n\n", pkg)
	needsTime := ps.needsTimeImportAny(types)
	bw.WriteString("import (\n")
	bw.WriteString("\t\"context\"\n")
	bw.WriteString("\t\"database/sql\"\n")
	bw.WriteString("\t\"fmt\"\n")
	if needsTime {
		bw.WriteString("\t\"time\"\n")
	}
	bw.WriteString("\n\t\"github.com/godror/godror\"\n")
	bw.WriteString(")\n\n")
	bw.WriteString("var _ = sql.ErrNoRows // avoid unused import\n")
	if needsTime {
		bw.WriteString("var _ = time.Time{} // avoid unused import\n")
	}
	bw.WriteString("\n")
	if err := bw.Flush(); err != nil {
		return err
	}
	return ps.WriteProcedures(ctx, w, types)
}
