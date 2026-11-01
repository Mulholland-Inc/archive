// Package pgx stands in for github.com/jackc/pgx/v4, which is unmaintained and has unfixed vulnerabilities
// (GO-2026-5004, and GO-2026-4518 in its pgproto3/v2 dependency). The only code in the archive build that
// imports it is pg-schema-diff's pkg/tempdb, which uses Identifier.Sanitize and blank-imports stdlib to
// register a database/sql driver. The archive opens every connection through pgx v5's stdlib (store.go), so
// this module provides just Identifier, copied from pgx v4.18.2 (MIT, Copyright (c) 2013-2021 Jack
// Christensen), and an empty stdlib. src/go.mod replaces pgx v4 with it.
package pgx

import "strings"

// Identifier is a PostgreSQL identifier or name. Identifiers can be composed of multiple parts such as
// ["schema", "table"] or ["table", "column"].
type Identifier []string

// Sanitize returns a sanitized string safe for SQL interpolation.
func (ident Identifier) Sanitize() string {
	parts := make([]string, len(ident))
	for i := range ident {
		s := strings.ReplaceAll(ident[i], string([]byte{0}), "")
		parts[i] = `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return strings.Join(parts, ".")
}
