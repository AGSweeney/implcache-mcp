// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"database/sql"
	"fmt"
)

const currentSchemaVersion = 7

const schemaV1 = `
CREATE TABLE documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uri TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    root_name TEXT,
    mtime INTEGER NOT NULL DEFAULT 0,
    hash TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL,
    heading TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL,
    start_line INTEGER NOT NULL DEFAULT 0,
    end_line INTEGER NOT NULL DEFAULT 0,
    UNIQUE(document_id, ordinal)
);

CREATE VIRTUAL TABLE chunks_fts USING fts5(
    heading,
    body,
    content='chunks',
    content_rowid='id'
);

CREATE TRIGGER chunks_ai AFTER INSERT ON chunks BEGIN
    INSERT INTO chunks_fts(rowid, heading, body) VALUES (new.id, new.heading, new.body);
END;

CREATE TRIGGER chunks_ad AFTER DELETE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, heading, body) VALUES ('delete', old.id, old.heading, old.body);
END;

CREATE TRIGGER chunks_au AFTER UPDATE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, heading, body) VALUES ('delete', old.id, old.heading, old.body);
    INSERT INTO chunks_fts(rowid, heading, body) VALUES (new.id, new.heading, new.body);
END;

CREATE INDEX idx_documents_source_type ON documents(source_type);
CREATE INDEX idx_chunks_document_id ON chunks(document_id);
`

// schemaV2 adds indexes that make root-scoped listing/search filters cheap.
const schemaV2 = `
CREATE INDEX IF NOT EXISTS idx_documents_root_name ON documents(root_name);
CREATE INDEX IF NOT EXISTS idx_documents_root_uri ON documents(root_name, uri);
CREATE INDEX IF NOT EXISTS idx_documents_root_source_type ON documents(root_name, source_type);
`

// schemaV3 adds implementation-context metadata: authority, symbols, recipes, aliases, root groups.
const schemaV3 = `
ALTER TABLE documents ADD COLUMN authority TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE documents ADD COLUMN technology TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN language TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN product_version TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN deprecated INTEGER NOT NULL DEFAULT 0;
ALTER TABLE documents ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS symbols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    root_name TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    name_norm TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    signature TEXT NOT NULL DEFAULT '',
    start_line INTEGER NOT NULL DEFAULT 0,
    end_line INTEGER NOT NULL DEFAULT 0,
    UNIQUE(document_id, name_norm, start_line)
);
CREATE INDEX IF NOT EXISTS idx_symbols_name_norm ON symbols(name_norm);
CREATE INDEX IF NOT EXISTS idx_symbols_root_name ON symbols(root_name, name_norm);

CREATE TABLE IF NOT EXISTS knowledge_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uri TEXT NOT NULL UNIQUE,
    subject TEXT NOT NULL DEFAULT '',
    technology TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    body_markdown TEXT NOT NULL,
    review_status TEXT NOT NULL DEFAULT 'generated',
    authority TEXT NOT NULL DEFAULT 'generated_summary',
    confidence TEXT NOT NULL DEFAULT 'medium',
    root_name TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT 0,
    verified_at INTEGER NOT NULL DEFAULT 0,
    hash TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_knowledge_entries_subject ON knowledge_entries(subject);
CREATE INDEX IF NOT EXISTS idx_knowledge_entries_tech ON knowledge_entries(technology, language);

CREATE TABLE IF NOT EXISTS knowledge_entry_sources (
    entry_id INTEGER NOT NULL REFERENCES knowledge_entries(id) ON DELETE CASCADE,
    source_uri TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(entry_id, source_uri)
);

CREATE TABLE IF NOT EXISTS aliases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    canonical TEXT NOT NULL,
    alias TEXT NOT NULL,
    technology TEXT NOT NULL DEFAULT '',
    root_name TEXT NOT NULL DEFAULT '',
    confidence REAL NOT NULL DEFAULT 1.0,
    UNIQUE(alias, technology, root_name)
);
CREATE INDEX IF NOT EXISTS idx_aliases_alias ON aliases(alias);

CREATE TABLE IF NOT EXISTS root_groups (
    name TEXT PRIMARY KEY,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS root_group_members (
    group_name TEXT NOT NULL REFERENCES root_groups(name) ON DELETE CASCADE,
    root_name TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY(group_name, root_name)
);
CREATE INDEX IF NOT EXISTS idx_root_group_members_group ON root_group_members(group_name, priority DESC);
`

