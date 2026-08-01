-- Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
-- Use of this source code is governed by an MIT-style
-- license that can be found in the LICENSE file.

-- Canonical schema (PRAGMA user_version = 12). New databases are created
-- directly from this file (embedded via store/schema.go). Version 11→12 is
-- an additive migrator for knowledge-group columns only; other mismatched
-- versions must be deleted and re-ingested.

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
    start_page INTEGER NOT NULL DEFAULT 0,
    end_page INTEGER NOT NULL DEFAULT 0,
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
    description TEXT NOT NULL DEFAULT '',
    id TEXT NOT NULL DEFAULT '',
    policies_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE root_group_members (
    group_name TEXT NOT NULL REFERENCES root_groups(name) ON DELETE CASCADE,
    root_name TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    role TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(group_name, root_name)
);

CREATE UNIQUE INDEX idx_root_groups_id ON root_groups(id) WHERE id != '';

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

CREATE TABLE chunk_term_vectors (
    chunk_id INTEGER PRIMARY KEY REFERENCES chunks(id) ON DELETE CASCADE,
    terms TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE chunk_term_postings (
    chunk_id INTEGER NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    root_name TEXT NOT NULL DEFAULT '',
    term TEXT NOT NULL,
    PRIMARY KEY(chunk_id, term)
);
CREATE INDEX idx_chunk_term_postings_root_term
    ON chunk_term_postings(root_name, term, chunk_id);

-- Persisted document frequency for query-time IDF (maintained on ingest/delete).
CREATE TABLE term_df (
    root_name TEXT NOT NULL,
    term TEXT NOT NULL,
    df INTEGER NOT NULL,
    PRIMARY KEY (root_name, term)
);

CREATE TABLE root_chunk_stats (
    root_name TEXT NOT NULL PRIMARY KEY,
    chunk_count INTEGER NOT NULL
);

CREATE TABLE web_sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    root_name TEXT NOT NULL,
    start_url TEXT NOT NULL,
    profile TEXT NOT NULL DEFAULT 'generic',
    allowed_prefixes TEXT NOT NULL DEFAULT '[]',
    authority TEXT NOT NULL DEFAULT 'official_documentation',
    product TEXT NOT NULL DEFAULT '',
    declared_version TEXT NOT NULL DEFAULT '',
    detected_version TEXT NOT NULL DEFAULT '',
    target TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    configuration_json TEXT NOT NULL DEFAULT '{}',
    last_attempt_at INTEGER NOT NULL DEFAULT 0,
    last_success_at INTEGER NOT NULL DEFAULT 0,
    last_status TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE web_pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    web_source_id INTEGER NOT NULL REFERENCES web_sources(id) ON DELETE CASCADE,
    document_id INTEGER REFERENCES documents(id) ON DELETE SET NULL,
    source_url TEXT NOT NULL,
    canonical_url TEXT NOT NULL DEFAULT '',
    relative_path TEXT NOT NULL DEFAULT '',
    page_title TEXT NOT NULL DEFAULT '',
    etag TEXT NOT NULL DEFAULT '',
    last_modified TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    http_status INTEGER NOT NULL DEFAULT 0,
    content_type TEXT NOT NULL DEFAULT '',
    content_length INTEGER NOT NULL DEFAULT 0,
    fetched_at INTEGER NOT NULL DEFAULT 0,
    verified_at INTEGER NOT NULL DEFAULT 0,
    crawl_generation INTEGER NOT NULL DEFAULT 0,
    crawl_depth INTEGER NOT NULL DEFAULT 0,
    last_seen_generation INTEGER NOT NULL DEFAULT 0,
    missing_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    UNIQUE(web_source_id, source_url)
);
CREATE INDEX idx_web_pages_source ON web_pages(web_source_id, last_seen_generation);
CREATE INDEX idx_web_sources_root ON web_sources(root_name);

