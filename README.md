# ImplCache MCP

**Local implementation context for coding agents.**

ImplCache is a Go-based MCP server backed by SQLite and FTS5. It indexes technical documentation, source trees, examples, and implementation recipes, then returns compact, source-grounded context that helps coding agents write correct software without repeatedly scanning files or searching the web.

> Give agents the APIs, examples, and project knowledge they need—without rereading the world.

Full documentation: **[docs/README.md](docs/README.md)**

---

## What it does

| Capability | Function |
|------------|----------|
| **Implementation packages** | `get_implementation_context` returns a budgeted package: summary, APIs/symbols, includes, sequence, examples, constraints, pitfalls, citations |
| **Symbol lookup** | `find_symbol` finds exact/near-exact APIs, functions, and types |
| **Full-text search** | `search_knowledge` runs root-scoped FTS5 with authority-aware ranking |
| **Recipe compiler** | `vomit` builds a source-grounded playbook; optional save as a generated recipe |
| **Corpus management** | Ingest Markdown/HTML/source trees; list/delete documents and roots |
| **Root isolation** | Each corpus is a `project://{rootName}/…` tree; ambiguous queries prompt for a root |

**Success metric:** fewer tokens, file opens, web searches, bad APIs, and correction cycles—not raw SQLite benchmark speed alone.

---

## What it is not

- A chatbot or open-ended Q&A product  
- A generic semantic-search / vector-DB product  
- A replacement for source control  
- A dump of full documentation trees into the agent context  
- A guarantee that local docs match the latest vendor release (see freshness signals)

---

## Agent workflow

```text
User asks an agent to build or modify software
  → Agent identifies task, technology, language, project
  → get_implementation_context  (primary)
  → Agent writes code
  → find_symbol / search_knowledge / get_document only if needed
```

See [docs/AGENT_GUIDE.md](docs/AGENT_GUIDE.md).

---

## Quick start

### Requirements

- Go 1.25+
- Pure Go SQLite (`modernc.org/sqlite`) — **no CGO**
- Optional: CGO only if you want `go test -race`

### Build and run

```bash
go test ./...
go build -o implcache-mcp .
./implcache-mcp -db ./implcache.db
```

Default transport is **stdio** (for Cursor / MCP clients). Optional HTTP:

```bash
./implcache-mcp -db ./implcache.db -http :8080
```

Bare `:port` / `0.0.0.0` binds are rewritten to **127.0.0.1** unless you pass an explicit host.

### Cursor MCP config

```json
{
  "mcpServers": {
    "implcache": {
      "command": "D:/GitHub/implcache-mcp/implcache-mcp.exe",
      "args": ["-db", "D:/GitHub/implcache-mcp/implcache.db"]
    }
  }
}
```

Use absolute paths. Reload MCP after rebuilding the binary.

### Ingest corpora

```bash
go build -o ingestcli ./cmd/ingestcli

# Docs (Markdown or HTML→Markdown)
./ingestcli -db ./implcache.db -mode markdown -root my_docs -path /path/to/docs

# Project source tree
./ingestcli -db ./implcache.db -mode project -root my_app -path /path/to/src
```

Same operations are available via MCP tools `ingest_markdown` and `ingest_project` (unless `-readonly`).

Details: [docs/INGEST.md](docs/INGEST.md).

---

## MCP tools (summary)

| Tool | Role |
|------|------|
| **`get_implementation_context`** | **Primary.** Compact cited package for a coding task |
| `find_symbol` | Exact/near-exact symbol lookup |
| `search_knowledge` | Broader FTS inside resolved roots |
| `get_document` | Full document / chunks when staged retrieval needs it |
| `list_documents` / `list_roots` | Inventory |
| `vomit` | Compile recipe/playbook; optional file write + `saveRecipe` |
| `ingest_markdown` / `ingest_project` | Add corpora |
| `delete_document` / `delete_by_uri_prefix` | Remove content |

