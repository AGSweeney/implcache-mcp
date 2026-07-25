# ImplCache MCP

**Give coding agents the exact APIs, examples, and pitfalls from *your* docs and source — not another web search.**

ImplCache is a local MCP server backed by SQLite. It indexes documentation, project trees, PDFs, mirrored sites, and Git repos, then returns a **budgeted, cited implementation package** so agents can write correct code without scanning the whole tree.

> Return the smallest sufficient package of accurate, implementation-ready context for the current coding task.

[![License: MIT](https://img.shields.io/badge/License-MIT-teal.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](go.mod)
[![MCP](https://img.shields.io/badge/MCP-stdio%20%7C%20HTTP-1a8a94)](docs/TOOLS.md)

![Librarian Dashboard](docs/images/librarian/librarian-dashboard.png)

---

## Why it exists

```text
Without ImplCache                          With ImplCache
─────────────────                          ──────────────
Agent greps / opens many files             Agent gets exact symbols
Agent skims broad docs                     One project-local example
Agent searches the web                     Init order + constraints
Agent may pick an obsolete API             Citations back to your corpus
```

Works with **Cursor** and other MCP clients. Optional **Librarian** browser UI manages sources, jobs, search, and health — no Node.js at runtime (UI is embedded in the binary).

| Sources | Library | Search Lab |
|:-------:|:-------:|:----------:|
| ![Sources](docs/images/librarian/librarian-sources.png) | ![Library](docs/images/librarian/librarian-library.png) | ![Search Lab](docs/images/librarian/librarian-search-lab.png) |

---

## Try it in 60 seconds (no Go install)

This repo ships a ready-to-run package under [`dist/`](dist/): Windows binaries, empty starter database, docs, and helper scripts.

```powershell
cd dist
.\run-librarian.cmd
# open http://127.0.0.1:8080/
```

Or point Cursor at the binary (use **absolute** paths):

```json
{
  "mcpServers": {
    "implcache": {
      "command": "D:/path/to/implcache-mcp/dist/implcache-mcp.exe",
      "args": [
        "-db", "D:/path/to/implcache-mcp/dist/implcache.db",
        "-mode", "agent"
      ]
    }
  }
}
```

Then ingest something you own:

```powershell
cd dist
.\ingestcli.exe -db .\implcache.db -mode markdown -root my-docs -path "C:\path\to\docs"
.\ingestcli.exe -db .\implcache.db -mode project -root my-app -path "D:\work\my-app"
```

Full end-user docs: [dist/README.md](dist/README.md) · [dist/docs/INSTALLATION.md](dist/docs/INSTALLATION.md)

> `dist/implcache.db` is a **sanitized empty** schema DB (no vendor corpora). Pack regenerates it with `pwsh ./scripts/pack-dist.ps1` and never copies a developer database.

---

## What agents get

Primary tool: **`get_implementation_context`**

```json
{
  "task": "Add a custom command using RegisterCommand and AddMenuItem",
  "language": "cpp",
  "technology": "Example Plugin SDK",
  "preferredRoots": ["example-control-app", "example-plugin-sdk"],
  "maxContextTokens": 2500
}
```

Typical response fields: `requiredApis`, `relevantSymbols`, `sequence`, `examples`, `constraints`, `pitfalls`, `citations`, `coverage`, `contextFingerprint`.

**Staged depth** (don’t dump the corpus):

1. `get_implementation_context`
2. `find_symbol` → `search_knowledge`
3. `get_document` only when the package isn’t enough

Agent playbook: [docs/AGENT_GUIDE.md](docs/AGENT_GUIDE.md) · Tool reference: [docs/TOOLS.md](docs/TOOLS.md)

---

## Features at a glance

| Capability | Notes |
|------------|--------|
| **Local-first** | SQLite on your machine; no cloud required |
| **Cited packages** | Budgeted answers with source URIs |
| **Multi-source ingest** | Markdown/docs, project trees, web mirrors, PDF Stage 1, Git repos |
| **Symbol search** | Go, C/C++/C#, Python, JS/TS, Java (heuristic extractors) |
| **Librarian UI** | Dashboard, Sources, Jobs, Library, Search Lab, Health |
| **Modes** | `agent` (retrieval) vs `admin` (ingest / delete / recipes) |
| **Optional semantics** | Sparse term similarity (`-enable-semantic`) — not embeddings |

---

## Build from source

Requirements: **Go 1.25+**, pure-Go SQLite (`modernc.org/sqlite`, no CGO).

```bash
go test ./...
go build -ldflags "-X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" -o implcache-mcp .
go build -o ingestcli ./cmd/ingestcli

./implcache-mcp -db ./implcache.db -mode agent
./implcache-mcp -version
```

### Common launches

```bash
# Admin / corpus maintenance (stdio)
./implcache-mcp -db ./implcache.db -mode admin

# Librarian UI + mutating REST on loopback
./implcache-mcp -db ./implcache.db -http :8080 \
  -enable-librarian -enable-http-mutations -mode admin
# → http://127.0.0.1:8080/

# Read-only admin surface (tools visible; writes denied)
./implcache-mcp -db ./implcache.db -mode admin -readonly
```

Optional workspace defaults via **`.implcache.yaml`** and `-workspace /path/to/repo` (or `-project-root my-app`). See [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

React UI sources: `frontend/` → rebuild into `embedui/dist` (or run `scripts/pack-dist.ps1`).

---

## Ingest

Prefer the offline CLI for large trees:

```bash
./ingestcli -db ./implcache.db -mode markdown -root my-sdk-docs -path /path/to/docs
./ingestcli -db ./implcache.db -mode project -root my-app -path /path/to/src
./ingestcli -db ./implcache.db -mode url -root web-docs -url "https://docs.example.com/api.html"
./ingestcli -db ./implcache.db -mode pdf-ingest -root manuals -path /path/to/manual.pdf
./ingestcli -db ./implcache.db -mode repo-ingest -name sdk -root sdk-main \
  -url https://github.com/org/sdk.git -ref main -acq managed_clone
```

Or use Librarian / admin MCP tools. Details: [docs/INGEST.md](docs/INGEST.md).

Each corpus is a portable URI tree (`project://{rootName}/…`, `git://…`, `pdf://…`). Ambiguous queries return `needsChoice` + `availableRoots` — agents should not guess across product families.

---

## Documentation

| Doc | Audience |
|-----|----------|
| [docs/USERS_MANUAL.md](docs/USERS_MANUAL.md) | Operators (install, ingest, Cursor, Librarian) |
| [dist/README.md](dist/README.md) | Binary package index |
| [docs/AGENT_GUIDE.md](docs/AGENT_GUIDE.md) | How agents should call tools |
| [docs/TOOLS.md](docs/TOOLS.md) | Full MCP tool reference |
| [docs/API_V1.md](docs/API_V1.md) | Librarian REST `/api/v1` |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Design & retrieval pipeline |
| [docs/OPERATIONS.md](docs/OPERATIONS.md) | Ops, security, eval |

---

## Limitations (pre-1.0)

- Schema and ranking can still change; pin or vendor if you need stability.
- Symbol extraction is heuristic; neural embeddings are deferred (optional sparse semantic only).
- Keep HTTP on **loopback** unless you deliberately expose it (`-allow-remote-http` + reverse proxy + HTTPS + Librarian tokens).
- HTTP mutations stay off unless `-enable-http-mutations`.
- SQLite WAL: fine for readers; serialize writers.
- An incompatible DB is refused — delete and re-ingest (no automatic migrations).

---

## License

MIT — Copyright (c) 2026 Adam G. Sweeney \<agsweeney@gmail.com\>.

See [LICENSE](LICENSE), [NOTICE](NOTICE), and [third_party/](third_party/).