CREATE TABLE pdf_sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id INTEGER REFERENCES documents(id) ON DELETE SET NULL,
    root_name TEXT NOT NULL DEFAULT '',
    document_uri TEXT NOT NULL UNIQUE,
    source_path TEXT NOT NULL DEFAULT '',
    file_name TEXT NOT NULL DEFAULT '',
    file_hash TEXT NOT NULL DEFAULT '',
    file_size INTEGER NOT NULL DEFAULT 0,
    page_count INTEGER NOT NULL DEFAULT 0,
    title TEXT NOT NULL DEFAULT '',
    product TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    authority TEXT NOT NULL DEFAULT 'official_documentation',
    language TEXT NOT NULL DEFAULT '',
    pdf_version TEXT NOT NULL DEFAULT '',
    encrypted INTEGER NOT NULL DEFAULT 0,
    ocr_mode TEXT NOT NULL DEFAULT 'off',
    extraction_status TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE pdf_pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pdf_source_id INTEGER NOT NULL REFERENCES pdf_sources(id) ON DELETE CASCADE,
    page_number INTEGER NOT NULL,
    page_label TEXT NOT NULL DEFAULT '',
    text_hash TEXT NOT NULL DEFAULT '',
    text_length INTEGER NOT NULL DEFAULT 0,
    page_type TEXT NOT NULL DEFAULT 'text',
    ocr_used INTEGER NOT NULL DEFAULT 0,
    layout_type TEXT NOT NULL DEFAULT 'single',
    extraction_confidence TEXT NOT NULL DEFAULT 'unknown',
    warning_flags TEXT NOT NULL DEFAULT '',
    UNIQUE(pdf_source_id, page_number)
);
CREATE INDEX idx_pdf_sources_root ON pdf_sources(root_name);

CREATE TABLE repo_sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    root_name TEXT NOT NULL,
    remote_url TEXT NOT NULL DEFAULT '',
    local_path TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    owner TEXT NOT NULL DEFAULT '',
    repository TEXT NOT NULL DEFAULT '',
    acquisition_mode TEXT NOT NULL DEFAULT 'snapshot',
    requested_ref TEXT NOT NULL DEFAULT '',
    resolved_commit_sha TEXT NOT NULL DEFAULT '',
    default_branch TEXT NOT NULL DEFAULT '',
    authority TEXT NOT NULL DEFAULT 'current_project',
    product TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    credential_reference TEXT NOT NULL DEFAULT '',
    include_patterns TEXT NOT NULL DEFAULT '[]',
    exclude_patterns TEXT NOT NULL DEFAULT '[]',
    sparse_paths TEXT NOT NULL DEFAULT '[]',
    submodule_policy TEXT NOT NULL DEFAULT 'ignore',
    symlink_policy TEXT NOT NULL DEFAULT 'ignore',
    working_tree_mode TEXT NOT NULL DEFAULT 'HEAD',
    clone_depth INTEGER NOT NULL DEFAULT 1,
    partial_clone_filter TEXT NOT NULL DEFAULT '',
    checkout_path TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    last_attempt_at INTEGER NOT NULL DEFAULT 0,
    last_success_at INTEGER NOT NULL DEFAULT 0,
    last_status TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE repo_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_source_id INTEGER NOT NULL REFERENCES repo_sources(id) ON DELETE CASCADE,
    document_id INTEGER REFERENCES documents(id) ON DELETE SET NULL,
    relative_path TEXT NOT NULL,
    blob_hash TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL DEFAULT '',
    content_class TEXT NOT NULL DEFAULT 'unknown',
    file_size INTEGER NOT NULL DEFAULT 0,
    resolved_commit_sha TEXT NOT NULL DEFAULT '',
    last_seen_generation INTEGER NOT NULL DEFAULT 0,
    UNIQUE(repo_source_id, relative_path)
);
CREATE INDEX idx_repo_sources_root ON repo_sources(root_name);
CREATE INDEX idx_repo_files_source ON repo_files(repo_source_id, last_seen_generation);
