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
	if v != 3 {
		t.Fatalf("schema version=%d want 3", v)
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

func TestMigrationFromV1ToV2(t *testing.T) {
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
	if v != 3 {
		t.Fatalf("got v%d", v)
	}
}
