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

func TestFreshDBHasExpectedSchemaObjects(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fresh.db")
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
		t.Fatalf("version=%d want %d", v, currentSchemaVersion)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tables := []string{
		"documents", "chunks", "chunks_fts", "symbols",
		"knowledge_entries", "knowledge_entry_sources", "aliases",
		"root_groups", "root_group_members", "chunk_term_vectors",
	}
	for _, name := range tables {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name=?`, name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("missing table/virtual %s", name)
		}
	}
	indexes := []string{
		"idx_documents_root_name",
		"idx_symbols_name_norm",
		"idx_symbols_unqualified",
		"idx_chunks_root_name",
	}
	for _, name := range indexes {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("missing index %s", name)
		}
	}
	// Symbol form columns from v4.
	rows, err := db.Query(`PRAGMA table_info(symbols)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		cols[name] = true
	}
	for _, c := range []string{"qualified_name", "unqualified_name", "namespace", "signature_norm"} {
		if !cols[c] {
			t.Fatalf("symbols missing column %s", c)
		}
	}
	var triggers int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='trigger'`).Scan(&triggers); err != nil {
		t.Fatal(err)
	}
	if triggers < 3 {
		t.Fatalf("expected FTS triggers, got %d", triggers)
	}
}
