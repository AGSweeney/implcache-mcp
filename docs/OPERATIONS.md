# Operations

## Build

```bash
go test ./...
go vet ./...
go build -o implcache-mcp .
go build -o ingestcli ./cmd/ingestcli
```

Module: `implcache-mcp` (Go 1.25+). Default reported version is `dev` (override with `-ldflags "-X main.version=…"`). SQLite is pure Go (`modernc.org/sqlite`); **no CGO** required for build/test. Schema `PRAGMA user_version` is currently **7**.

### Race detector

`go test -race` requires a **gcc-compatible** C toolchain (`CGO_ENABLED=1` + `gcc`/`clang`). MSVC `cl.exe` alone is not enough. The project still ships:

- `store.TestConcurrentReads` — concurrent reader smoke test without `-race`
- normal `go test ./...` as the default CI gate

`scripts/check.ps1` auto-discovers common Windows installs (PATH, Qt MinGW, MSYS2). Manual run:

```powershell
# PowerShell (example: Qt MinGW)
$env:CGO_ENABLED = "1"
$env:CC = "C:\Qt\Tools\mingw1310_64\bin\gcc.exe"
$env:Path = "C:\Qt\Tools\mingw1310_64\bin;$env:Path"
go test -race ./...
```

```bash
# bash
CGO_ENABLED=1 go test -race ./...
```

### Large-corpus checks

```bash
go test ./store -run TestLargeCorpusRootScopedPlan -count=1
go test ./store -bench=Benchmark -benchtime=200ms
go run ./cmd/evaltasks -seed-demo
```

## Run (stdio — Cursor)

```bash
./implcache-mcp -db ./implcache.db
```

Cursor `mcp.json` example:

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

After rebuilding the exe, **reload the MCP server** in Cursor so clients pick up the new binary.

### Read-only production-ish profile

```bash
./implcache-mcp -db ./implcache.db -readonly
```

Or finer gates:

```bash
./implcache-mcp -db ./implcache.db \
  -allow-ingest=false \
  -allow-delete=false \
  -allow-output-write=false
```

## Run (HTTP)

```bash
./implcache-mcp -db ./implcache.db -http :8080
```

Safety defaults:

- Bare `:8080`, `0.0.0.0`, or `::` → rewritten to `127.0.0.1`
- Pass an explicit non-loopback host only if you intend LAN exposure
- HTTP server uses header/idle timeouts and graceful SIGINT/SIGTERM shutdown

## Server flags

| Flag | Default | Purpose |
|------|---------|---------|
| `-db` | `./implcache.db` | SQLite path |
| `-http` | _(stdio)_ | Streamable HTTP address |
| `-readonly` | `false` | Search-only; opens DB read-only when possible |
| `-allow-ingest` | `true` | Enable ingest tools |
| `-allow-delete` | `true` | Enable delete tools |
| `-allow-output-write` | `true` | Allow `vomit` file writes |
| `-output-root` | `./vomit-output` | Jail for recipe output paths |
| `-max-results` | `20` | Search result cap |
| `-max-ingest-files` | `50000` | Per-ingest file cap |
| `-max-document-bytes` | `8MiB` | Per-file size cap |

## Security model (summary)

| Control | Behavior |
|---------|----------|
| Output jail | `vomit` writes must resolve under `-output-root` (`internal/safePath`) |
| Symlinks | Ingest refuses/skips symlink paths |
| HTTP bind | Loopback rewrite for accidental exposure |
| Readonly | Disables ingest/delete/file output |
| No full-body logging | Tool results are not dumped to server logs by default |
| Generated recipes | Labeled; ranked below human-reviewed |
| HTTP auth | **None** — loopback + flags by default; put a reverse proxy/auth in front for remote binds |

Do not commit `*.db` or ingested vendor corpora (see `.gitignore`).

## Limitations and risks

Treat ImplCache as **pre-1.0**: schema, ranking, and tool contracts can still change. Development builds report `dev`; inject a tag with `-ldflags "-X main.version=…"`.

