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
go build -ldflags "-X main.version=$(git describe --tags --always 2>/dev/null || echo 0.2.0)" -o implcache-mcp .
./implcache-mcp -db ./implcache.db
```

Default **`-mode agent`** registers retrieval tools only. Use **`-mode admin`** (or `-enable-admin-tools`) for ingest/delete/`vomit`.

```bash
# Coding-agent default (stdio)
./implcache-mcp -db ./implcache.db -workspace /path/to/repo

# Admin / corpus maintenance
./implcache-mcp -db ./implcache.db -mode admin

# Optional HTTP (loopback; mutations off unless -enable-http-mutations)
./implcache-mcp -db ./implcache.db -http :8080
```

### Cursor MCP config

```json
{
  "mcpServers": {
    "implcache": {
      "command": "D:/GitHub/implcache-mcp/implcache-mcp.exe",
      "args": ["-db", "D:/GitHub/implcache-mcp/implcache.db", "-mode", "agent"]
    }
  }
}
```

Use absolute paths. Reload MCP after rebuilding.

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

## License

MIT — Copyright (c) 2026 Adam G. Sweeney <agsweeney@gmail.com>.

See [LICENSE](LICENSE), [NOTICE](NOTICE), and [third_party/](third_party/).
