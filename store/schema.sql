-- Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
-- Use of this source code is governed by an MIT-style
-- license that can be found in the LICENSE file.

-- Schema mirror (applied via PRAGMA user_version migrations in migrate.go).
-- Current: v2 (v1 tables + root indexes).

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
CREATE INDEX idx_documents_root_name ON documents(root_name);
CREATE INDEX idx_documents_root_uri ON documents(root_name, uri);
CREATE INDEX idx_documents_root_source_type ON documents(root_name, source_type);
