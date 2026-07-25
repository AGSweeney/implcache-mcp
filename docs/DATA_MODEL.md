# Data model

Schema version: **7** (`PRAGMA user_version`). `store/schema.sql` is the single canonical schema, embedded by `store/schema.go`: new databases are created directly at version 7 in one transaction. `user_version` is a schema identity check, not a migration ladder — there is no upgrade path during pre-release development.

Ingest extracts symbols from Go, C/C++/C#, Python, JavaScript/TypeScript, and Java only. Runtime **freshness** (`current` / `version-specific` / `mixed` / `stale` / `unknown`) is computed separately from document **authority**. Implementation-context responses include a **`contextFingerprint`** of the final trimmed payload (see [TOOLS.md](TOOLS.md)).

## Portable URIs

Documents use portable URIs so databases can move across machines:

```text
project://{rootName}/{relative/path}
```

Examples:

```text
project://example-control-app/help/topics/timers.md
project://my_app/src/menu.c
project://example-device-sdk/ug/RegisterHandler.html
```

- **`rootName`** — corpus id (set at ingest; default = directory basename)
- **`relative/path`** — path under that ingest root, forward slashes

Legacy `file:///` URIs may still exist; `delete_by_uri_prefix` can remove them.

## Tables

### `documents`

One row per ingested file (or logical document).

| Column | Notes |
|--------|-------|
| `id` | Primary key |
| `uri` | Unique portable URI |
| `title` | Display title |
| `source_type` | `markdown` or `source` |
| `path` | Original filesystem path at ingest (informational) |
| `root_name` | Corpus id |
| `mtime`, `hash` | Change detection / skip unchanged |
| `authority` | Ranking class (see below) |
| `technology`, `language` | Optional tags |
| `product_version` | Optional version string (also inferred at ingest from path/body, e.g. `docs/v3.2/…`, `Version: 1.0`) |
| `deprecated`, `archived` | Flags (0/1) |
| `created_at`, `updated_at` | Unix times |

### `chunks`

Split bodies for FTS and partial retrieval.

| Column | Notes |
|--------|-------|
| `document_id` | FK → documents (CASCADE) |
| `ordinal` | Order within document |
| `heading`, `body` | Indexed text |
| `start_line`, `end_line` | Optional line range |

### `chunks_fts` (FTS5)

External-content FTS5 virtual table on `heading` + `body`, kept in sync by triggers.

### `chunk_term_vectors`

Optional sparse semantic-search support, one row per chunk.

| Column | Notes |
|--------|-------|
| `chunk_id` | Primary key and FK → `chunks.id` (CASCADE) |
| `terms` | Deterministically sorted, top-48 normalized keyword terms (empty only for text without usable terms) |
| `updated_at` | Unix time for the vector write |

Term frequency chooses the top terms (capped at 48), then each selected term is
stored once as a presence set. At query time, corpus document frequency from
`chunk_term_postings` supplies smooth IDF weights on the **query** vector
(`log(1 + N/(df+1)) + 1`); document vectors stay presence-normalized. The result
is IDF-weighted sparse cosine — **not** classic TF-IDF with per-chunk TF weights,
embeddings, or a vector database. Semantic search remains opt-in
(`-enable-semantic` or `semantic: true`) and supplements FTS rather than
replacing it. Candidate lookup uses at most 16 most-discriminative query terms
against the postings index; final scoring still uses the full query vector.

The tokenizer splits identifiers on camelCase/PascalCase boundaries and
underscores *before* lowercasing, keeping both the combined identifier and its
components (`RetryPolicy` → `retrypolicy`, `retry`, `policy`; acronym runs
survive: `HTTPServer` → `httpserver`, `http`, `server`). Namespace (`::`) and
member (`.`) separators split qualified names into parts. Tokens shorter than
3 runes and stopwords are dropped. This differs deliberately from symbol
normalization, which preserves qualification for exact lookup.

### `chunk_term_postings`

An inverted index over the terms in `chunk_term_vectors`.

| Column | Notes |
|--------|-------|
| `chunk_id` | FK → `chunks.id` (CASCADE); part of the primary key |
| `root_name` | Denormalized root scope for indexed lookup |
| `term` | One normalized vector term per row; part of the primary key |

