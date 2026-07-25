# ImplCache MCP

ImplCache MCP is a local SQLite-backed implementation-context server for coding agents. It indexes technical documentation, source trees, examples, symbols, and implementation recipes, then returns compact, source-grounded context so agents can write software without repeatedly scanning files or searching the web.

> Return the smallest sufficient package of accurate, implementation-ready context for the current coding task.

Full documentation: **[docs/README.md](docs/README.md)**

---

## Why it exists

```text
Without ImplCache:
- agent searches multiple source and documentation files
- agent reads broad sections
- agent may search the web
- agent may choose an obsolete or incorrect API

With ImplCache:
- agent receives exact symbols
- agent receives one project-local example
- agent receives initialization constraints
- agent receives source citations
- full documents remain available only when needed
```

---

## Build and run

Requirements: Go 1.25+, pure-Go SQLite (`modernc.org/sqlite`, no CGO).

```bash
go test ./...
go build -ldflags "-X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" -o implcache-mcp .
./implcache-mcp -db ./implcache.db
./implcache-mcp -version   # "dev" unless injected via -ldflags
```

Default **`-mode agent`** registers retrieval tools only. Admin tools are **not** available in agent mode. Use **`-mode admin`** (or `-enable-admin-tools`) for ingest/delete/`vomit`. `-readonly` disables mutations even in admin mode.

```bash
# Coding-agent default (stdio, retrieval only)
./implcache-mcp -db ./implcache.db -workspace ./example-data

# Admin / corpus maintenance
./implcache-mcp -db ./implcache.db -mode admin

# Read-only admin surface (schemas present; writes denied)
./implcache-mcp -db ./implcache.db -mode admin -readonly

# Optional HTTP (loopback; mutations off unless -enable-http-mutations)
./implcache-mcp -db ./implcache.db -http :8080
```

### Cursor MCP config

```json
{
  "mcpServers": {
    "implcache": {
      "command": "D:/Tools/ImplCache/implcache-mcp.exe",
      "args": ["-db", "D:/Tools/ImplCache/implcache.db", "-mode", "agent"]
    }
  }
}
```

Use absolute paths. Reload MCP after rebuilding.

**Symbol matching** (`find_symbol`): staged exact → normalized → qualified → unqualified → prefix/suffix/token → bounded fuzzy. Definitions outrank declarations and calls. Extractors: Go, C/C++/C#, Python, JS/TS, Java.

**Optional semantic search**: `-enable-semantic` (or tool arg `semantic: true`) supplements FTS with deterministic sparse keyword-vector cosine. The tokenizer splits identifiers on camelCase/underscore boundaries (keeping the combined token), and the v7 inverted term postings index avoids wildcard vector scans; it is not embeddings or TF-IDF.

**Schema**: SQLite `PRAGMA user_version = 7` (see [docs/DATA_MODEL.md](docs/DATA_MODEL.md)). Pre-1.0; contracts may evolve.

---

## Ingest content

Prefer the offline CLI for large trees:

```bash
go build -o ingestcli ./cmd/ingestcli
./ingestcli -db ./implcache.db -mode markdown -root example-device-sdk -path /path/to/docs
./ingestcli -db ./implcache.db -mode project -root example-control-app -path /path/to/src
```

Or enable admin tools and call `ingest_markdown` / `ingest_project`. Details: [docs/INGEST.md](docs/INGEST.md).

---

## How an agent requests context

Primary tool:

```json
{
  "task": "Add a custom command using RegisterCommand and AddMenuItem",
  "language": "cpp",
  "technology": "Example Plugin SDK",
  "preferredRoots": ["example-control-app", "example-plugin-sdk"],
  "maxContextTokens": 2500
}
```

Typical fields: `requiredApis`, `relevantSymbols`, `sequence`, `examples`, `constraints`, `pitfalls`, `citations`, `coverage`, `contextFingerprint`, `estimatedTokens`.

Staged follow-ups: `find_symbol` → `search_knowledge` → `get_document`.

Agent guide: [docs/AGENT_GUIDE.md](docs/AGENT_GUIDE.md).

---

## Roots and project priority

- Each corpus is a portable URI tree: `project://{rootName}/…`
- Ambiguous queries return `needsChoice` + `availableRoots` (do not guess across product families)
- Optional workspace manifest **`.implcache.yaml`** sets the current project root and related roots:

```yaml
rootName: example-control-app
technology:
  - Example Device SDK
languages:
  - cpp
authority: current_project
relatedRoots:
  - example-device-sdk
  - example-device-examples
```

Pass `-workspace /path/to/repo` or `-project-root example-control-app` so agents need not repeat `projectRoot` on every call.

---

## Evaluation

```bash
go run ./cmd/evaltasks -seed-demo
go test ./store -bench=Benchmark -benchtime=200ms
```

---

## Freshness and fingerprints

- **Freshness** is not inferred from authority alone (official docs without version metadata stay `unknown`).
- **`contextFingerprint`** identifies the final trimmed package the client receives (source hash changes update it).
- Pre-release schema policy: one canonical schema, no migrations. An incompatible database is refused with instructions to delete and re-ingest it.

## Limitations (pre-1.0)

- Schema and ranking will evolve; pin or vendor if you need stability.
- Symbol extraction is heuristic (Go/C-family/Python/JS/Java); neural embeddings deferred — optional sparse semantic only.
- `estimatedTokens` is approximate (`runes/4`).
- HTTP has no auth — keep it on loopback or put a proxy in front. Mutations over HTTP stay off unless `-enable-http-mutations`.
- Recipe quality needs human review; generated entries rank below curated/project sources.
- SQLite WAL: fine for readers; serialize writers. Requires **Go 1.25+**.

Details: [docs/OPERATIONS.md](docs/OPERATIONS.md#limitations-and-risks).

---

## License

MIT — Copyright (c) 2026 Adam G. Sweeney <agsweeney@gmail.com>.

See [LICENSE](LICENSE), [NOTICE](NOTICE), and [third_party/](third_party/).
