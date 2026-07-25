# Operations

## Build

```bash
go test ./...
go vet ./...
go build -o implcache-mcp .
go build -o ingestcli ./cmd/ingestcli
```

Module: `implcache-mcp` (Go 1.25+). SQLite is pure Go (`modernc.org/sqlite`); **no CGO** required for build/test.

`go test -race` needs CGO and is optional; skip it on no-CGO hosts.

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
| `-output-root` | `./vomit` | Jail for recipe output paths |
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

Do not commit `*.db` or ingested vendor corpora (see `.gitignore`).

## Database files

| File | Role |
|------|------|
| `implcache.db` | Main SQLite DB |
| `implcache.db-wal` / `-shm` | WAL mode sidecars when present |

Schema migrates automatically on open (see [DATA_MODEL.md](DATA_MODEL.md)).

## Evaluation

```bash
go test ./implctx ./ingest ./store ./vomit -count=1
go test ./store -bench=Benchmark -benchtime=200ms
go run ./cmd/evaltasks -db ./implcache.db
```

`evaltasks` reports estimated tokens and expected-API recall for a small task set. It requires ingested data. Do not invent benchmark numbers in docs or reports.

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