The `(root_name, term, chunk_id)` index selects semantic candidates without
leading-wildcard scans. Candidate ordering favors chunks sharing more query
terms before cosine scoring.

### `symbols`

Pragmatic extractions at ingest (Go, C/C++/C#, Python, JavaScript/TypeScript, Java).

| Column | Notes |
|--------|-------|
| `document_id` | FK |
| `root_name` | Denormalized for filters |
| `name`, `name_norm` | Display + normalized lookup key |
| `kind` | e.g. `function`, `api`, `type` |
| `language`, `signature` | Metadata |
| `start_line`, `end_line` | Location |

Unique on `(document_id, name_norm, start_line)`. Cap ~80 symbols per file at ingest.

### `knowledge_entries` (recipes)

Task-shaped Markdown packages.

| Column | Notes |
|--------|-------|
| `uri` | Unique recipe URI |
| `subject` | Task / topic |
| `technology`, `language`, `version` | Tags |
| `body_markdown` | Full recipe body |
| `review_status` | `generated` or `human_reviewed` |
| `authority` | Usually `generated_summary` or `curated_internal_recipe` |
| `confidence` | Qualitative |
| `root_name` | Optional association |
| `hash`, timestamps | Dedup / audit |

### `knowledge_entry_sources`

Lineage: `(entry_id, source_uri)` + optional note. Required so generated recipes remain grounded.

### `aliases`

Controlled query expansion: `alias` → `canonical`, optional `technology` / `root_name`.

### `root_groups` / `root_group_members`

Named groups of roots with integer `priority` (higher first). Used by `get_implementation_context` via `rootGroup`.

## Authority values

Constants in `store`:

| Value | Meaning |
|-------|---------|
| `current_project` | Active application code |
| `related_internal_project` | Related internal code |
| `curated_internal_recipe` | Human-reviewed recipe |
| `official_example` | Official samples |
| `official_documentation` | Official docs/help |
| `generated_summary` | Auto-generated recipe |
| `third_party_reference` | Third-party material |
| `unknown` | Default / unset |

Ingest heuristics (`InferAuthority`) set a best-effort class from path/root; override in DB or future ingest options when precision matters.

## Ranking (search)

Composite score ≈ FTS rank + `AuthorityBoost(authority)` + light example/title biases, then diversify across documents. Generated summaries receive a lower boost than curated recipes and project code.

## Context budget (not stored)

Runtime structure `store.ContextBudget` limits how much text `implctx` returns. See [ARCHITECTURE.md](ARCHITECTURE.md).

## Schema versioning policy (pre-release)

One canonical schema, no migration ladder:

- **Version matches** (`user_version == 7`): run a lightweight `sqlite_master` check for required objects (`documents`, `chunks`, `chunks_fts`, `symbols`, `chunk_term_vectors`, `chunk_term_postings`, `idx_chunk_term_postings_root_term`), then open. Missing objects are refused with rebuild instructions — the file is not repaired.
- **Empty/new database**: create the full v7 schema directly from `store/schema.sql`, then set `user_version = 7`.
- **Any other version** (or an unversioned non-empty file): refuse to open without modifying the file. The error reports the database path, the found version, and the expected version, and instructs the developer to delete the database (and its `-wal`/`-shm` sidecars) and re-ingest.

During development, delete and recreate databases after schema changes; no time is spent backfilling old versions or testing historical upgrades.

> ImplCache has not yet shipped a persistent production database format. During pre-release development, schema changes require recreating the local database. Backward migrations will begin when the first persistent deployment is released.

### When migrations begin (post-deployment cutover)

Treat **schema v7** as the baseline once any deployed database must be preserved. At that point:

1. Stop deleting incompatible databases by policy; introduce an explicit migration ladder from `user_version = 7` forward.
2. Keep `store/schema.sql` as the canonical *fresh* schema for new installs at the latest version.
3. Add per-version upgrade steps that run inside transactions and leave failed opens retryable.
4. Never silently rewrite a production file; refuse unknown future versions until a matching build is deployed.
5. Document each bump in release notes with recreate-vs-migrate guidance for operators still on pre-release DBs.

The vector table intentionally has no ordinary `terms` B-tree index: SQLite
cannot use it for leading-wildcard `LIKE '%term%'`. Semantic candidate lookup
instead uses `idx_chunk_term_postings_root_term`; the query-plan test records
indexed candidate selection.
