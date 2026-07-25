# Librarian UI and REST API

The Librarian is an optional browser console for managing ImplCache corpora: sources, jobs, library browsing, roots, search playground, health, and logs.

---

## Enable

```bash
implcache-mcp -db ./implcache.db -http :8080 \
  -enable-librarian -enable-http-mutations -mode admin
```

Open `http://127.0.0.1:8080/` (or the path set by `-librarian-base-path`).

| Surface | Path |
|---------|------|
| Librarian UI | `/` (or base path) |
| REST API | `/api/v1` |
| MCP over HTTP | `/mcp` |

No Node.js is required at runtime; the UI is embedded in the server binary.

---

## UI pages

| Page | Purpose |
|------|---------|
| Dashboard | High-level library stats |
| Sources | Inventory of local / web / PDF / Git sources |
| Add Source | Register or ingest a new source |
| Jobs | In-process job status and progress |
| Library | Browse documents and symbols |
| Roots | Knowledge roots and root groups |
| Search Lab | Interactive search / context playground |
| Health | Library-wide health issues |
| Logs | Recent in-process log lines |
| Settings | Server capability / auth hints |

---

## Authentication

| Flag | Role |
|------|------|
| `-librarian-token` | Administrator (full mutating API) |
| `-librarian-viewer-token` | Viewer (reads + search; mutations → 403) |

Send `Authorization: Bearer <token>`.

For job event streams (SSE), clients that cannot set headers may use `?access_token=`.

- If either token is configured, missing/invalid tokens → `401`  
- Viewer calling mutating methods → `403`  
- If neither token is set → API is open (intended for loopback only)  

For non-loopback exposure: use `-allow-remote-http`, terminate TLS at a reverse proxy, and set a Librarian token. Keep mutations gated with `-enable-http-mutations`.

---

## REST overview (`/api/v1`)

Capability discovery:

```http
GET /api/v1/server
```

| Area | Examples |
|------|----------|
| Sources | `GET /sources`, web/git/pdf/local ingest, refresh, delete, preview |
| Uploads | `POST /uploads` (multipart) |
| Jobs | `GET /jobs`, `GET /jobs/{id}`, SSE `/jobs/{id}/events`, cancel |
| Library | stats, documents, symbols |
| Roots | roots, root-groups CRUD |
| Search | playground, symbols, implementation context |
| Health / logs | `GET /health`, `GET /logs` |

Errors use a consistent JSON envelope:

```json
{
  "code": "not_found",
  "message": "source web/foo not found",
  "detail": "",
  "retryable": false,
  "sourceId": "web/foo",
  "jobId": ""
}
```

Common codes: `validation`, `authentication`, `authorization`, `not_found`, `conflict`, `ingestion`, `server`, `unsupported`.

---

## Jobs

Long-running site crawls and Git ingest/refresh report job ids. Progress is **process-local**:

- Survives browser reload while the server process is running  
- Does **not** survive server restart  

---

## Safety defaults

- HTTP binds to loopback by default (bare `:port` rewritten to `127.0.0.1`)  
- Mutations over HTTP require `-enable-http-mutations`  
- Security headers (CSP, frame denial, etc.) are applied to UI/API responses  

See also [CONFIGURATION.md](CONFIGURATION.md).
