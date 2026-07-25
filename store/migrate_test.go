// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSchemaMigratesToV2WithIndexes(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "m.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	v, err := st.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v != currentSchemaVersion {
		t.Fatalf("schema version=%d want %d", v, currentSchemaVersion)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, name := range []string{
		"idx_documents_root_name",
		"idx_documents_root_uri",
		"idx_documents_root_source_type",
		"idx_chunks_document_id",
	} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("missing index %s", name)
		}
	}
}

func TestMigrationFromV1ToCurrent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "old.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schemaV1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 1`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.SchemaVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != currentSchemaVersion {
		t.Fatalf("got v%d want %d", v, currentSchemaVersion)
	}
}

func TestMigrationV3SymbolBackfill(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "v3.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{schemaV1, schemaV2, schemaV3} {
		if _, err := db.Exec(step); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`PRAGMA user_version = 3`); err != nil {
		t.Fatal(err)
	}
	now := int64(1)
	res, err := db.Exec(`
		INSERT INTO documents(uri, title, source_type, path, root_name, hash, created_at, updated_at)
		VALUES ('project://example-plugin-sdk/api.h', 'api', 'source', 'api.h', 'example-plugin-sdk', 'h', ?, ?)`,
		now, now)
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := res.LastInsertId()
	_, err = db.Exec(`
		INSERT INTO symbols(document_id, root_name, name, name_norm, kind, language, signature, start_line, end_line)
		VALUES (?, 'example-plugin-sdk', 'demo::RegisterHandler', 'demo::registerhandler', 'declaration', 'cpp',
		        'int demo::RegisterHandler(const char* name);', 1, 1)`, docID)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.SchemaVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != currentSchemaVersion {
		t.Fatalf("got v%d", v)
	}

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var qual, unqual, ns, sigNorm string
	err = db.QueryRow(`
		SELECT qualified_name, unqualified_name, namespace, signature_norm
		FROM symbols WHERE name = 'demo::RegisterHandler'`).Scan(&qual, &unqual, &ns, &sigNorm)
	if err != nil {
		t.Fatal(err)
	}
	if qual != "demo::RegisterHandler" || unqual != "RegisterHandler" || ns != "demo" {
		t.Fatalf("backfill qual=%q unqual=%q ns=%q", qual, unqual, ns)
	}
	if sigNorm == "" {
		t.Fatal("signature_norm empty after backfill")
	}

	// Idempotent: reopen does not change forms.
	st.Close()
	st2, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	var unqual2 string
	if err := db.QueryRow(`SELECT unqualified_name FROM symbols WHERE name = 'demo::RegisterHandler'`).Scan(&unqual2); err != nil {
		t.Fatal(err)
	}
	if unqual2 != "RegisterHandler" {
		t.Fatalf("idempotent backfill broke: %q", unqual2)
	}
}
