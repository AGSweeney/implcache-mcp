-- Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
-- Use of this source code is governed by an MIT-style
-- license that can be found in the LICENSE file.

-- Schema mirror (applied via PRAGMA user_version migrations in migrate.go).
-- Current: v4 (symbols forms + chunks.root_name). Prefer migrate.go as source of truth.

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
