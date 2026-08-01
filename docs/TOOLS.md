# MCP tools reference

All tools are registered by `tools.RegisterWithOptions`. Default **`-mode agent`** registers retrieval tools only; **`-mode admin`** adds ingest/delete/`vomit`. Mutating calls can still be disabled with `-readonly` or individual `-allow-*` flags.

When root scope is ambiguous, several tools return a JSON payload with `needsChoice`, `message`, and `availableRoots` (often as an error-shaped MCP result). Ask the user, then retry with an explicit root.

Schema: `PRAGMA user_version = 12`. Symbol extraction at ingest supports: Go, C/C++/C#, Python, JavaScript/TypeScript, Java. Unsupported languages yield no symbols. Web fetch/crawl, PDF, and Git repo tools are **admin-only** (never registered in agent mode).

---

## Primary tools

### `get_implementation_context`

**Use first** for any coding task that needs local APIs, examples, or conventions.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `task` | string | yes | Coding task in plain language |
| `language` | string | no | e.g. `c`, `cpp`, `go`, `python`, `typescript` |
| `technology` | string | no | e.g. Example Plugin SDK |
| `projectRoot` | string | no | Preferred current-project root name |
| `preferredRoots` | string[] | no | Ordered roots to search |
| `knowledgeGroup` | string | no | Knowledge group id (e.g. `netburner`) for trusted cross-root retrieval |
| `rootGroup` | string | no | Deprecated alias for `knowledgeGroup` |
| `maxContextTokens` | int | no | Soft budget (default ~2500) |
| `semantic` | bool | no | Supplement FTS with sparse term vectors and indexed term postings (`-enable-semantic`) |
| `debug` | bool | no | Include `debugTaskTokens` (identifier-like tokens pulled from `task`) |
| `version` | string | no | Requested product/API version (affects freshness + soft ranking preference) |

**Returns** (`implctx.Response`), including:

- `summary`, `requiredApis`, `relevantSymbols`, `includes`, `sequence`
- `examples[]` (short excerpts + authority)
- `constraints`, `pitfalls`, `projectConventions`
- `citations[]` (`uri`, `title`, `section`, `lines`, `authority`, `rootName`, optional `sourceUris` for recipes)
- `coverage` (`high` \| `medium` \| `low`)
- `freshness` (independent of authority: `current` \| `version-specific` \| `mixed` \| `stale` \| `unknown`)
- `webSearchRecommended` (from coverage + freshness)
- `contextFingerprint` (hash of the **final trimmed** payload the client receives)
- `missingInformation`, `recommendedFollowUp` (staged next tools when coverage is low)
- `rootsUsed` (roots searched), `rootContribution` (searched vs contributing roots, citations-by-root, related over-limit, near-duplicate pairs), `recipeReviewStatus`, `estimatedTokens`, `chars`, `tokenEstimateNote`
- `debugTaskTokens` when `debug=true` — tokens matching CamelCase / `snake_case` / `ns::name` / `obj.method` heuristics

`preferredRoots` / `knowledgeGroup` / `projectRoot` must resolve to a **single knowledge group** (or a single ungrouped product family). Multiple roots in one configured knowledge group are searchable together. Roots spanning unrelated groups return `needsChoice` + `availableRoots` / `availableGroups`.

---

### `find_symbol`

Staged lookup of APIs, functions, and types from the `symbols` table:

`exact` → `exact_normalized` → `exact_qualified` → `exact_unqualified` → `prefix` → `suffix` → `token` → bounded `fuzzy` (unqualified_name candidates).

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `name` | string | yes | Symbol name (e.g. `RegisterCommand`) |
| `rootName` | string | no | Single root filter |
| `preferredRoots` | string[] | no | Multi-root filter (same knowledge group / family) |
| `limit` | int | no | Default 20 |

**Returns:** `{ symbols: Symbol[], count }` or `{ needsChoice, message, availableRoots }` when root scope is missing/ambiguous. Empty roots are never searched globally.

`Symbol` fields include `name`, `matchType`, `confidence`, `kind`, `language`, `signature`, `uri`, `rootName`, `authority`, line range. Definitions outrank declarations and calls.

