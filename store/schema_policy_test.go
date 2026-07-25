// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCurrentSchemaVersionEleven(t *testing.T) {
	if currentSchemaVersion != 11 {
		t.Fatalf("currentSchemaVersion=%d want 11", currentSchemaVersion)
	}
}

func TestFreshDatabaseCreatedAtCurrentVersion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fresh.db")
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
		t.Fatalf("fresh version=%d want %d", v, currentSchemaVersion)
	}
	var fk int
	if err := st.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatal("foreign keys must be enabled on the store connection")
	}
}

func TestReopenCurrentVersionIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "reopen.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := st.UpsertDocument(ctx, UpsertInput{
		URI: "project://sdk/a.md", Title: "A", SourceType: SourceMarkdown, Path: "a.md",
		RootName: "sdk", Authority: AuthorityOfficialDocs, Hash: "a",
		Chunks: []Chunk{{Body: "RetryPolicy reconnect backoff", StartLine: 1, EndLine: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	st.Close()

	st2, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	v, err := st2.SchemaVersion(ctx)
	if err != nil || v != currentSchemaVersion {
		t.Fatalf("reopen version=%d err=%v", v, err)
	}
	docs, err := st2.ListDocuments(ctx, "")
	if err != nil || len(docs) != 1 {
		t.Fatalf("reopen docs=%d err=%v", len(docs), err)
	}
}

func TestOpenRefusesMismatchedSchemaVersion(t *testing.T) {
	for _, version := range []int{1, 5, 6, 7, 8, 9, 10, currentSchemaVersion + 1} {
		dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("v%d.db", version))
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TABLE legacy (id INTEGER PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
			t.Fatal(err)
		}
		db.Close()

		_, err = Open(dbPath)
		if err == nil {
			t.Fatalf("version %d: expected refusal", version)
		}
		msg := err.Error()
		for _, want := range []string{
			dbPath,
			fmt.Sprintf("schema version %d", version),
			fmt.Sprintf("expected %d", currentSchemaVersion),
			"delete the database file",
		} {
			if !strings.Contains(msg, want) {
				t.Fatalf("version %d: error missing %q: %v", version, want, err)
			}
		}

		// The refused database must be untouched: same version, same objects.
		db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		var after, legacy int
		if err := db.QueryRow(`PRAGMA user_version`).Scan(&after); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'legacy'`).Scan(&legacy); err != nil {
			db.Close()
			t.Fatal(err)
		}
		var objects int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'`).Scan(&objects); err != nil {
			db.Close()
			t.Fatal(err)
		}
		db.Close()
		if after != version || legacy != 1 || objects != 1 {
			t.Fatalf("version %d: refused database was modified (version=%d legacy=%d objects=%d)",
				version, after, legacy, objects)
		}
	}
}

func TestOpenRefusesUnversionedNonEmptyDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "foreign.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE something_else (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if _, err := Open(dbPath); err == nil {
		t.Fatal("expected refusal for unversioned non-empty database")
	}
}

func TestOpenRefusesMalformedCurrentSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "broken-v7.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// Claim current schema version but omit required semantic objects.
	if _, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE documents (id INTEGER PRIMARY KEY);
		CREATE TABLE chunks (id INTEGER PRIMARY KEY);
		PRAGMA user_version = %d;
	`, currentSchemaVersion)); err != nil {
		t.Fatal(err)
	}
	db.Close()

	_, err = Open(dbPath)
	if err == nil {
		t.Fatal("expected refusal for malformed current-schema database")
	}
	msg := err.Error()
	for _, want := range []string{
		dbPath,
		fmt.Sprintf("schema version %d", currentSchemaVersion),
		"missing required object",
		"delete the database file",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}

	// Must not repair or migrate the damaged file.
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, postings int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'chunk_term_postings'`).Scan(&postings); err != nil {
		t.Fatal(err)
	}
	if version != currentSchemaVersion || postings != 0 {
		t.Fatalf("malformed database was modified: version=%d postings=%d", version, postings)
	}
}

func TestDeleteAndRecreateDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "recreate.db")
	ctx := context.Background()

	// Simulate an incompatible development database.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE legacy (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 6`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Open(dbPath); err == nil {
		t.Fatal("expected refusal before recreation")
	}

	// The instructed recovery: delete the file (and sidecars), reopen, re-ingest.
	for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.UpsertDocument(ctx, UpsertInput{
		URI: "project://sdk/retry.md", Title: "Retry", SourceType: SourceMarkdown, Path: "retry.md",
		RootName: "sdk", Authority: AuthorityOfficialDocs, Hash: "r1",
		Chunks: []Chunk{{Heading: "Retry", Body: "RetryPolicy reconnect exponential backoff", StartLine: 1, EndLine: 2}},
	}); err != nil {
		t.Fatal(err)
	}
	hits, err := st.SearchOpts(ctx, SearchOptions{Query: "reconnect backoff", Limit: 5, Roots: []string{"sdk"}, Semantic: true})
	if err != nil || len(hits) == 0 {
		t.Fatalf("recreated db search hits=%d err=%v", len(hits), err)
	}
}

func TestFreshIngestWritesVectorsAndPostings(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "vectors.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	heading, body := "Reconnect", "Reconnect with exponential backoff and RetryPolicy."
	if _, err := st.UpsertDocument(ctx, UpsertInput{
		URI: "project://sdk/retry.md", Title: "Retry", SourceType: SourceMarkdown, Path: "retry.md",
		RootName: "sdk", Authority: AuthorityOfficialDocs, Hash: "r1",
		Chunks: []Chunk{{Heading: heading, Body: body, StartLine: 1, EndLine: 3}},
	}); err != nil {
		t.Fatal(err)
	}

	var terms string
	if err := st.db.QueryRow(`SELECT terms FROM chunk_term_vectors`).Scan(&terms); err != nil {
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
