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
		"root_groups", "root_group_members", "chunk_term_vectors", "chunk_term_postings",
		"term_df", "root_chunk_stats",
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
		"idx_documents_source_type",
		"idx_documents_root_name",
		"idx_documents_root_uri",
		"idx_documents_root_source_type",
		"idx_chunks_document_id",
		"idx_symbols_name_norm",
		"idx_symbols_root_name",
		"idx_symbols_unqualified",
		"idx_symbols_qualified",
		"idx_chunks_root_name",
		"idx_chunk_term_postings_root_term",
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
	vectorRows, err := db.Query(`PRAGMA table_info(chunk_term_vectors)`)
	if err != nil {
		t.Fatal(err)
	}
	vectorCols := map[string]bool{}
	vectorPK := map[string]bool{}
	for vectorRows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt any
		if err := vectorRows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			vectorRows.Close()
			t.Fatal(err)
		}
		vectorCols[name] = true
		if pk > 0 {
			vectorPK[name] = true
		}
	}
	vectorRows.Close()
	for _, c := range []string{"chunk_id", "terms", "updated_at"} {
		if !vectorCols[c] {
			t.Fatalf("chunk_term_vectors missing column %s", c)
		}
	}
	if !vectorPK["chunk_id"] {
		t.Fatal("chunk_term_vectors.chunk_id must be the primary key")
	}
	indexRows, err := db.Query(`PRAGMA index_list(chunk_term_vectors)`)
	if err != nil {
		t.Fatal(err)
	}
	defer indexRows.Close()
	for indexRows.Next() {
		var seq int
		var name string
		var unique, partial int
		var origin string
		if err := indexRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatal(err)
		}
		if name == "idx_chunk_term_vectors_terms" {
			t.Fatal("leading-wildcard semantic lookup must not create an unused terms B-tree")
		}
	}
	var triggers int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='trigger'`).Scan(&triggers); err != nil {
		t.Fatal(err)
	}
	if triggers < 3 {
		t.Fatalf("expected FTS triggers, got %d", triggers)
	}

	// Foreign keys declared on child tables (PRAGMA foreign_keys may be off; list still reports schema).
	expectFK := map[string]string{
		"chunks":                  "documents",
		"symbols":                 "documents",
		"knowledge_entry_sources": "knowledge_entries",
		"root_group_members":      "root_groups",
		"chunk_term_vectors":      "chunks",
		"chunk_term_postings":     "chunks",
	}
	for child, parent := range expectFK {
		rows, err := db.Query(`PRAGMA foreign_key_list(` + child + `)`)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for rows.Next() {
			var id, seq int
			var table, from, to, onUpdate, onDelete, match string
			if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			if table == parent {
				found = true
			}
		}
		rows.Close()
		if !found {
			t.Fatalf("%s missing FK to %s", child, parent)
		}
	}
}
