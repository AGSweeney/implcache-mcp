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
// New databases are created at this version. A narrow additive migrator exists
// for 11→12 (knowledge-group columns). Other mismatched versions are refused.
const currentSchemaVersion = 12

// CurrentSchemaVersion returns the canonical knowledge DB schema identity version.
func CurrentSchemaVersion() int { return currentSchemaVersion }

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
	"root_groups",
	"root_group_members",
}

// ensureSchema opens-or-creates the canonical schema:
//   - user_version == currentSchemaVersion: validate required objects, then open.
//   - user_version == 11: additive migrate knowledge-group columns → 12.
//   - empty database (user_version 0, no objects): create the schema.
//   - anything else: refuse without modification.
func ensureSchema(db *sql.DB, path string) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if version == currentSchemaVersion {
		return validateCanonicalSchema(db, path)
	}
	if version == 11 {
		if err := migrateSchema11To12(db); err != nil {
			return fmt.Errorf("migrate schema 11→12: %w", err)
		}
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

// migrateSchema11To12 adds knowledge-group columns without touching corpus tables.
func migrateSchema11To12(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	alterIfMissing := func(table, column, ddl string) error {
		ok, err := tableHasColumnTx(tx, table, column)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		_, err = tx.Exec(ddl)
		return err
	}
	if err := alterIfMissing("root_groups", "id", `ALTER TABLE root_groups ADD COLUMN id TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := alterIfMissing("root_groups", "policies_json", `ALTER TABLE root_groups ADD COLUMN policies_json TEXT NOT NULL DEFAULT '{}'`); err != nil {
		return err
	}
	if err := alterIfMissing("root_group_members", "role", `ALTER TABLE root_group_members ADD COLUMN role TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	// Backfill id from name when empty.
	if _, err := tx.Exec(`UPDATE root_groups SET id = lower(replace(name, ' ', '-')) WHERE id = '' OR id IS NULL`); err != nil {
		return err
	}
	if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_root_groups_id ON root_groups(id) WHERE id != ''`); err != nil {
		return err
	}
	if _, err := tx.Exec(`PRAGMA user_version = 12`); err != nil {
		return err
	}
	return tx.Commit()
}

func tableHasColumnTx(tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
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
