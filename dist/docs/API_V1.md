# Admin API v1

Versioned REST surface for the ImplCache Librarian UI. Mounted under `/api/v1` when `-http` is set (UI assets require `-enable-librarian`).

Base URL example: `http://127.0.0.1:8080/api/v1`

MCP Streamable HTTP remains at `/mcp`. Managed Git clones for `managed_clone` sources are cached under `<db-dir>/.implcache/repos/` (gitignored; not part of the published tree).

## Capability discovery

### `GET /server`

Returns server capabilities. No auth required when `authMode` is `none`.

```json
{
  "serverVersion": "dev",
  "apiVersion": 1,
  "schemaVersion": 11,
  "readOnly": false,
  "semanticEnabled": false,
  "ocrSupported": false,
  "supportedSourceTypes": ["local", "web", "pdf", "repo"],
  "authMode": "none",
  "librarianEnabled": true,
  "role": "administrator"
}
```

## Error envelope

All errors use:

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

## Sources

| Method | Path | Description |
|--------|------|-------------|
| GET | `/sources` | Unified source inventory |
| GET | `/sources/{kind}/{id}` | Source detail |
| GET | `/sources/{kind}/{id}/health` | Health snapshot |
| GET | `/sources/{kind}/{id}/errors` | Recent errors |
| POST | `/sources/web` | Register web source |
| POST | `/sources/web/{name}/ingest` | Start crawl job |
| POST | `/sources/web/{name}/refresh` | Refresh crawl job |
| DELETE | `/sources/web/{name}` | Remove web source |
| POST | `/sources/git` | Ingest / register git source |
| POST | `/sources/git/{name}/refresh` | Refresh git source |
| DELETE | `/sources/git/{name}` | Remove git source |
| POST | `/sources/pdf/inspect` | Inspect PDF (JSON body: path) |
| POST | `/sources/pdf/ingest` | Ingest PDF |
| DELETE | `/sources/pdf` | Remove by `?uri=` |
| POST | `/sources/local/preview` | Dry-run local path walk (no index writes) |
| POST | `/sources/local/ingest` | Ingest local markdown/project tree |
| DELETE | `/sources/local/{name}` | Remove synthesized local root (`project://{name}/…`) |
| POST | `/sources/git/inspect` | Inspect remote/local git repo (no ingest) |
| POST | `/sources/web/preview` | Dry-run web crawl plan |
| POST | `/uploads` | Upload file (multipart); returns server path token |

Kinds: `web`, `pdf`, `repo`, `local`. IDs are URL-escaped (PDF URIs).

## Jobs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/jobs` | Recent jobs |
| GET | `/jobs/{id}` | Job status |
| GET | `/jobs/{id}/events` | SSE progress stream |
| POST | `/jobs/{id}/cancel` | Best-effort cancel (context cancel) |

## Library

| Method | Path | Description |
|--------|------|-------------|
| GET | `/library/stats` | Dashboard aggregates |
| GET | `/library/documents` | Paginated documents (`?root=&sourceType=&limit=&offset=`) |
| GET | `/library/documents/{id}` | Document + bounded chunks |
| GET | `/library/documents/{id}/symbols` | Symbols for document |
| POST | `/library/purge-empty-docs` | Delete documents with no chunks (admin/delete; returns `{deleted,before,…}`) |

## Roots

| Method | Path | Description |
|--------|------|-------------|
| GET | `/roots` | Distinct root names |
| GET | `/root-groups` | List groups |
| PUT | `/root-groups/{name}` | Upsert group + members |
| DELETE | `/root-groups/{name}` | Delete group |

## Search

| Method | Path | Description |
|--------|------|-------------|
| POST | `/search` | Search playground: query, roots, rootName, limit, semantic, explain (score breakdown), allRoots |
| POST | `/search/symbols` | Find symbol |
| POST | `/search/context` | Implementation context (thin wrapper) |

## Health / logs

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Library-wide health issues |
| GET | `/logs` | Recent in-process log lines (ring buffer) |

## Auth (Stage 5)

| Flag | Role |
|------|------|
| `-librarian-token` | Administrator (full mutating API) |
| `-librarian-viewer-token` | Viewer (reads + search; mutations → `403`) |

Send `Authorization: Bearer <token>`. For SSE (`/jobs/{id}/events`), EventSource cannot set headers — use `?access_token=` as a fallback.

- Missing/invalid token (when either flag is set) → `401 authentication`
- Viewer mutating methods → `403 authorization`
- Neither token configured → open (intended for loopback)

Responses include security headers (`Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`). For non-loopback binds use `-allow-remote-http`, terminate TLS at a reverse proxy, and set a Librarian token.

## Jobs durability

Stage 1–5 jobs are **process-local** (in-memory tracker). They survive browser reload while the process lives, but not server restart. Persisted job rows (schema bump) remain optional future work.

## Embedding

With `-enable-librarian`, static assets are served from the configured base path (default `/`). API always under `/api/v1`. Production UI is embedded via `embedui/dist` (no Node.js required at runtime).
