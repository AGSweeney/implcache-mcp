// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"fmt"
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

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM symbols`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count changed: %d", count)
	}
	syms, err := st2.FindSymbols(context.Background(), "RegisterHandler", []string{"example-plugin-sdk"}, 5)
	if err != nil || len(syms) == 0 {
		t.Fatalf("lookup after migration: %v %+v", err, syms)
	}
}

func TestMigrationRollbackKeepsUserVersion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fail.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schemaV1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schemaV2); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 2`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	testMigrationHook = func(version int, tx *sql.Tx) error {
		if version == 3 {
			return fmt.Errorf("injected failure at v3")
		}
		return nil
	}
	t.Cleanup(func() { testMigrationHook = nil })

	_, err = Open(dbPath)
	if err == nil {
		t.Fatal("expected migration failure")
	}

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Fatalf("user_version=%d want 2 after failed v3", v)
	}
	// v3 tables must not exist after rollback.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='symbols'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("symbols table should not exist after rolled-back v3, count=%d", n)
	}
}

func TestMigrationIdempotentOnCurrentSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cur.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	v1, err := st.SchemaVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	st2, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	v2, err := st2.SchemaVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v1 != currentSchemaVersion || v2 != currentSchemaVersion {
		t.Fatalf("versions %d %d want %d", v1, v2, currentSchemaVersion)
	}
}

func TestFreshAndMigratedSchemaObjectsMatch(t *testing.T) {
	dir := t.TempDir()
	freshPath := filepath.Join(dir, "fresh.db")
	migratedPath := filepath.Join(dir, "migrated.db")

	stFresh, err := Open(freshPath)
	if err != nil {
		t.Fatal(err)
	}
	stFresh.Close()

	db, err := sql.Open("sqlite", migratedPath)
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
	stMig, err := Open(migratedPath)
	if err != nil {
		t.Fatal(err)
	}
	stMig.Close()

	objs := func(path string) map[string]string {
		d, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		rows, err := d.Query(`SELECT type, name FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string]string{}
		for rows.Next() {
			var typ, name string
			if err := rows.Scan(&typ, &name); err != nil {
				t.Fatal(err)
			}
			out[typ+"/"+name] = typ
		}
		var v int
		if err := d.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
			t.Fatal(err)
		}
		if v != currentSchemaVersion {
			t.Fatalf("%s user_version=%d", path, v)
		}
		return out
	}
	a, b := objs(freshPath), objs(migratedPath)
	for k := range a {
		if _, ok := b[k]; !ok {
			t.Fatalf("migrated missing %s", k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			t.Fatalf("fresh missing %s", k)
		}
	}
}
