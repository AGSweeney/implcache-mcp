// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func schemaSignature(t *testing.T, path string) string {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT type, name, COALESCE(sql, '')
		FROM sqlite_master
		WHERE name NOT LIKE 'sqlite_%'
		ORDER BY type, name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var entries []string
	var tables []string
	for rows.Next() {
		var typ, name, sqlText string
		if err := rows.Scan(&typ, &name, &sqlText); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, typ+"/"+name+"/"+sqlText)
		if typ == "table" {
			tables = append(tables, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(tables)
	for _, table := range tables {
		info, err := db.Query(`PRAGMA table_info(` + table + `)`)
		if err != nil {
			t.Fatal(err)
		}
		for info.Next() {
			var cid, notNull, pk int
			var name, ctype string
			var dflt any
			if err := info.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
				info.Close()
				t.Fatal(err)
			}
			entries = append(entries, fmt.Sprintf("column/%s/%d/%s/%s/%d/%d", table, cid, name, ctype, notNull, pk))
		}
		info.Close()
		fks, err := db.Query(`PRAGMA foreign_key_list(` + table + `)`)
		if err != nil {
			t.Fatal(err)
		}
		for fks.Next() {
			var id, seq int
			var toTable, from, to, onUpdate, onDelete, match string
			if err := fks.Scan(&id, &seq, &toTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
				fks.Close()
				t.Fatal(err)
			}
			entries = append(entries, fmt.Sprintf("fk/%s/%d/%d/%s/%s/%s/%s", table, id, seq, toTable, from, to, onDelete))
		}
		fks.Close()
	}
	sort.Strings(entries)
	return strings.Join(entries, "\n")
}

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
	if currentSchemaVersion != 7 {
		t.Fatalf("currentSchemaVersion=%d want 7", currentSchemaVersion)
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

func TestCurrentSchemaVersionSeven(t *testing.T) {
	if currentSchemaVersion != 7 {
		t.Fatalf("currentSchemaVersion=%d want 7", currentSchemaVersion)
	}
}

func createV5Fixture(t *testing.T, dbPath string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, step := range []string{schemaV1, schemaV2, schemaV3, schemaV4} {
		if _, err := db.Exec(step); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := backfillSymbolFormsTx(tx); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 5`); err != nil {
		t.Fatal(err)
	}
	now := int64(1)
	doc, err := db.Exec(`
		INSERT INTO documents(uri, title, source_type, path, root_name, hash, created_at, updated_at)
		VALUES ('project://example-network-sdk/retry.md', 'Retry', 'markdown', 'retry.md',
		        'example-network-sdk', 'v5-hash', ?, ?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	docID, err := doc.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO chunks(document_id, ordinal, heading, body, start_line, end_line, root_name)
		VALUES (?, 0, 'Reconnect', 'Reconnect with exponential backoff and RetryPolicy.', 1, 3, 'example-network-sdk')`, docID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO symbols(document_id, root_name, name, name_norm, qualified_name, unqualified_name,
		                    namespace, kind, language, signature, signature_norm, start_line, end_line)
		VALUES (?, 'example-network-sdk', 'demo::Reconnect', 'demo::reconnect', 'demo::Reconnect',
		        'Reconnect', 'demo', 'function', 'cpp', 'void demo::Reconnect()', 'voiddemo::reconnect()', 1, 1)`, docID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO knowledge_entries(uri, subject, body_markdown, root_name, hash)
		VALUES ('recipe://example-network-sdk/retry', 'Retry', '# Retry', 'example-network-sdk', 'recipe-hash')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO root_groups(name, description) VALUES ('network', 'Network roots')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO root_group_members(group_name, root_name, priority) VALUES ('network', 'example-network-sdk', 10)`); err != nil {
		t.Fatal(err)
	}
}

func createV6Fixture(t *testing.T, dbPath string) {
	t.Helper()
	createV5Fixture(t, dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(schemaV6); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := backfillChunkTermVectorsTx(tx); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 6`); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationV6ToV7BuildsIndexedPostings(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "v6.db")
	createV6Fixture(t, dbPath)

	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if v, err := st.SchemaVersion(ctx); err != nil || v != 7 {
		t.Fatalf("version=%d err=%v", v, err)
	}
	var heading, body, terms string
	if err := st.db.QueryRow(`
		SELECT c.heading, c.body, v.terms
		FROM chunks c JOIN chunk_term_vectors v ON v.chunk_id = c.id`).Scan(&heading, &body, &terms); err != nil {
		t.Fatal(err)
	}
	if want := BuildTermVector(heading, body); terms != want {
		t.Fatalf("vector=%q want %q", terms, want)
	}
	postings, err := st.TermPostingCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := len(strings.Fields(terms)); postings != want {
		t.Fatalf("postings=%d want %d", postings, want)
	}
}

func TestMigrationV7RollbackDuringPostingBackfill(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "v6-fail.db")
	createV6Fixture(t, dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	now := int64(1)
	res, err := db.Exec(`
		INSERT INTO documents(uri, title, source_type, path, root_name, hash, created_at, updated_at)
		VALUES ('project://example-network-sdk/second.md', 'Second', 'markdown', 'second.md',
		        'example-network-sdk', 'second', ?, ?)`, now, now)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	docID, _ := res.LastInsertId()
	res, err = db.Exec(`
		INSERT INTO chunks(document_id, ordinal, heading, body, start_line, end_line, root_name)
		VALUES (?, 0, 'Second', 'second retry reconnect document', 1, 1, 'example-network-sdk')`, docID)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	chunkID, _ := res.LastInsertId()
	if _, err := db.Exec(`INSERT INTO chunk_term_vectors(chunk_id, terms, updated_at) VALUES (?, ?, 1)`,
		chunkID, BuildTermVector("Second", "second retry reconnect document")); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	seen := 0
	testTermPostingBackfillHook = func(int64) error {
		seen++
		if seen == 2 {
			return fmt.Errorf("injected v7 backfill failure")
		}
		return nil
	}
	t.Cleanup(func() { testTermPostingBackfillHook = nil })
	if _, err := Open(dbPath); err == nil {
		t.Fatal("expected v7 migration failure")
	}
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var version, table int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='chunk_term_postings'`).Scan(&table); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	if version != 6 || table != 0 {
		t.Fatalf("rollback version=%d postingsTable=%d", version, table)
	}
	testTermPostingBackfillHook = nil
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if n, err := st.TermPostingCount(context.Background()); err != nil || n == 0 {
		t.Fatalf("retry postings=%d err=%v", n, err)
	}
}

func TestMigrationV5ToV7PreservesDataAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "v5.db")
	createV5Fixture(t, dbPath)

	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	v, err := st.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v != 7 {
		t.Fatalf("version=%d want 7", v)
	}
	vectors, err := st.TermVectorCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if vectors != 1 {
		t.Fatalf("v5 backfill vectors=%d want 1", vectors)
	}
	st.Close()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for table, want := range map[string]int{
		"documents": 1, "chunks": 1, "symbols": 1, "knowledge_entries": 1,
		"root_groups": 1, "root_group_members": 1, "chunk_term_vectors": 1,
		"chunk_term_postings": 4,
	} {
		var got int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&got); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if got != want {
			db.Close()
			t.Fatalf("%s=%d want %d", table, got, want)
		}
	}
	db.Close()

	// A second startup must not rerun v6 or duplicate its backfilled rows.
	st, err = Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	vectors, err = st.TermVectorCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if vectors != 1 {
		t.Fatalf("vectors after idempotent startup=%d want 1", vectors)
	}
}

func TestMigrationV6RollbackLeavesV5Retryable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "v5-fail.db")
	createV5Fixture(t, dbPath)
	testMigrationHook = func(version int, _ *sql.Tx) error {
		if version == 6 {
			return fmt.Errorf("injected v6 failure")
		}
		return nil
	}
	t.Cleanup(func() { testMigrationHook = nil })

	if _, err := Open(dbPath); err == nil {
		t.Fatal("expected v6 migration failure")
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var version, docs, chunks, symbols, recipes, groups int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&docs); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&chunks); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM symbols`).Scan(&symbols); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM knowledge_entries`).Scan(&recipes); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM root_groups`).Scan(&groups); err != nil {
		db.Close()
		t.Fatal(err)
	}
	var v6Table int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='chunk_term_vectors'`).Scan(&v6Table); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	if version != 5 || docs != 1 || chunks != 1 || symbols != 1 || recipes != 1 || groups != 1 || v6Table != 0 {
		t.Fatalf("rollback version=%d docs=%d chunks=%d symbols=%d recipes=%d groups=%d v6table=%d",
			version, docs, chunks, symbols, recipes, groups, v6Table)
	}

	testMigrationHook = nil
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	v, err := st.SchemaVersion(context.Background())
	if err != nil || v != 7 {
		t.Fatalf("retry version=%d err=%v", v, err)
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

	createV5Fixture(t, migratedPath)
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
	if fresh, migrated := schemaSignature(t, freshPath), schemaSignature(t, migratedPath); fresh != migrated {
		t.Fatalf("fresh and v5→v6 schema differ\nfresh:\n%s\nmigrated:\n%s", fresh, migrated)
	}
}
