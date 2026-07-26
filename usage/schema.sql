-- Copyright 2026 Adam G. Sweeney <agsweeney@gmail.com>. All rights reserved.
-- Use of this source code is governed by an MIT-style
-- license that can be found in the LICENSE file.

-- Usage analytics schema (PRAGMA user_version = 2).
-- Separate from the knowledge database; no FKs into implcache.db.

CREATE TABLE usage_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE request_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL UNIQUE,
    occurred_at TEXT NOT NULL,

    session_hash TEXT,
    client_name TEXT,
    model_name TEXT,
    tool_name TEXT NOT NULL,

    task_hash TEXT,
    task_summary TEXT,

    result_status TEXT NOT NULL,
    coverage TEXT,
    freshness TEXT,

    latency_ms INTEGER,
    estimated_tokens INTEGER,
    returned_tokens INTEGER,
    structured_tokens INTEGER,
    raw_document_tokens INTEGER,
    estimated_source_tokens INTEGER,
    estimated_tokens_avoided INTEGER,
    context_reduction_percent REAL,
    token_estimator_version TEXT,

    coverage_applicable INTEGER,
    request_class TEXT,

    context_fingerprint TEXT,

    root_selection_required INTEGER NOT NULL DEFAULT 0,
    additional_retrieval_recommended INTEGER NOT NULL DEFAULT 0,

    root_count INTEGER NOT NULL DEFAULT 0,
    source_count INTEGER NOT NULL DEFAULT 0,
    citation_count INTEGER NOT NULL DEFAULT 0,
    curated_count INTEGER NOT NULL DEFAULT 0,
    recipe_count INTEGER NOT NULL DEFAULT 0,
    symbol_count INTEGER NOT NULL DEFAULT 0,

    error_category TEXT,
    error_message TEXT
);

CREATE TABLE request_roots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL,
    root_key TEXT,
    root_name TEXT,
    root_group_key TEXT,
    root_role TEXT,
    selected INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE evidence_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL,

    evidence_type TEXT NOT NULL,
    evidence_key TEXT,
    root_key TEXT,
    source_uri TEXT,
    authority TEXT,

    rank_position INTEGER,
    selected_for_package INTEGER NOT NULL DEFAULT 0,
    included_after_trimming INTEGER NOT NULL DEFAULT 0,
    estimated_tokens INTEGER,

    source_hash TEXT
);

CREATE TABLE retrieval_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id TEXT NOT NULL,
    occurred_at TEXT NOT NULL,

    retrieval_type TEXT NOT NULL,
    query_hash TEXT,
    root_key TEXT,

    candidate_count INTEGER,
    selected_count INTEGER,
    latency_ms INTEGER
);

CREATE TABLE outcome_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,

    request_id TEXT,
    context_fingerprint TEXT,

    reporter_type TEXT,
    outcome TEXT,

    used_implcache_evidence INTEGER,
    additional_sources_used INTEGER,
    first_package_sufficient INTEGER,

    compile_status TEXT,
    test_status TEXT,
    helpfulness INTEGER,

    missing_information TEXT,
    incorrect_information TEXT
);

CREATE INDEX idx_request_events_time ON request_events(occurred_at);
CREATE INDEX idx_request_events_status ON request_events(result_status);
CREATE INDEX idx_request_events_coverage ON request_events(coverage);
CREATE INDEX idx_request_events_fingerprint ON request_events(context_fingerprint);
CREATE INDEX idx_request_events_tool ON request_events(tool_name);
CREATE INDEX idx_request_events_time_status ON request_events(occurred_at, result_status);
CREATE INDEX idx_request_events_time_coverage ON request_events(occurred_at, coverage);
CREATE INDEX idx_request_events_class ON request_events(request_class);
CREATE INDEX idx_request_roots_request ON request_roots(request_id);
CREATE INDEX idx_request_roots_root ON request_roots(root_key);
CREATE INDEX idx_evidence_events_request ON evidence_events(request_id);
CREATE INDEX idx_evidence_events_key ON evidence_events(evidence_key);
CREATE INDEX idx_evidence_events_type ON evidence_events(evidence_type);
CREATE INDEX idx_outcome_events_request ON outcome_events(request_id);
CREATE INDEX idx_outcome_events_fingerprint ON outcome_events(context_fingerprint);
