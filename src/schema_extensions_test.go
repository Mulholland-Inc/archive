package main

import (
	"strings"
	"testing"
)

func TestSchemaRoundTripsConstraintsOnCoreTables(t *testing.T) {
	in := newInst(t)
	in.must("user", "add", "wren@example.com")
	// Public foreign keys must retain their required keys on core tables.
	ddl := `ALTER TABLE archive.file ADD CONSTRAINT original_identity UNIQUE (id, hash, size, space);
` + e2eTable("original", `    original_id uuid NOT NULL,
    digest text NOT NULL,
    length bigint NOT NULL,
    CONSTRAINT original_content FOREIGN KEY (original_id, digest, length, space)
        REFERENCES archive.file (id, hash, size, space) ON UPDATE RESTRICT ON DELETE RESTRICT,
    search tsvector GENERATED ALWAYS AS (archive.search_doc(digest)) STORED`)
	in.push(map[string]string{"original.sql": ddl})
	files := in.roundTrips()
	if !strings.Contains(files["archive.file.sql"], "original_identity") ||
		!strings.Contains(files["original.sql"], "original_content") {
		t.Fatalf("export lost a constraint: %v", files)
	}
	if strings.Contains(files["archive.file.sql"], "CREATE TABLE") {
		t.Fatal("export copied the core table instead of its extension")
	}
	if p := decode[SchemaPlan](t, in.push(files)); len(p.Statements) != 0 {
		t.Fatalf("reapplying exported extensions changed the schema: %+v", p)
	}
}
