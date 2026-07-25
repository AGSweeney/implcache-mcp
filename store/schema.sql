-- Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
-- Use of this source code is governed by an MIT-style
-- license that can be found in the LICENSE file.

-- Schema mirror for a fresh database after all migrations (PRAGMA user_version = 5).
-- Source of truth for stepwise upgrades remains migrate.go.

CREATE TABLE documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uri TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    root_name TEXT,
    mtime INTEGER NOT NULL DEFAULT 0,
    hash TEXT NOT NULL,
    authority TEXT NOT NULL DEFAULT 'unknown',
    technology TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    product_version TEXT NOT NULL DEFAULT '',
    deprecated INTEGER NOT NULL DEFAULT 0,
    archived INTEGER NOT NULL DEFAULT 0,
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
    root_name TEXT NOT NULL DEFAULT '',
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

CREATE TABLE symbols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    root_name TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    name_norm TEXT NOT NULL,
    qualified_name TEXT NOT NULL DEFAULT '',
    unqualified_name TEXT NOT NULL DEFAULT '',
    namespace TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    signature TEXT NOT NULL DEFAULT '',
    signature_norm TEXT NOT NULL DEFAULT '',
    start_line INTEGER NOT NULL DEFAULT 0,
    end_line INTEGER NOT NULL DEFAULT 0,
    UNIQUE(document_id, name_norm, start_line)
);

CREATE TABLE knowledge_entries (
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

CREATE TABLE knowledge_entry_sources (
    entry_id INTEGER NOT NULL REFERENCES knowledge_entries(id) ON DELETE CASCADE,
    source_uri TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(entry_id, source_uri)
);

CREATE TABLE aliases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    canonical TEXT NOT NULL,
    alias TEXT NOT NULL,
    technology TEXT NOT NULL DEFAULT '',
    root_name TEXT NOT NULL DEFAULT '',
    confidence REAL NOT NULL DEFAULT 1.0,
    UNIQUE(alias, technology, root_name)
);

CREATE TABLE root_groups (
    name TEXT PRIMARY KEY,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE root_group_members (
    group_name TEXT NOT NULL REFERENCES root_groups(name) ON DELETE CASCADE,
    root_name TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY(group_name, root_name)
);

CREATE INDEX idx_documents_source_type ON documents(source_type);
CREATE INDEX idx_chunks_document_id ON chunks(document_id);
CREATE INDEX idx_documents_root_name ON documents(root_name);
CREATE INDEX idx_documents_root_uri ON documents(root_name, uri);
CREATE INDEX idx_documents_root_source_type ON documents(root_name, source_type);
CREATE INDEX idx_symbols_name_norm ON symbols(name_norm);
CREATE INDEX idx_symbols_root_name ON symbols(root_name, name_norm);
CREATE INDEX idx_symbols_unqualified ON symbols(unqualified_name);
CREATE INDEX idx_symbols_qualified ON symbols(qualified_name);
CREATE INDEX idx_chunks_root_name ON chunks(root_name);
CREATE INDEX idx_knowledge_entries_subject ON knowledge_entries(subject);
CREATE INDEX idx_knowledge_entries_tech ON knowledge_entries(technology, language);
CREATE INDEX idx_aliases_alias ON aliases(alias);
CREATE INDEX idx_root_group_members_group ON root_group_members(group_name, priority DESC);
