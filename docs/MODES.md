# Tool modes and HTTP safety

## Modes

| Mode | Flag | Registered tools |
|------|------|------------------|
| **agent** (default) | `-mode agent` | `get_implementation_context`, `find_symbol`, `search_knowledge`, `get_document`, `list_roots` |
| **admin** | `-mode admin` or `-enable-admin-tools` | agent tools + all admin-only tools below |

**Admin-only tools** (schemas omitted in agent mode):

| Group | Tools |
|-------|-------|
| Local ingest | `ingest_markdown`, `ingest_project` |
| Web | `ingest_url`, `add_web_source`, `ingest_site`, `refresh_web_source`, `list_web_sources`, `remove_web_source`, `prune_web_source` |
| PDF | `inspect_pdf`, `ingest_pdf`, `remove_pdf` |
| Git | `inspect_repo`, `add_repo_source`, `ingest_repo`, `refresh_repo_source`, `list_repo_sources`, `remove_repo_source` |
| Inventory / delete | `list_documents`, `delete_document`, `delete_by_uri_prefix` |
| Librarian (MCP) | `list_sources`, `get_source`, `source_health`, `recent_source_errors`, `preview_document`, `search_playground`, `get_operation`, `list_operations` |
| Recipes | `vomit` |

The **Librarian browser UI** is separate from MCP: enable with `-http` + `-enable-librarian` (see [API_V1.md](API_V1.md) / [OPERATIONS.md](OPERATIONS.md)). It talks REST `/api/v1`, not MCP-over-HTTP.

Call-time permission flags (`-readonly`, `-allow-ingest`, …) still apply when admin tools are present.

## Workspace defaults

| Flag | Purpose |
|------|---------|
| `-workspace DIR` | Load `DIR/.implcache.yaml` for default project / related roots |
| `-project-root NAME` | Default `projectRoot` when the agent omits it |

## HTTP

| Behavior | Default |
|----------|---------|
| Bind | Loopback; bare `:port` / `0.0.0.0` rewritten to `127.0.0.1` |
| Paths | MCP Streamable HTTP at `/mcp`; Librarian REST at `/api/v1`; UI at `-librarian-base-path` when `-enable-librarian` |
| Non-loopback | Refused unless `-allow-remote-http` |
| Mutations over HTTP | Off unless `-enable-http-mutations` |
| Librarian auth | Optional Bearer (`-librarian-token` admin, `-librarian-viewer-token` viewer); open when unset (loopback-oriented) |

## Version

Local/development builds report **`dev`**. Inject a tag or commit at build time:

```bash
./implcache-mcp -version
go build -ldflags "-X main.version=$(git describe --tags --always)" -o implcache-mcp .
```

Pre-1.0: MCP tool contracts and schema may still evolve. Schema version is independent (`PRAGMA user_version`; currently **11**).

## Optional semantic search

```bash
./implcache-mcp -mode agent -enable-semantic
```

Or per-call `semantic: true` on `search_knowledge` / `get_implementation_context`. Uses sparse term vectors over chunks (cosine), merged with FTS; indexed term postings plus persisted DF for candidates/IDF. It is not an embedding model.

## Read-only

`-readonly` opens the DB read-only when possible and clears ingest/delete/output-write permissions. Admin tool **schemas** may still register in admin mode; mutation calls are denied.
