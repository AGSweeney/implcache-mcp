# Data model

Schema version: **3** (`PRAGMA user_version`). Migrations live in `store/migrate.go` and apply forward-only on open.

## Portable URIs

Documents use portable URIs so databases can move across machines:

```text
project://{rootName}/{relative/path}
```

Examples:

```text
project://ccw_help/help/topics/timers.md
project://my_app/src/menu.c
project://creo_toolkit_help/ug/user_initialize.html
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
| `product_version` | Optional version string |
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

### `symbols`

Pragmatic extractions at ingest (Go / C / C++ / Pro\* heuristics).

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

## Migrations

| Version | Change |
|---------|--------|
| 1 | documents, chunks, FTS5, triggers |
| 2 | Indexes on `root_name` (+ uri/source_type) |
| 3 | Authority columns, symbols, recipes, aliases, root groups |

Opening a DB always migrates to the current version.
