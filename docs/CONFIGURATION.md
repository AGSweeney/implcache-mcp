# Configuration

ImplCache is configured with **command-line flags** and an optional **workspace manifest** (`.implcache.yaml`). A few analytics flags also accept environment overrides (`IMPLCACHE_TELEMETRY`, `IMPLCACHE_USAGE_DB`).

---

## Cursor / MCP client

Use absolute paths for the binary and database.

### Agent profile (daily coding)

```json
{
  "mcpServers": {
    "implcache": {
      "command": "D:/Tools/ImplCache/implcache-mcp.exe",
      "args": [
        "-db", "D:/Tools/ImplCache/implcache.db",
        "-mode", "agent",
        "-workspace", "D:/work/my_app"
      ]
    }
  }
}
```

`-mode agent` registers **retrieval tools only** (recommended for coding sessions).

### Admin profile (ingest / delete / recipes via MCP)

```json
{
  "mcpServers": {
    "implcache": {
      "command": "D:/Tools/ImplCache/implcache-mcp.exe",
      "args": [
        "-db", "D:/Tools/ImplCache/implcache.db",
        "-mode", "admin"
      ]
    }
  }
}
```

You can keep two MCP entries (e.g. `implcache` and `implcache-admin`) if you want both profiles available.

---

## Modes

| Mode | How | Tools available |
|------|-----|-----------------|
| **agent** (default) | `-mode agent` | `get_implementation_context`, `find_symbol`, `search_knowledge`, `get_document`, `list_roots` |
| **admin** | `-mode admin` or `-enable-admin-tools` | Agent tools + ingest, delete, web/PDF/Git, Librarian MCP tools, `vomit` |

Even in admin mode, you can deny writes with `-readonly` or individual `-allow-*` flags.

---

## Server flags

| Flag | Default | Purpose |
|------|---------|---------|
| `-db` | `./implcache.db` | SQLite knowledge database path |
| `-http` | _(empty = stdio)_ | Serve HTTP: MCP at `/mcp`, Librarian at `/api/v1` |
| `-mode` | `agent` | `agent` or `admin` |
| `-enable-admin-tools` | `false` | Register admin tools even when `-mode=agent` |
| `-readonly` | `false` | Disable ingest/delete/file output; open DB read-only when possible |
| `-allow-ingest` | `true` | Allow ingest when admin tools are enabled |
| `-allow-delete` | `true` | Allow delete when admin tools are enabled |
| `-allow-output-write` | `true` | Allow `vomit` to write files under `-output-root` |
| `-enable-http-mutations` | `false` | Allow ingest/delete/writes over HTTP |
| `-allow-remote-http` | `false` | Allow binding HTTP to a non-loopback address |
| `-enable-librarian` | `false` | Serve Librarian UI + `/api/v1` (requires `-http`) |
| `-librarian-base-path` | `/` | URL base path for the UI |
| `-librarian-token` | _(none)_ | Bearer token for administrator API access |
| `-librarian-viewer-token` | _(none)_ | Bearer token for viewer (read-only) API access |
| `-upload-dir` | `<db-dir>/uploads` | Librarian upload directory |
| `-workspace` | _(none)_ | Directory containing `.implcache.yaml` |
| `-project-root` | _(none)_ | Default knowledge root treated as current project |
| `-output-root` | `./vomit-output` | Directory that confines `vomit` output paths |
| `-max-results` | `20` | Search result cap |
| `-max-ingest-files` | `50000` | Max files per ingest operation |
| `-max-document-bytes` | `8 MiB` | Max bytes per ingested file |
| `-enable-semantic` | `false` | Optional sparse term-vector similarity (not embeddings) |
| `-telemetry` | `local` | Usage analytics: `local` or `off` (env `IMPLCACHE_TELEMETRY`) |
| `-usage-db` | `<db-dir>/implcache-usage.db` | Separate usage SQLite path (env `IMPLCACHE_USAGE_DB`) |
| `-telemetry-retention-days` | `90` | Purge usage events older than N days (`0` = unlimited) |
| `-telemetry-store-task-text` | `false` | Store truncated task text (off by default) |
| `-telemetry-store-evidence-text` | `false` | Store evidence text snippets (off by default) |
| `-version` | | Print version and exit |

---

## Local usage analytics

Analytics are **on by default**, metadata-only, and never block retrieval. Data is stored in a separate SQLite file (`implcache-usage.db`), not in the knowledge database. Disable with `-telemetry=off` or via Librarian **Settings → Usage analytics**. Clear via Settings or `DELETE /api/v1/analytics/data`. See [PRD_LOCAL_ANALYTICS.md](PRD_LOCAL_ANALYTICS.md) and the Analytics page in the Librarian UI.

---

## Workspace manifest (`.implcache.yaml`)

Optional file at a project root. Loaded when you pass `-workspace DIR`.

```yaml
rootName: my_app

technology:
  - Example Device SDK

languages:
  - cpp

authority: current_project

relatedRoots:
  - example-device-sdk
  - example-device-examples

versions:
  device_sdk: "3.x"
```

| Field | Required | Notes |
|-------|----------|-------|
| `rootName` | yes | Current project corpus id (no path separators) |
| `technology` | no | Hints for retrieval |
| `languages` | no | Language hints |
| `authority` | no | Usually `current_project` for your app |
| `relatedRoots` | no | Other corpora to prefer after the project |
| `versions` | no | Free-form version labels |

`-project-root NAME` overrides the manifest’s `rootName` as the default project root.

---

## HTTP and Librarian

```bash
implcache-mcp -db ./implcache.db -http :8080 \
  -enable-librarian -enable-http-mutations -mode admin
```

Safety defaults:

- Bare `:8080`, `0.0.0.0`, or `::` is rewritten to `127.0.0.1`
- Non-loopback binds require `-allow-remote-http`
- Mutations over HTTP are off unless `-enable-http-mutations`
- Optional Bearer auth: `-librarian-token` (admin), `-librarian-viewer-token` (viewer)

MCP over HTTP is at `/mcp`. It does not use Librarian Bearer tokens; protect exposure with bind address, mutation flags, and a reverse proxy + HTTPS if you leave loopback.

Shared deployment example:

```bash
implcache-mcp -db ./implcache.db -http 127.0.0.1:8080 \
  -enable-librarian -enable-http-mutations -mode admin \
  -librarian-token "your-admin-secret" \
  -librarian-viewer-token "your-viewer-secret"
```

Clients send `Authorization: Bearer <token>`. Job SSE streams may use `?access_token=` when headers cannot be set.

---

## Read-only profile

```bash
implcache-mcp -db ./implcache.db -mode agent -readonly
```

Or finer gates:

```bash
implcache-mcp -db ./implcache.db -mode admin \
  -allow-ingest=false -allow-delete=false -allow-output-write=false
```

---

## Optional semantic search

```bash
implcache-mcp -db ./implcache.db -mode agent -enable-semantic
```

Or pass `semantic: true` on supported tool calls. This supplements full-text search with sparse term similarity; it is **not** a neural embedding model. Default remains FTS-only.