| Area | Notes |
|------|--------|
| Symbol extraction | Heuristic regex for Go, C/C++/C#, Python, JS/TS, Java. Optional tree-sitter still future work. Unknown languages do not fall through to noisy C regex. |
| Search model | FTS5 + authority ranking by default. Optional **IDF-weighted sparse cosine** (`-enable-semantic` / `semantic: true`) supplements FTS through v7 indexed term postings — query-side corpus IDF, not neural embeddings or classic TF-IDF. Pure keyword search can still miss related concepts. |
| Freshness | Independent of authority. Official docs without version/date → `unknown`. `webSearchRecommended` uses coverage + freshness. |
| Fingerprints | `contextFingerprint` is over the post-trim response (+ citation content hashes). |
| Token estimates | `estimatedTokens` is roughly `utf8_runes/4` on the serialized JSON. Use for budgeting only — approximate, not exact. |
| HTTP | No built-in authentication. Loopback rewrite and `-allow-remote-http` are the safety defaults; remote exposure needs an external auth layer. |
| Recipes | Quality depends on human review of `vomit` / `saveRecipe` output. Ranking already demotes generated entries vs human-reviewed and project code. |
| Concurrency | SQLite **WAL** helps readers; multiple writers still need care (single writer process, or serialize admin ingest). See concurrent smoke tests; prefer one admin writer. |
| Go version | Module requires **Go 1.25+** — note for downstream consumers. |
| Semantic index | V7 indexed `(root_name, term, chunk_id)` postings replace leading-wildcard vector scans. Query-time smooth IDF downweights ubiquitous terms; candidate IN-lists are capped (16 terms). Posting growth is bounded by 48 terms/chunk — use `Store.SemanticStats` to watch cardinality on large corpora. |

## Database files

| File | Role |
|------|------|
| `implcache.db` | Main SQLite DB |
| `implcache.db-wal` / `-shm` | WAL mode sidecars when present |

New databases are created at the canonical schema on open. A database with a different `user_version` is refused: delete `implcache.db` (and `-wal`/`-shm`) and re-ingest (see [DATA_MODEL.md](DATA_MODEL.md)).

## Evaluation

```bash
go test ./implctx ./ingest ./store ./vomit -count=1
go test ./store -bench=Benchmark -benchtime=200ms
go run ./cmd/evaltasks -db ./implcache.db
```

`evaltasks` reports estimated tokens and expected-API recall for a small task set. It requires ingested data. Do not invent benchmark numbers in docs or reports.

Compare sparse semantic search against FTS-only for a corpus:

```bash
go run ./cmd/evaltasks -db ./implcache.db
go run ./cmd/evaltasks -db ./implcache.db -semantic
go test ./store -run TestSemanticPostingQueryPlan -count=1
```

On the sanitized 13-task seed corpus (including `reconnect-amid-noise` plus
generic noise documents), semantic off/on both produced top-1 and top-3 symbol
recall of 1.0, expected-source recall of 1.0, zero forbidden hits, and zero
duplicate excerpts. Semantic search increased average estimated response size
from 665.6 to 671.8 tokens and raised `reconnect-network-client` coverage from
medium to high; median and p95 latency were 3 ms in both modes. On a local
bench, multi-term semantic candidate lookup over ~800 chunks was ~7.6 ms/op
versus ~3.2 ms/op for a short query over 250 chunks. Semantic search remains
opt-in: seed recall is saturated, so the default stays FTS-only until a larger
corpus shows a clear ranking win.

## Typical ops loop

1. Build `implcache-mcp` + `ingestcli`  
2. Ingest docs root(s) and project root(s)  
3. Configure Cursor MCP with absolute paths  
4. Agents call `get_implementation_context`  
5. Optionally `vomit` + human-review high-value recipes  
6. Re-ingest when vendor docs or project code change  
7. Rebuild + reload MCP after code changes  

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Tools missing / old behavior | Reload MCP; confirm exe mtime |
| Empty search | `list_roots`; re-ingest |
| `needsChoice` | Pick a root; pass `rootName` / `preferredRoots` |
| Vomit write fails | Path must be relative under `-output-root`; check `-allow-output-write` |
| Ingest denied | Not `-readonly`; `-allow-ingest=true` |
| DB locked | Only one writer; stop other ingest/MCP writers |
