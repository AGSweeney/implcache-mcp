# MCP tools reference

Tools depend on server mode. Default **`-mode agent`** registers retrieval tools only. **`-mode admin`** adds ingest, delete, web/PDF/Git, inventory, and `vomit`.

Mutating calls can still be denied with `-readonly` or `-allow-*` flags even when admin schemas are registered.

When root scope is ambiguous, several tools return `needsChoice`, `message`, and `availableRoots`. Ask the user, then retry with an explicit root.

---

## Agent tools (always in agent mode)

### `get_implementation_context`

**Use first** for coding tasks that need local APIs, examples, or conventions.

| Argument | Required | Description |
|----------|----------|-------------|
| `task` | yes | Coding task in plain language |
| `language` | no | e.g. `c`, `cpp`, `go`, `python` |
| `technology` | no | Platform / library hint |
| `projectRoot` | no | Current-project root name |
| `preferredRoots` | no | Ordered roots to search |
| `rootGroup` | no | Named root group |
| `maxContextTokens` | no | Soft budget (default ~2500) |
| `semantic` | no | Supplement FTS with sparse term similarity |

**Useful result fields:** `requiredApis`, `relevantSymbols`, `includes`, `sequence`, `examples`, `constraints`, `pitfalls`, `citations`, `coverage`, `freshness`, `webSearchRecommended`, `contextFingerprint`, `estimatedTokens`.

---

### `find_symbol`

Exact → normalized → qualified → fuzzy lookup of APIs, functions, and types.

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | yes | Symbol name |
| `rootName` | no | Single root filter |
| `preferredRoots` | no | Multi-root filter |
| `limit` | no | Default 20 |

---

### `search_knowledge`

Full-text search with snippets. Prefer `get_implementation_context` for coding tasks.

| Argument | Required | Description |
|----------|----------|-------------|
| `query` | yes | Search query |
| `limit` | no | Capped by server (hard max 100) |
| `rootName` | no | Explicit root; else inferred |
| `semantic` | no | Also score related chunks |

May return `needsChoice` when the root is ambiguous.

---

### `get_document`

Fetch one document by URI or id. Use when the implementation package is not enough.

| Argument | Required | Description |
|----------|----------|-------------|
| `uri` | one of | e.g. `project://…` |
| `id` | one of | Numeric document id |
| `includeBody` | no | Concatenate chunk bodies into `body` |

---

### `list_roots`

No arguments. Lists distinct knowledge root names in the database.

---

## Admin-only tools

Registered with `-mode admin` or `-enable-admin-tools`.

### Local ingest

| Tool | Purpose |
|------|---------|
| `ingest_markdown` | Ingest `.md` / `.html` file or directory |
| `ingest_project` | Walk a source tree |

### Web

| Tool | Purpose |
|------|---------|
| `ingest_url` | Fetch and index one documentation URL |
| `add_web_source` | Register a site crawl configuration |
| `ingest_site` | Crawl within allowed URL prefixes |
| `refresh_web_source` | Refresh mirrored pages |
| `list_web_sources` | List web sources |
| `remove_web_source` | Remove source + docs |
| `prune_web_source` | Drop pages missing for N generations |

### PDF

| Tool | Purpose |
|------|---------|
| `inspect_pdf` | Classify / metadata (no DB write) |
| `ingest_pdf` | Index a local text PDF |
| `remove_pdf` | Delete by `pdf://…` URI |

### Git

| Tool | Purpose |
|------|---------|
| `inspect_repo` | Remote/local metadata (no ingest) |
| `add_repo_source` | Persist repo configuration |
| `ingest_repo` | Snapshot / managed clone / local checkout |
| `refresh_repo_source` | Fetch and reindex changes |
| `list_repo_sources` | List repo sources |
| `remove_repo_source` | Remove config / index / clone |

### Inventory and delete

| Tool | Purpose |
|------|---------|
| `list_documents` | List docs (optional `sourceType` filter) |
| `delete_document` | Delete one URI |
| `delete_by_uri_prefix` | Delete all URIs with a prefix |

### Librarian (MCP)

Parallel to the browser UI:

| Tool | Purpose |
|------|---------|
| `list_sources` / `get_source` | Unified inventory |
| `source_health` / `recent_source_errors` | Health |
| `preview_document` | Bounded preview |
| `search_playground` | Search with optional explain |
| `get_operation` / `list_operations` | In-process job progress |

### Recipes

| Tool | Purpose |
|------|---------|
| `vomit` | Compile a source-grounded playbook; optional file write / `saveRecipe` |

---

## Permission matrix (summary)

| Action | Blocked by |
|--------|------------|
| Ingest / crawl / refresh | `-readonly` or `-allow-ingest=false` |
| Delete / remove | `-readonly` or `-allow-delete=false` |
| `vomit` file write | `-readonly` or `-allow-output-write=false` |
| Mutations over HTTP | Missing `-enable-http-mutations` |

Retrieval tools remain available under `-readonly`.

---

## Cheat sheet

| Situation | Call |
|-----------|------|
| Help me implement X | `get_implementation_context` |
| Does `FooBar` exist / signature? | `find_symbol` |
| What’s in the help about timers? | `search_knowledge` + `rootName` |
| Show the whole sample file | `get_document` with `includeBody` |
| Write a reusable playbook | `vomit` (admin) |
| What corpora are loaded? | `list_roots` or `list_sources` |
| Is this source healthy? | `source_health` |
| How is the crawl going? | `get_operation` / `list_operations` |