Full argument/response reference: [docs/TOOLS.md](docs/TOOLS.md).

### Example: implementation context

```json
{
  "task": "register a Creo TOOLKIT menubar pushbutton",
  "language": "c",
  "technology": "Creo TOOLKIT",
  "preferredRoots": ["my_app", "creo_toolkit_help"],
  "maxContextTokens": 2500
}
```

Typical response fields: `summary`, `requiredApis`, `relevantSymbols`, `includes`, `sequence`, `examples`, `constraints`, `pitfalls`, `citations`, `coverage`, `freshness`, `webSearchRecommended`, `estimatedTokens`.

Default budget is roughly **1.5–3k tokens** (few hits, short excerpts, stop when full).

---

## Core concepts

| Concept | Meaning |
|---------|---------|
| **Knowledge root** | Isolated corpus named in URIs: `project://{rootName}/{rel}` |
| **Root group** | Named, priority-ordered set of roots for a tech/project |
| **Authority** | Source class used in ranking (project code → curated recipe → official example → docs → generated) |
| **Symbols** | Extracted APIs/functions/types for precise lookup |
| **Recipes** | Task-shaped Markdown entries (`knowledge_entries`), curated or generated-with-lineage |
| **Context budget** | Caps on results, examples, excerpt length, and estimated tokens |
| **Staged retrieval** | Prefer brief package → symbols → examples → surrounding source → full doc |

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) and [docs/DATA_MODEL.md](docs/DATA_MODEL.md).

---

## Server flags

| Flag | Default | Purpose |
|------|---------|---------|
| `-db` | `./implcache.db` | SQLite database path |
| `-http` | _(empty = stdio)_ | Streamable HTTP listen address |
| `-readonly` | `false` | Disable ingest, delete, and filesystem recipe writes |
| `-allow-ingest` | `true` | Gate `ingest_*` |
| `-allow-delete` | `true` | Gate `delete_*` |
| `-allow-output-write` | `true` | Gate `vomit` file writes under `-output-root` |
| `-output-root` | `./vomit` | Jail for recipe file output |
| `-max-results` | `20` | Search result cap |
| `-max-ingest-files` | `50000` | Max files per ingest |
| `-max-document-bytes` | `8MiB` | Max bytes per ingested file |

Operations & security: [docs/OPERATIONS.md](docs/OPERATIONS.md).

---

## Repository layout

| Path | Role |
|------|------|
| `main.go` | Flags, DB open, MCP stdio/HTTP |
| `tools/` | MCP tool registration and permission gates |
| `implctx/` | Budgeted implementation-package assembly |
| `store/` | SQLite schema, FTS, symbols, recipes, roots, ranking |
| `ingest/` | Markdown/HTML/project ingest + symbol extraction |
| `vomit/` | Recipe/playbook compiler + jailed writes |
| `internal/safePath/` | Output-path jail |
| `cmd/ingestcli/` | Offline ingest / delete-prefix CLI |
| `cmd/evaltasks/` | Small retrieval usefulness harness |
| `docs/` | This documentation set |

---

## Evaluation

```bash
go test ./...
go vet ./...
go test ./store -bench=Benchmark -benchtime=200ms
go run ./cmd/evaltasks -db ./implcache.db
```

`evaltasks` needs ingested data. Token estimates in responses are **approximate** (`utf8_runes/4`).

---

## Version and freshness

Local corpora can lag vendor docs. Responses may set `coverage`, `freshness`, and `webSearchRecommended`. Generated recipes are labeled and ranked **below** human-reviewed recipes.

---

## License

MIT — Copyright (c) 2026 Adam G. Sweeney <agsweeney@gmail.com>.

See [LICENSE](LICENSE) for the full text, [NOTICE](NOTICE) for third-party
attribution (MCP Go SDK, SQLite, and modernc.org/sqlite), and
[third_party/](third_party/) for upstream license copies.
