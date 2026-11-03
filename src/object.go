package main

// Objects: every row has a name. Each table of the public schema inherits archive.object (archive.sql), which
// gives it id, name, access, created_at and updated_at, and each of Archive's own tables the catalog shows does
// too, named as it is written (archive.object_name). name is what every screen titles a row by, and every link
// to it, so no screen falls back on a column it guesses or on the row's id. Check holds every table and view
// to it, and schemaDerive gives a new table what it lacks of it.

// objectText says whether c can be the name archive.object gives a table: plain text, written, not computed.
func objectText(c *Column) bool {
	return c.Schema == "pg_catalog" && c.Type == "text" && !c.List && !c.Generated
}

// objectFits says whether t can inherit archive.object once schemaDerive added what it lacks: each column it
// has of object's is of object's type, NOT NULL where object's is.
func objectFits(t *Table) bool {
	for _, w := range []struct {
		name, typ string
		notNull   bool
	}{{"id", "uuid", true}, {"name", "text", false}, {"access", "uuid", false}, {"created_at", "timestamptz", true}, {"updated_at", "timestamptz", true}} {
		c := t.col(w.name)
		if c != nil && (c.Schema != "pg_catalog" || c.Type != w.typ || c.List || c.Generated || w.notNull && !c.NotNull) {
			return false
		}
	}
	return true
}
