// Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package store

import (
	"database/sql"
	"fmt"
)

const currentSchemaVersion = 4

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

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	for version < currentSchemaVersion {
		next := version + 1
		if err := applyMigration(db, next); err != nil {
			return fmt.Errorf("migrate to v%d: %w", next, err)
		}
		if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, next)); err != nil {
			return fmt.Errorf("set user_version=%d: %w", next, err)
		}
		version = next
	}
	return nil
}

func applyMigration(db *sql.DB, version int) error {
	switch version {
	case 1:
		_, err := db.Exec(schemaV1)
		return err
	case 2:
		_, err := db.Exec(schemaV2)
		return err
	case 3:
		_, err := db.Exec(schemaV3)
		return err
	case 4:
		_, err := db.Exec(schemaV4)
		return err
	default:
		return fmt.Errorf("unknown schema version %d", version)
	}
}