---

### `search_knowledge`

Full-text search over chunk FTS5. Prefer `get_implementation_context` for coding tasks; use this for broader exploration.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `query` | string | yes | FTS query |
| `limit` | int | no | Capped by server `-max-results` (hard max 100) |
| `rootName` | string | no | Explicit root; else inferred |
| `semantic` | bool | no | Also score related chunks via sparse term vectors and indexed postings |

**Returns:** `{ hits, count, roots, matchedHints }` or `{ needsChoice, message, availableRoots, … }`

Hits include URI, title, snippet, score, authority, root, language/technology when present.

---

### `get_document`

Fetch one document by URI or numeric id. Use as **stage 5** when the package is insufficient.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `uri` | string | one of | `project://…` URI |
| `id` | int | one of | Document id |
| `includeBody` | bool | no | Concatenate chunk bodies into `body` |

**Returns:** `{ document, chunks[], body? }`

---

### `vomit`

Compile a source-grounded implementation recipe/playbook from local knowledge.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `subject` | string | yes | Topic to research |
| `outPath` | string | no | Relative path under `-output-root` |
| `limit` | int | no | Max documents to cite (default 8, max 20) |
| `maxCharsPerDoc` | int | no | Soft scan budget per source body |
| `rootName` | string | no | Explicit root; else inferred |
| `returnBody` | bool | no | Deprecated; body is always returned |
| `saveRecipe` | bool | no | Persist as generated `knowledge_entry` with lineage |
| `technology` | string | no | Metadata for saved recipe |
| `language` | string | no | Metadata for saved recipe |

**Returns:** `{ subject, outPath?, bytes, sourceCount, sources[], roots[], body?, recipeUri?, reviewNote? }`

Filesystem writes require `-allow-output-write` and stay inside `-output-root`.

---

## Inventory tools

### `list_roots`

No arguments. Returns `{ roots: string[], count }`.

### `list_documents`

| Argument | Type | Description |
|----------|------|-------------|
| `sourceType` | string | Optional filter: `markdown`, `source`, `web`, or `pdf` |

Returns `{ documents[], count }`.

---

## Mutation tools

Disabled when `-readonly`, or when the matching `-allow-*` flag is false. Web/PDF tools below are admin-only.

### `ingest_markdown`

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `path` | string | yes | File or directory (`.md` / `.html`) |
| `recursive` | bool | yes* | Recurse when `path` is a directory |
| `rootName` | string | no | Default: directory basename |

\* JSON schema marks recursive; pass `true`/`false` explicitly from clients that require it.

**Returns:** short text summary + structured `{ rootName, ingested, skipped, errors }`.

### `ingest_project`

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `path` | string | yes | Project root directory |
| `rootName` | string | no | Default: directory basename |

Walks text-like files, skips common junk / symlinks, extracts symbols where possible.

### `ingest_url`

Fetch and index **one** approved documentation URL (SSRF-safe; no link following). HTTPS preferred; `http://` requires `allowInsecureHTTP`.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `url` | string | yes | Documentation URL |
| `rootName` | string | no | Root for `project://` URIs |
| `profile` | string | no | `generic` \| `sphinx` \| `doxygen` |
| `authority`, `product`, `version`, `target`, `language` | string | no | Metadata |
| `allowInsecureHTTP` | bool | no | Permit `http://` |

### Web site mirroring

| Tool | Purpose |
|------|---------|
| `add_web_source` | Persist crawl config (`name`, `startUrl`, `rootName`, prefixes, profile) |
| `ingest_site` | Full crawl within allowed prefixes |
| `refresh_web_source` | Conditional GET / hash-skip refresh |
| `list_web_sources` | List registered sources + last status |
| `remove_web_source` | Delete source + mirrored docs |
| `prune_web_source` | Delete pages missing for N successful generations |

Defaults (crawl): maxPages 5000, maxDepth 16, concurrency 2, delay 100ms, 5 MB/response. Private/loopback/metadata hosts are blocked.

### PDF Stage 1

