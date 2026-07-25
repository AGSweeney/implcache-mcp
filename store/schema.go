// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"database/sql"
	_ "embed"
	"fmt"
)

// currentSchemaVersion identifies the canonical schema (PRAGMA user_version).
// It is a schema identity check, not a migration ladder: pre-release databases
// with a different version must be deleted and recreated. Real migrations
// begin only after a deployment contains data that must be preserved.
const currentSchemaVersion = 11

// canonicalSchema is the complete, authoritative schema for new databases.
//
//go:embed schema.sql
var canonicalSchema string

// requiredSchemaObjects are the minimum sqlite_master names a claimed current
// schema must expose. This catches damaged or partially created databases that
// already have the right user_version without comparing the entire DDL.
var requiredSchemaObjects = []string{
	"documents",
	"chunks",
	"chunks_fts",
	"symbols",
	"chunk_term_vectors",
	"chunk_term_postings",
	"idx_chunk_term_postings_root_term",
	"term_df",
	"root_chunk_stats",
	"web_sources",
	"web_pages",
	"pdf_sources",
	"pdf_pages",
	"repo_sources",
	"repo_files",
}

// ensureSchema opens-or-creates the canonical schema:
//   - user_version == currentSchemaVersion: validate required objects, then open.
//   - empty database (user_version 0, no objects): create the schema.
//   - anything else: refuse without modification, with instructions to
//     delete and rebuild.
func ensureSchema(db *sql.DB, path string) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if version == currentSchemaVersion {
		return validateCanonicalSchema(db, path)
	}
	if version != 0 {
		return schemaMismatchError(path, version)
	}

	var objects int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name NOT LIKE 'sqlite_%'`).Scan(&objects); err != nil {
		return fmt.Errorf("inspect database: %w", err)
	}
	if objects != 0 {
		// Unversioned but non-empty: not a database this build created.
		return schemaMismatchError(path, version)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(canonicalSchema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, currentSchemaVersion)); err != nil {
		return fmt.Errorf("set user_version=%d: %w", currentSchemaVersion, err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return validateCanonicalSchema(db, path)
}

func validateCanonicalSchema(db *sql.DB, path string) error {
	for _, name := range requiredSchemaObjects {
		var found string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE name = ? LIMIT 1`, name,
		).Scan(&found)
		if err == sql.ErrNoRows || found == "" {
			return schemaStructureError(path, name)
		}
		if err != nil {
			return fmt.Errorf("inspect schema object %s: %w", name, err)
		}
	}
	return nil
}

func schemaMismatchError(path string, found int) error {
	return fmt.Errorf(
		"database %s: schema version %d is incompatible with this build (expected %d); "+
			"pre-release databases are not migrated — delete the database file "+
			"(and its -wal/-shm sidecars) and re-ingest to rebuild it",
		path, found, currentSchemaVersion,
	)
}

func schemaStructureError(path, missing string) error {
	return fmt.Errorf(
		"database %s: schema version %d is marked current but missing required object %q; "+
			"pre-release databases are not repaired — delete the database file "+
			"(and its -wal/-shm sidecars) and re-ingest to rebuild it",
		path, currentSchemaVersion, missing,
	)
}
