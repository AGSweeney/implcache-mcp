# MCP tools reference

All tools are registered by `tools.RegisterWithOptions`. Mutating tools can be disabled with `-readonly` or individual `-allow-*` flags.

When root scope is ambiguous, several tools return a JSON payload with `needsChoice`, `message`, and `availableRoots` (often as an error-shaped MCP result). Ask the user, then retry with an explicit root.

---

## Primary tools

### `get_implementation_context`

**Use first** for any coding task that needs local APIs, examples, or conventions.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `task` | string | yes | Coding task in plain language |
| `language` | string | no | e.g. `c`, `cpp`, `go` |
| `technology` | string | no | e.g. `example-device-sdk`, `example-network-sdk` |
| `projectRoot` | string | no | Preferred current-project root name |
| `preferredRoots` | string[] | no | Ordered roots to search |
| `rootGroup` | string | no | Named root group (DB) |
| `maxContextTokens` | int | no | Soft budget (default ~2500) |

**Returns** (`implctx.Response`), including:

- `summary`, `requiredApis`, `relevantSymbols`, `includes`, `sequence`
- `examples[]` (short excerpts + authority)
- `constraints`, `pitfalls`, `projectConventions`
- `citations[]` (`uri`, `title`, `section`, `lines`, `authority`, `rootName`)
- `coverage` (`high` \| `medium` \| `low`)
- `freshness`, `webSearchRecommended`, `missingInformation`, `recommendedFollowUp`
- `rootsUsed`, `estimatedTokens`, `chars`, `tokenEstimateNote`

---

### `find_symbol`

Exact/near-exact lookup of APIs, functions, and types from the `symbols` table.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `name` | string | yes | Symbol name (e.g. `RegisterCommand`) |
| `rootName` | string | no | Single root filter |
| `preferredRoots` | string[] | no | Multi-root filter |
| `limit` | int | no | Default 20 |

**Returns:** `{ symbols: Symbol[], count }`

`Symbol` fields include `name`, `kind`, `language`, `signature`, `uri`, `rootName`, `authority`, line range.

---

### `search_knowledge`

Full-text search over chunk FTS5. Prefer `get_implementation_context` for coding tasks; use this for broader exploration.

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `query` | string | yes | FTS query |
| `limit` | int | no | Capped by server `-max-results` (hard max 100) |
| `rootName` | string | no | Explicit root; else inferred |

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
| `sourceType` | string | Optional filter: `markdown` or `source` |

Returns `{ documents[], count }`.

---

## Mutation tools

Disabled when `-readonly`, or when the matching `-allow-*` flag is false.

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

### `delete_document`

| Argument | Type | Required |
|----------|------|----------|
| `uri` | string | yes |

### `delete_by_uri_prefix`

| Argument | Type | Required | Description |
|----------|------|----------|-------------|
| `prefix` | string | yes | e.g. `file:///` or `project://old_root/` |

---

## Permission matrix

| Tool | `-readonly` | `-allow-ingest=false` | `-allow-delete=false` | `-allow-output-write=false` |
|------|-------------|------------------------|------------------------|------------------------------|
| get / find / search / list / get_document | allowed | allowed | allowed | allowed |
| ingest_* | denied | denied | allowed | allowed |
| delete_* | denied | allowed | denied | allowed |
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
| “What corpora are loaded?” | `list_roots` |