// schemaV4 adds richer symbol forms and denormalized root_name on chunks for query plans.
const schemaV4 = `
ALTER TABLE symbols ADD COLUMN qualified_name TEXT NOT NULL DEFAULT '';
ALTER TABLE symbols ADD COLUMN unqualified_name TEXT NOT NULL DEFAULT '';
ALTER TABLE symbols ADD COLUMN namespace TEXT NOT NULL DEFAULT '';
ALTER TABLE symbols ADD COLUMN signature_norm TEXT NOT NULL DEFAULT '';

UPDATE symbols SET
  qualified_name = name,
  unqualified_name = name,
  namespace = '',
  signature_norm = LOWER(REPLACE(signature, ' ', ''))
WHERE unqualified_name = '' OR unqualified_name IS NULL;

CREATE INDEX IF NOT EXISTS idx_symbols_unqualified ON symbols(unqualified_name);
CREATE INDEX IF NOT EXISTS idx_symbols_qualified ON symbols(qualified_name);

ALTER TABLE chunks ADD COLUMN root_name TEXT NOT NULL DEFAULT '';
UPDATE chunks SET root_name = COALESCE((
  SELECT d.root_name FROM documents d WHERE d.id = chunks.document_id
), '') WHERE root_name = '';
CREATE INDEX IF NOT EXISTS idx_chunks_root_name ON chunks(root_name);
`

// schemaV5 re-derives symbol lookup forms for rows naively backfilled in v4.
// Applied in Go via backfillSymbolForms (SQL alone cannot split :: / . namespaces).

// schemaV6 adds sparse term vectors for optional semantic (related-term) search.
const schemaV6 = `
CREATE TABLE IF NOT EXISTS chunk_term_vectors (
    chunk_id INTEGER PRIMARY KEY REFERENCES chunks(id) ON DELETE CASCADE,
    terms TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT 0
);
`

// schemaV7 replaces the leading-wildcard vector scan with an inverted index.
// root_name is denormalized from chunks so root-scoped semantic candidates use
// one indexed lookup without joining documents to discover their root.
const schemaV7 = `
CREATE TABLE IF NOT EXISTS chunk_term_postings (
    chunk_id INTEGER NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    root_name TEXT NOT NULL DEFAULT '',
    term TEXT NOT NULL,
    PRIMARY KEY(chunk_id, term)
);
CREATE INDEX IF NOT EXISTS idx_chunk_term_postings_root_term
    ON chunk_term_postings(root_name, term, chunk_id);
`

// testMigrationHook, when set, runs inside migrateOne after DDL/backfill and
// before PRAGMA user_version is set. Tests use it to force rollback.
var testMigrationHook func(version int, tx *sql.Tx) error

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	for version < currentSchemaVersion {
		next := version + 1
		if err := migrateOne(db, next); err != nil {
			return fmt.Errorf("migrate to v%d: %w", next, err)
		}
		version = next
	}
	return nil
}

// migrateOne applies a single schema step and user_version bump in one transaction.
// On failure the transaction rolls back and the previous user_version remains.
// SQLite applies PRAGMA user_version transactionally on the connection/tx.
func migrateOne(db *sql.DB, version int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := applyMigrationTx(tx, version); err != nil {
		return err
	}
	if testMigrationHook != nil {
		if err := testMigrationHook(version, tx); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
		return fmt.Errorf("set user_version=%d: %w", version, err)
	}
	return tx.Commit()
}

func applyMigrationTx(tx *sql.Tx, version int) error {
	switch version {
	case 1:
		_, err := tx.Exec(schemaV1)
		return err
	case 2:
		_, err := tx.Exec(schemaV2)
		return err
	case 3:
		_, err := tx.Exec(schemaV3)
		return err
	case 4:
		_, err := tx.Exec(schemaV4)
		return err
	case 5:
		return backfillSymbolFormsTx(tx)
	case 6:
		if _, err := tx.Exec(schemaV6); err != nil {
			return err
		}
		return backfillChunkTermVectorsTx(tx)
	case 7:
		if _, err := tx.Exec(schemaV7); err != nil {
			return err
		}
		return backfillChunkTermPostingsTx(tx)
	default:
		return fmt.Errorf("unknown schema version %d", version)
	}
}

// backfillSymbolFormsTx populates derived symbol columns using DeriveSymbolForms.
// Idempotent: re-running yields the same values for a given name/signature.
func backfillSymbolFormsTx(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id, name, COALESCE(signature, '') FROM symbols`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id   int64
		name string
		sig  string
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.name, &r.sig); err != nil {
			return err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(list) == 0 {
		return nil
	}

	stmt, err := tx.Prepare(`
		UPDATE symbols SET
			qualified_name = ?,
			unqualified_name = ?,
			namespace = ?,
			signature_norm = ?,
			name_norm = ?
		WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range list {
		forms := DeriveSymbolForms(r.name, r.sig)
		if _, err := stmt.Exec(
			forms.QualifiedName, forms.UnqualifiedName, forms.Namespace,
			forms.SignatureNorm, forms.NameNorm, r.id,
		); err != nil {
			return err
		}
	}
	return nil
}