| Tool | Purpose |
|------|---------|
| `inspect_pdf` | Metadata/classification/bookmarks; **no DB writes** |
| `ingest_pdf` | Text PDF → `pdf://{root}/{file}` with page-cited chunks |
| `remove_pdf` | Delete by `pdf://…` URI |

`ocrMode` must be `off` (OCR deferred). Image-only PDFs are reported as requiring OCR and are not ingested.

### Git repository ingestion

Separate from web crawl. Uses system `git` (no hooks); credentials via GCM/SSH/`credentialReference` only (never stored in SQLite).

| Tool | Purpose |
|------|---------|
| `inspect_repo` | Remote/local metadata; no ingest |
| `add_repo_source` | Persist repo config |
| `ingest_repo` | Snapshot / managed clone / local checkout → `git://` root |
| `refresh_repo_source` | Fetch + changed-file reindex; failed refresh keeps prior commit |
| `list_repo_sources` | List configs + status |
| `remove_repo_source` | Remove config / index / managed clone |

GitHub HTML URLs (`/tree/`, `/blob/`) and `.git` remotes are rejected by web crawl tools with guidance to use repo tools instead.

### Librarian (MCP + browser UI)

The **browser Librarian** (`-enable-librarian`) is the primary GUI: REST `/api/v1` + embedded SPA (see [API_V1.md](API_V1.md)). These MCP tools are the parallel inventory/debug surface for admin/stdio clients. Per-type ingest/refresh/remove tools remain available over MCP; the UI uses the same acquisition packages via HTTP.

| Tool | Purpose |
|------|---------|
| `list_sources` | Union of web / PDF / Git / synthesized local roots |
| `get_source` | Inspect one source (`kind` + `id`) |
| `source_health` | State, counts, recent errors |
| `recent_source_errors` | Error list for one source |
| `preview_document` | Bounded chunk preview (`maxChunks` / `maxChars`) |
| `search_playground` | Search with optional `explain` (query plan) |
| `get_operation` / `list_operations` | In-process job progress (`opId` on site/repo ingest and refresh) |

Progress is process-local (not persisted across restarts). Schema remains `user_version = 12`.

### `delete_document`

| Argument | Type | Required |
|----------|------|----------|
| `uri` | string | yes |

### `delete_by_uri_prefix`

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `prefix` | string | yes | e.g. `file:///`, `project://old_root/`, `pdf://root/` |

---

## Permission matrix

| Tool | `-readonly` | `-allow-ingest=false` | `-allow-delete=false` | `-allow-output-write=false` |
|------|-------------|------------------------|------------------------|------------------------------|
| get / find / search / list_roots / get_document | allowed | allowed | allowed | allowed |
| Librarian read (`list_sources`, `get_source`, `source_health`, `recent_source_errors`, `preview_document`, `search_playground`, `get_operation`, `list_operations`, `list_*_sources`, `list_documents`, `inspect_*`) | allowed | allowed | allowed | allowed |
| ingest_* / `ingest_site` / `refresh_*` / `add_*_source` / `prune_web_source` | denied | denied | allowed | allowed |
| delete_* / `remove_*` | denied | allowed | denied | allowed |
| vomit (body / saveRecipe) | body ok* | allowed | allowed | body/save ok; file write denied |
| vomit (file write) | denied | allowed | allowed | denied |

\* With `-readonly`, filesystem output is off; the tool still returns the recipe body when called.

---

## Tool selection cheat sheet

| Situation | Call |
|-----------|------|
| “Help me implement X” | `get_implementation_context` |
| “Does `FooBar` exist / what’s the signature?” | `find_symbol` |
| “What’s in the example-control-app help about timers?” | `search_knowledge` + `rootName` |
| “Show me the whole sample file” | `get_document` with `includeBody` |
| “Write a reusable playbook for X” | `vomit` (+ optional `saveRecipe`) |
| “What corpora are loaded?” | `list_roots` (agent) or `list_sources` (admin / Librarian) |
| “Is this web/PDF/Git source healthy?” | `source_health` |
| “Preview a document without full body” | `preview_document` |
| “Debug a search query / plan” | `search_playground` (+ optional `explain`) |
| “How is the crawl going?” | `get_operation` / `list_operations` (`opId` from `ingest_site`) |
